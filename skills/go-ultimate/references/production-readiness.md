# Production Readiness

What a Go service needs beyond "it compiles and the tests pass". Read this when
building, reviewing or hardening a **backend service**, a long-running MCP
server, or a hosted agent — anything that runs unattended and that someone gets
paged for.

Scope boundary: [engineering-policy.md](engineering-policy.md) covers how the
code is written; this covers how it behaves in production. Where the two touch
(outbound HTTP, graceful shutdown, observability baseline), engineering-policy
states the rule and this reference states the operational shape.

## Contents

- [Timeouts, retries, idempotency](#timeouts-retries-idempotency)
- [The API error contract](#the-api-error-contract)
- [Health and readiness](#health-and-readiness)
- [Observability beyond logs](#observability-beyond-logs)
- [Configuration and the 12-factor rules that matter](#configuration-and-the-12-factor-rules-that-matter)
- [Secrets and hardening](#secrets-and-hardening)
- [Data: migrations, transactions, outbox](#data-migrations-transactions-outbox)
- [Performance work](#performance-work)
- [The runbook](#the-runbook)
- [Sources](#sources)

---

## Timeouts, retries, idempotency

### Every network call is bounded

No exceptions. A call without a deadline is an unbounded resource hold that
turns one slow upstream into a cascading outage.

- **Derive a new context with a timeout at every network or I/O boundary.** The
  inbound request deadline is not enough — an upstream that is slower than your
  own SLA must fail fast rather than consume your whole budget.
- The `*http.Client` `Timeout` is the **backstop**, the per-call context is the
  **budget**. Set both (see
  [engineering-policy.md § Outbound HTTP](engineering-policy.md)).
- Budget downward: if the handler has 2s, an upstream gets a fraction of it, not
  all of it. Leave room to render an error.

### Retry only what is safe to retry

A retry is a correctness decision before it is a reliability one.

- **Retry idempotent operations only.** `GET`, `PUT`, `DELETE` by natural key —
  yes. A bare `POST` that creates something — no, unless it carries an
  idempotency key (below).
- **Exponential backoff with jitter**, always. Fixed-interval retries from many
  clients synchronize into a thundering herd; jitter is what breaks the phase
  lock, and it is not optional.
- **Cap both the attempt count and the total elapsed time**, and abort the moment
  `ctx.Done()` fires. A retry loop that outlives its caller's deadline is a leak.
- **Do not retry what cannot succeed** — 4xx (except 429), validation failures,
  context cancellation. Retrying a deterministic failure just multiplies load
  during an incident.
- Retries at more than one layer multiply. If the client retries 3× and the
  gateway retries 3×, the upstream sees 9×. Pick **one** layer to own retries and
  make the others pass failures through.

Test backoff with `testing/synctest` rather than by sleeping — see
[testing.md § Testing concurrent code](testing.md#testing-concurrent-code).

### Idempotency keys

For any API or job that creates or mutates state and that a client may retry:

- The client supplies a key (header or field); the server stores
  `key → result` and returns the **stored result** on a repeat, rather than
  performing the work twice.
- Decide the key's retention window explicitly and document it — an idempotency
  record that expires sooner than the client's retry budget is not idempotency.
- Design this **up front**. Retrofitting deduplication onto a live write path
  means reconciling the duplicates that already happened.

---

## The API error contract

An error surface is public API. Define it once, at the boundary, and keep it
stable.

```json
{
  "code": "rate_limited",
  "message": "too many requests, retry after 30s",
  "details": { "retry_after_seconds": 30 },
  "request_id": "01JD7X2Q8M5V0K"
}
```

- **`code`** is a stable machine-readable string. Clients branch on it, so it is
  covered by your compatibility guarantees — see
  [libraries.md § Semver and public API stability](libraries.md).
- **`message`** is for humans and may change freely.
- **`request_id`** appears in the response *and* in every log line for that
  request. Without it, a user's bug report cannot be joined to your logs.
- Map **domain errors to transport codes in the adapter**, never in the business
  package — the domain does not know what HTTP is (non-negotiable principle 5).
  A sentinel or typed error crosses the port; the adapter translates:

  ```go
  // internal/in/http — the only place that knows about status codes.
  switch {
  case errors.Is(err, app.ErrNotFound):
      writeError(w, http.StatusNotFound, "not_found", err)
  case errors.As(err, &verr):
      writeError(w, http.StatusBadRequest, "invalid_argument", verr)
  default:
      // Never leak an internal error string to the client.
      writeError(w, http.StatusInternalServerError, "internal", errInternal)
  }
  ```

- **Never return a raw internal error to a client.** Log the detail with the
  request id, return the code. Unwrapped errors leak table names, file paths and
  upstream hostnames.

---

## Health and readiness

Two endpoints, two different questions. Conflating them causes rolling deploys to
either kill healthy pods or route traffic to dead ones.

| Endpoint | Question | Fails when | Orchestrator reaction |
|---|---|---|---|
| `/healthz` (liveness) | Is the process alive? | Deadlock, unrecoverable state | Restart the container |
| `/readyz` (readiness) | Can it serve traffic *now*? | A required dependency is down, still warming up, draining | Remove from the load balancer |

- **Liveness must not check dependencies.** A liveness probe that pings the
  database restarts every replica when the database blips — turning a
  degradation into an outage.
- **Readiness must reflect required dependencies** (DB, broker, cache if the
  service cannot serve without it), with a short cached TTL so the probe does not
  itself become load.
- **On shutdown, fail readiness first**, then drain. The load balancer needs to
  stop sending work before the server stops accepting it, or in-flight requests
  die at the socket. See
  [engineering-policy.md § Graceful shutdown](engineering-policy.md).

---

## Observability beyond logs

[engineering-policy.md § Observability baseline](engineering-policy.md) sets the
minimum: `log/slog`, a metrics registry, request-id plumbing. This is what a
service that gets paged for actually needs.

### Metrics

Instrument the four signals that answer "is it broken and where":

- **Latency** — histogram, so p95/p99 exist. An average latency metric cannot
  detect the failure mode users notice.
- **Error rate** — by `code`, matching the API error contract above.
- **Throughput** — requests/messages per second.
- **Saturation** — queue depth, pool utilization, worker occupancy. This is the
  one that predicts the outage instead of reporting it.

**Keep label cardinality bounded.** Never label a metric with a user id, request
id, raw path, or any unbounded string — every distinct value is a new time
series, and this is the standard way to take down a metrics backend.

### Tracing

- OpenTelemetry, propagated across **both** HTTP and messaging boundaries. A
  trace that stops at the queue is a trace that stops exactly where the hard bugs
  live.
- Same cardinality discipline on span attributes.
- Inbound adapters extract or generate the trace context; outbound adapters
  inject it. Both are adapter concerns, not business-logic concerns.

### SLOs

- Track **p95/p99, not averages**, and define an error budget.
- **Alert on symptoms, not causes.** "Checkout error rate above budget" pages
  someone who can act; "CPU is 80%" pages someone at 3am for a healthy service.

---

## Configuration and the 12-factor rules that matter

Not all twelve are worth restating. These four are the ones Go services get
wrong:

1. **Config comes from the environment**, and is read **once at startup** into
   the business `Config` struct (see
   [engineering-policy.md § Constructor pattern](engineering-policy.md)). Nothing
   below `main` reads `os.Getenv` — that is what makes the code testable without
   `t.Setenv`, and what keeps tests parallel.
2. **Logs are an event stream to stdout.** The process never opens a log file,
   never rotates one, never manages retention. The platform collects stdout.
   Structured JSON in production; human-readable is a local-dev toggle.
3. **`.env` is a local-development convenience only.** Provide a committed
   `.env.example` listing every variable with a safe placeholder; never commit
   `.env` itself (it is in the ignore baseline — see
   [repo-and-ci.md](repo-and-ci.md)). Production configuration comes from the
   platform's secret store, not from a file in the image.
4. **Fail fast on bad configuration.** `Resolve()` validates at startup and the
   process exits non-zero with a field-attributed error. A service that starts
   with a missing required setting and fails on the first request has converted a
   deploy-time error into a runtime incident.

> A config library (`envconfig`, `viper`) is allowed but is not the default.
> Plain `os.Getenv` into a `Config` struct with a `Resolve()` covers most
> services without a dependency. Reach for a library when you genuinely need
> layered sources (file + env + flags) or live reload.

---

## Secrets and hardening

- **Never log a secret.** Give credential-bearing types a `String()`/`LogValue()`
  that redacts, so an accidental `slog.Any("cfg", cfg)` cannot leak them. Relying
  on reviewers to never log the wrong struct is not a control.
- **Prefer short-lived credentials** (workload identity, STS, OIDC federation)
  over static long-lived keys. When a static key is unavoidable, document its
  rotation procedure in the runbook.
- **Validate at the boundary, bound everything.** Size limits on request bodies
  (`http.MaxBytesReader`), a cap on decompressed size, rate limits per client,
  and a bound on any user-controlled iteration count. See
  [code-review.md § Security](code-review.md).
- **Container hardening:** run as a non-root user, read-only root filesystem
  where possible, drop all capabilities, set resource requests and limits. A Go
  binary is a static artifact — it should be running in a near-empty image with
  no shell to be exploited.
- Vulnerability scanning is `govulncheck` in CI — see
  [repo-and-ci.md](repo-and-ci.md).

---

## Data: migrations, transactions, outbox

### Migrations

- Use a migration tool (`goose`, `atlas`, `golang-migrate`) — never hand-applied
  SQL. Migrations are **checked into the repo** and **applied by CI/CD**, not by
  a human at a psql prompt.
- **Reversible where feasible.** When a down-migration is genuinely impossible
  (a destructive backfill), say so explicitly in the migration file rather than
  shipping a down that silently does nothing.
- **Safe for rolling deploys.** During a rollout, old and new code run
  simultaneously against one schema, so a migration must be compatible with
  both. Destructive changes are a multi-deploy sequence, never one step:

  | Goal | Wrong (breaks the rollout) | Right |
  |---|---|---|
  | Rename a column | `ALTER … RENAME` | add new → dual-write → backfill → switch reads → drop old |
  | Drop a column | `DROP COLUMN` in the same deploy | stop writing → deploy → drop in a later migration |
  | Add a `NOT NULL` column | add with `NOT NULL`, no default | add nullable → backfill → add constraint |

- Adding an index on a large table locks writes on some engines — use the
  concurrent form and expect it to run outside a transaction.

### Transactions

- **Keep transaction scope small.** Never hold a transaction across a network
  call to another service; that couples your database's lock duration to a third
  party's latency.
- Pass `context.Context` into every DB operation so a cancelled request releases
  its connection.
- Be explicit about isolation level when the default is not right, and comment
  *why* — the next reader cannot infer it.

### The outbox pattern

**"Write to the database, then publish an event" is a distributed-transaction
bug.** If the publish fails after the commit, the event is lost forever; if the
commit fails after the publish, consumers act on something that never happened.

Instead: write the event to an `outbox` table **inside the same transaction** as
the state change, and have a relay (poller or CDC stream) publish committed rows
and mark them sent. This makes delivery at-least-once, which is why consumers
must be idempotent — see [idempotency keys](#timeouts-retries-idempotency).

---

## Performance work

**Measure first.** Every rule here is conditional on a profile showing the
problem; applied speculatively they cost readability and buy nothing.

- **`net/http/pprof` + `go tool pprof`** for CPU and heap profiles. In production,
  expose pprof on a **separate internal-only listener**, never on the public
  serving port — the profile endpoints are both expensive and revealing.
- **`go build -gcflags="-m"`** to see escape-analysis decisions when allocation
  volume is the problem.
- **`go test -bench=. -benchmem`** to quantify before and after. A performance
  change without a benchmark diff in the PR description is a guess.
- **`sync.Pool`** for genuinely high-frequency, short-lived allocations of the
  same type — after a profile shows GC pressure, not before. Misused, it holds
  memory live and slows things down.
- Prefer `strconv` over `fmt` for primitive-to-string conversion on hot paths,
  and give `make()` a capacity hint when the size is known — these two are cheap
  enough to apply without a profile.

---

## The runbook

Every service ships a runbook — in the repo, next to the code, reviewed like
code. Minimum contents:

- **How to roll back**, with the exact command. During an incident nobody
  reconstructs this from CI config.
- **Where the dashboards and logs are** — direct links, not "in Grafana".
- **How to enable `pprof` safely in production**, including which port and how
  it is access-controlled.
- **The top three alerts**: what each one means, what it does *not* mean, and the
  first mitigation step for each.
- **Credential rotation** for any static secret the service holds.

An [ADR](engineering-policy.md) records *why* the architecture is what it is; the
runbook records *what to do at 3am*. Both are required, and they are not the same
document.

---

## Sources

- [metalagman `go-senior-developer`](https://github.com/metalagman/agent-skills) — retries/idempotency, API error contract, health endpoints, observability signals, 12-factor selection, migrations/outbox, runbook contents, profiling tooling. Rules extracted and rewritten; see the skill's source-versions notes.
- Cardinality discipline, the liveness-vs-readiness failure modes, the rolling-deploy migration table, the retry-multiplication rule and the secret-redaction control are this skill's additions.
