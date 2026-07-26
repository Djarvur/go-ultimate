# Backend Service Architecture — Bounded-Context Hexagonal

**Source:** condensed from [powerman `go-bounded-context-hexagonal` v0.6.0](https://github.com/powerman/skills)
(~710 lines). For full reasoning on every edge case, read the upstream SKILL.md.

**Applies to:** backend services (HTTP/gRPC/messaging). For CLI, library, or
script layouts, see [project-layouts.md](project-layouts.md).

This reference wins over generic architecture/design-pattern guidance for
service layout, package boundaries, ports/adapters, and wiring. Sibling skills:
`golang-design-patterns` stays useful as background reference and for adapting
third-party codebases; [engineering-policy.md](engineering-policy.md) still
governs non-architectural Go conventions.

Apply during: architecture discussions, package-layout design, refactoring an
app structure, designing CLI/backend services, modular-monolith design, and
testing-strategy discussions for application boundaries.

## Contents

- [Core model](#core-model)
- [Layouts — three variants](#layouts-three-variants)
- [Ports](#ports)
- [Adapters](#adapters)
- [Wiring — `wire.go` is mandatory in every layout](#wiring-wirego-is-mandatory-in-every-layout)
- [Shared leaf packages (multi-app repos only)](#shared-leaf-packages-multi-app-repos-only)
- [Naming](#naming)
- [Modular monolith](#modular-monolith)
- [Testing strategy (architecture-level)](#testing-strategy-architecture-level)
- [Decision checklist](#decision-checklist)
- [What to avoid](#what-to-avoid)
- [Source](#source)

---

## Core model

- **One application = one bounded context** (owned by one team). Do not introduce
  extra internal bounded-context-like boundaries by default.
- **One `App` In port** for the whole application.
- **One `Repo`/DAL Out port per owned database.**
- **The application is the sole owner of its database data.** Other apps use its
  API, or in a modular monolith call its `port.InprocApp`.
- **A binary ≠ an application.** A binary is a deployment unit. One binary may
  include multiple applications; the same application may live in different
  binaries. Importable application code must stay importable (not buried under
  executable-local `internal/`).
- **Business logic lives in one `app` package by default.** Do not split into
  `domain/` + `application/` unless there is a real need (cycle break, etc.).

### `app` internal growth order

1. Keep business logic in one `app` package.
2. Split it into files first.
3. If needed, add helper subpackages under `internal/app/...` while keeping `app`
   the facade.
4. If it still feels too large, re-check whether the bounded context is too wide.

### Adapter discipline (non-negotiable)

Adapters may do **only** three things:
1. Talk to the outside world.
2. Validate external protocol or transport shape.
3. Convert between external formats and application-facing types.

**Adapters must not contain business decisions.** The business-logic package
(`app`) must not import `net/http`, `os`, or transport SDKs.

---

## Layouts — three variants

Choose the layout shape before proposing a package tree. **Start with the
simplest layout that honestly fits today; move to a bigger one only when a
concrete need appears — not because growth is imaginable.**

Transition signals:
- **Flat → Default**: when the app stops feeling flat, adapters multiply, tests
  become awkward, or `app` wants helper subpackages.
- **Default → Importable**: when another binary/package/repo must call the app
  through a stable in-process API.
- **Never skip straight to a more complex layout on imagined future growth.**

### Flat — tiny executable

```text
tool-basic/
├── go.mod
├── main.go
├── wire.go                     Returns a Runtime + startable units.
├── port.go                     App + other Out ports + port-level errors.
├── app.go                      Implements port.App.
├── redis.go
└── ...
```

Keep responsibilities separated even if files share a package. Do not move wiring
into `main.go` just because the app is small. Collapse packages only when truly
tiny; grow into Default first.

### Default — app private to its executable

```text
tool/
├── go.mod
├── main.go
├── wire.go                     Returns a Runtime + startable units.
└── internal/
    ├── port/
    │   ├── port.go             App + Repo + other Out ports + port-level errors.
    │   └── types.go            Optional boundary DTO/types file.
    ├── app/
    │   ├── app.go              Implements port.App.
    │   └── ...
    ├── in/
    │   ├── http/adapter.go
    │   └── nats/adapter.go
    ├── dal/
    │   ├── adapter.go
    │   └── migrations/
    │       └── 00001_some_name.sql
    └── out/
        ├── nats/adapter.go
        └── redis/adapter.go
```

`wire.go` may live in `package main`. Keep the in-process contract in
`internal/port`; `internal/app` implements `port.App`.

### Importable — app importable by other binaries/modules/repos

```text
modular-monolith/
├── go.mod
├── api/                        External wire contracts: proto, events, OpenAPI, etc.
├── apps/
│   ├── apps.go                 Composition layer wiring the apps a binary includes.
│   └── exampleapp/             Suffix "…app" is optional.
│       ├── wire.go             Public wiring API; returns a Runtime + startable units.
│       ├── integration_test.go Integration smoke tests for the whole app.
│       ├── port/               PUBLIC — the bounded-context contract.
│       │   ├── port.go         App + Repo + other Out ports + InprocApp in-port + errors.
│       │   └── types.go        Optional boundary DTO/types file.
│       └── internal/
│           ├── app/            PRIVATE — implements port.App.
│           │   ├── app.go
│           │   └── ...
│           ├── in/
│           │   ├── grpc/adapter.go
│           │   ├── http/adapter.go
│           │   └── nats/adapter.go
│           ├── dal/
│           │   ├── adapter.go
│           │   └── migrations/
│           │       └── 00001_some_name.sql
│           └── out/
│               ├── nats/adapter.go
│               └── redis/adapter.go
├── cmd/
│   ├── example/                Single-app binary: main.go calls exampleapp.Wire directly.
│   │   └── main.go
│   └── monolith/               Multi-app binary.
│       ├── main.go
│       └── wire.go             Composes the embedded apps via apps.go.
├── dom/                        Optional shared pure business types if public outside repo.
├── internal/
│   └── dom/                    Optional shared pure business types if only repo-internal.
└── platform/                   Shared infra helpers.
    ├── log/
    ├── metrics/
    ├── repo/
    └── serve/
```

Wire-format split: **`api/*` defines wire formats between processes; `port/*`
defines the in-process contract; `internal/in/*` adapts `api/*` or
transport-specific DTOs to `port.App`; `internal/out/*` and `internal/dal/*` may
depend on `api/*` when external wire formats are shared.** `integration_test.go`
next to `wire.go` is the recommended first place to smoke-test the whole app.

---

## Ports

### One `port.go`, all interfaces together

Keep `App`, `Repo`, other Out ports, and port-level errors in **one `port.go`**
in the `port` package. Optional splits: DTO-like types → `types.go`; errors →
`errors.go` (only if `port.go` gets noisy). Keep them in the same `port`
package — errors are part of the contract, not implementation details.

### Keep `port` outside `app` except in the Flat variant

`port` is the contract; `app` is its implementation. Once `app` grows helper
subpackages, a `port.go` inside `app` forces those subpackages to import `app`
to reach the contract, while `app` itself needs to import its helpers — that
creates avoidable import-cycle pressure around the very contract that should stay
dependency-light.

Defaults: **Flat** = one package OK; **Default** = `internal/port` +
`internal/app`; **Importable** = public `port/` + private `internal/app`. Do not
collapse them as the growth path for non-flat layouts.

### What `port` may import — the leaf tier rule

A `port` is the most-depended-upon package of its app: adapters, wiring, tests,
and in-process callers of sibling apps all import it. To stay universally
importable without cycles it must sit at the **leaf tier** and import only:

- the standard library;
- pure value/vocabulary libraries that behave like extended stdlib (value
  semantics, no I/O or transport, no dependency on this repo's apps or app-code
  — e.g. a leak-safe secret handle or a ULID type);
- the two shared leaf packages `dom` and `xport` (see below). These two exhaust
  the shared-leaf need — avoid inventing a third.

It must **NOT** import another application's `port`, any adapter, or any
non-leaf shared package (transport helpers, clients, logging). Avoid even an
*acyclic* `port → other-port` edge: since cyclic in-process app dependencies are
normal, it turns a future cycle into a compile break in the worst place. Express
a cross-app dependency as an interface in leaf terms, owned by the consumer's
`port` or by `xport`, with the implementation injected at the composition root —
so a `port` never names a sibling's `port` or any app-code.

This constrains *architectural* dependencies (app-code, adapters, other apps'
ports), not third-party test libraries. Co-locating a generated mock with its
interface (in `port` or `xport`) is a free choice.

### `App` vs `InprocApp`

- **`port.App`** — the business interface implemented by `internal/app`, called
  by the application's *own* inbound adapters. Never expose `internal/app` as
  public API.
- **`port.InprocApp`** — declared **only in the Importable layout**. The in-port
  for callers (sibling app, CLI command) that reach the app *in-process* rather
  than over the network. It is the hexagon's inbound adapter for the in-process
  transport, the peer of the gRPC/HTTP adapters.

`InprocApp` exists because an in-process caller must go through an inbound
adapter before hitting business logic. The in-adapter:

- prepares the execution environment the business logic expects (context setup,
  instrumentation, logging, deadlines, admission/rate control — which may even
  reject the call);
- propagates out-of-band request metadata (request-id, trace-id, deadline, auth
  tokens) so it keeps flowing when the business logic goes on to call further
  applications.

The concrete adapter behind `InprocApp` is an ordinary thin inbound adapter —
hand-written or generated from `port.App` — whose type name and package are an
implementation detail, not part of the contract.

**Authenticated identity** should be an **explicit parameter** on `App` methods
that need it (e.g. `dom.Auth`), not pulled from context inside `app`. Resolving
that identity (from a token, a session, or out-of-band metadata) is the inbound
adapter's job; it passes the resolved value in. This turns "forgot to check
authorization" into a compile error and keeps `app` free of transport-resolution
logic. It is also exactly why `InprocApp` is not a verbatim copy of `App`: it
omits the request-scoped inputs the adapter resolves (auth above all), so an
in-process caller supplies only the business arguments.

Flat and Default layouts need no `InprocApp`: the app is reached only by its own
executable, which calls `port.App` directly. `InprocApp` is *more correct* in any
layout but only *necessary* in Importable; elsewhere it is ceremony this
architecture avoids. Even there, reach the app through public `port` interfaces
(CLI without a fake `in/cli` adapter; tests through public methods), never
`internal/app`.

### Repo is a transaction boundary

Treat the DAL port as a Transaction-Script-style boundary:
- **Task-centric methods** that reflect business operations and their
  transactional needs — *not* entity-shaped `Get/Save` repositories.
- Do **not** invent extra transaction abstractions (`app/tx.go`) unless a
  concrete case requires them.
- This gives clearer transaction boundaries and lets SQL be optimized per task.

Like `app`, the internal structure of `dal` is free to follow the actual
persistence complexity: one package, then files/helpers/private subpackages.

---

## Adapters

| Directory | Purpose | Notes |
|---|---|---|
| `internal/in/<proto>/` | Inbound transport/messaging adapters (HTTP, gRPC, NATS/Kafka subscriber) | **Never create `internal/in/cli`.** CLI lives in `main`/`cmd/`. |
| `internal/dal/` | Owned-database adapter + migrations | Separate from `out/` because it owns transactions and migrations. |
| `internal/out/<dep>/` | Other outbound adapters (Redis, NATS publisher, email, 3rd-party clients) | May depend on `api/*` when external wire contracts are shared. |

### CLI lives in `main`

For CLI binaries: treat `main.go` or `cmd/*/main.go` as the CLI adapter; wire
the app in `wire.go`; call the app's in-process API directly (`port.App`, or
`port.InprocApp` when present). This avoids fake abstraction layers for Cobra,
Kong, urfave/cli, and similar frameworks.

---

## Wiring — `wire.go` is mandatory in every layout

`wire.go` does wiring only: build the dependency graph; prepare already-
configured application and adapter objects; return values ready to be started or
used by the caller. **`wire.go` must NOT start servers, background workers, or
lifecycle-managed processes** — that would make the same wiring harder to reuse
from CLI code and integration tests. **Do not put dependency-graph construction
into `main.go`** — `main.go` only coordinates flags/env, process lifecycle,
graceful shutdown, and starting/stopping the values returned by `wire.go`.

### One wiring entrypoint

Recommend one main wiring entrypoint per application. The API convenient for the
app's own integration tests usually also works for a single-app binary, a
monolith binary, and integration tests in other repos. Only introduce multiple
entrypoints when use cases truly diverge.

### Design the returned value for all main consumers

The main wiring entrypoint should usually return a value that can satisfy all
key consumers with minimal adaptation:
- **CLI code** needs direct access to the application's in-process API.
- **Server code** needs the prepared inbound adapters or startable components.
- **Integration tests** may need mock injection plus `port.App` access for
  introspection.

Stable rule: make wiring explicit, reusable, testable, and separate from
`main.go`. Do not optimize `wire.go` for one executable in a way that makes the
same application harder to reuse elsewhere.

### Recommended wiring shape (reference, not required)

```go
// Wire builds the application and its startable units.
// - ctx:        long-lived base context for the whole process.
// - startupCtx: bounds wiring work only (timeouts for deps connecting).
// - cfg:        business configuration (already Resolved).
func Wire(ctx, startupCtx context.Context, cfg Config) (*Runtime, []Unit, error) {
    // ... construct adapters, app ...
    return &Runtime{app: app, db: db, ...}, units, nil
}
```

A `Runtime` carries the application boundary, health/readiness, owned resources,
and a single `Close`. Startable units are descriptors run by a small loop in
`main` that owns signals and graceful shutdown. This keeps `wire.go` reusable
verbatim from a server, a CLI command, and integration tests. For the full
reference implementation (Runtime shape, serve loop, CLI reuse), see
[wiring-and-runtime.md](wiring-and-runtime.md).

---

## Shared leaf packages (multi-app repos only)

Two shared leaf packages exist; together they exhaust the shared-leaf need.
Avoid inventing a third.

### `dom/` — shared pure business types

Locations: `dom/` when shared types are part of public repo API;
`internal/dom/` when shared only inside the repository. Good examples: `Money`,
`Email`, `CountryCode`, `TimeRange`. Must stay **pure** — no dependencies on
ports, adapters, or infrastructure. Do not turn it into a generic dumping ground.

### `xport/` — shared application types (`internal/xport`)

The second shared leaf: the neutral home for a contract that some apps' `port`
must reference but that **no single app's `port` can or should own**. The name
reads as the cross(-app) `port` tier.

**Litmus:** if exactly one app can own it, it belongs in that app's `port`, not
here.

Two kinds live there, with opposite rationales:

- **Cross-cutting DIP interfaces — deliberate and healthy.** The consumer-owned
  interface for an in-process dependency on another app, phrased in leaf terms
  so a `port` can depend on it without importing the provider's `port`.
- **Cycle-forced DTOs — a smell marker; minimize.** When two apps form an
  in-process dependency cycle, the DTOs each side needs cannot stay in either
  `port` (they would import each other); moving them here breaks the cycle.
  Prefer removing these over adding them.

Keep `xport` a leaf holding interfaces and DTO/value types, **not**
implementations.

> **Breaking change vs older drafts:** this package was previously documented
> here as `internal/commonport/`. That name is obsolete; v0.6.0 of the source
> skill renamed it to `internal/xport`.

---

## Naming

Prefer short, high-signal names for packages and frequently used identifiers
(short names matter most for packages referenced often in code; for packages
that mostly stay in the directory tree, clarity and local convention matter
more). Recommended package names: `dom`, `app`, `dal`, `srv`, `svc`, `sub`,
`repo` (when `dal` is not the better fit).

**Do not repeat the same business noun at every level.** If the directory
already says `order_service`, do not add `order_service.go`, `order/order.go`,
or `service/order_service.go`. Prefer consistent generic filenames where the
package gives context: `adapter.go` (main adapter), `app.go` (main business
logic), `port.go` (main port contract), `wire.go` (wiring). Use `New` as the
constructor name when the package already explains what is being created.

For an importable application package, an `app` suffix on the package itself
(`apps/exampleapp`, `package exampleapp`) is optional but convenient: it lets
embedders import it without an alias and avoids a generic, shadow-prone bare
noun. If used, keep the bare context name for runtime identity
(`port.AppName`, env prefixes, asset paths).

---

## Modular monolith

In repositories with multiple `apps/*`, treat each application as the same
module you would otherwise deploy as a separate service. Moving them into one
binary changes transport and wiring, **not** ownership, dependency mapping, or
interaction design.

`apps/apps.go` is the composition root. It owns no bounded context itself — it
is a composition root, **not** a service locator and not a dumping ground.

Mandatory guidelines:

- Each application is an application module: keep its `port` boundary,
  ownership, and interaction design exactly as if it were a separate service.
- Cross-app data ownership still applies: apps do not read each other's data
  directly from the database (enforce operationally with separate credentials
  even on a shared DB).
- An app that needs a sibling depends on that sibling's `port.InprocApp`,
  injected by the composition layer after wiring — **not** on the whole
  composition layer.
- Document the interaction map the same way you would for separate services when
  it stops being obvious from the directory tree alone.

### Recommended composition style (reference)

A recommended (not required) implementation treats the composition layer as an
*active accumulator* rather than a passive struct of boundary fields:

- it wires the included apps in parallel and bounds startup with a shared
  startup context;
- it collects each app's startable units, health, and closer, exposing one
  aggregate `Close` and health;
- it injects late cross-app dependencies once all apps exist, via a `Setup` step
  on each app — explicit two-phase init that also supports cyclic A↔B
  dependencies.

For the full reference composition implementation (registration, parallel
wiring, the `InprocApp` in-adapter contract, what may import what across apps),
see [modular-monolith.md](modular-monolith.md).

---

## Testing strategy (architecture-level)

- Generate mocks for all In/Out ports.
- Business logic → unit tests with mocked Out ports.
- In adapters → unit tests with a mock `port.App`.
- DAL adapter → integration tests against a real temporary database.
- Other Out adapters → integration tests against fake or real network services
  started by the test.
- Services → smoke tests for each external API entrypoint and each subscriber.

For importable applications, external repositories' tests may import the app and
start it via public wiring APIs — that is a valid use case and should shape the
package-mode choice.

---

## Decision checklist

When proposing a service design or refactor:

1. Is the application reusable or private? (picks layout)
2. One `App` boundary for the whole bounded context.
3. One Repo/DAL port per owned DB.
4. `wire.go` explicit and mandatory.
5. Importable apps exposed through `port.InprocApp` (built on `port.App`), never `internal/app`.
6. No `internal/in/cli`.
7. Adapters thin.
8. Package tree flat: `in/`, `out/`, `dal/`, `app/` — not deep `adapter/primary/secondary/...`.
9. `dom/` only for truly shared pure types in multi-app repos.
10. If `app` or `dal` grows: split by files → helper subpackages → question the bounded context.
11. Modular monolith: each `apps/*` is a module; assemble in `apps/apps.go`; inject sibling `InprocApp`, not the composition layer.
12. Inject a sibling's `port.InprocApp` after wiring, not the whole composition layer.
13. Document the application-module map when cross-application interactions stop being obvious from the directory tree.

---

## What to avoid

- Splitting every app into `domain/` + `application/` + `service/`.
- One repo per entity/aggregate by default.
- Deep `adapter/primary/secondary` directory trees.
- `internal/in/cli`.
- Putting all wiring into `main.go`.
- Exposing concrete internal app types instead of `port.App`.
- Placing importable application code under executable-local `internal/`.
- Treating multi-app repos as if apps should import each other's internals.
- Turning `apps/apps.go` into a service locator or dumping ground.
- Handing an app the whole composition layer instead of the specific sibling `InprocApp`(s).
- `port` importing another app's `port`, an adapter, or any non-leaf shared package.
- Inventing a third shared leaf package beyond `dom` and `xport`.

---

## Source

Condensed from powerman's `go-bounded-context-hexagonal` v0.6.0. For the full
treatment of `InprocApp` semantics, two-phase modular-monolith init, and the
reference wiring/runtime implementation, read the upstream SKILL.md:
https://github.com/powerman/skills/blob/main/go-bounded-context-hexagonal/SKILL.md

The two companion references — [wiring-and-runtime.md](wiring-and-runtime.md)
(the `Wire()` / `Runtime` / serve-loop pattern) and
[modular-monolith.md](modular-monolith.md) (composition root, two-phase init,
in-process adapter) — are ported verbatim from the upstream skill's
`references/` directory.
