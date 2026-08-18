# MCP Servers in Go

**Source:** [go-sdk `design/design.md`](https://github.com/modelcontextprotocol/go-sdk/blob/main/design/design.md)
(primary, natively Go), [Phil Schmid — MCP best practices](https://www.philschmid.de/mcp-best-practices),
[modelcontextprotocol.info best-practices](https://modelcontextprotocol.info/docs/best-practices/)
(secondary, translated from language-agnostic).

**Applies to:** servers exposing tools / resources / prompts to LLMs over the
Model Context Protocol. For general backend services see
[architecture.md](architecture.md); for LLM-driven agents see
[agents.md](agents.md).

This reference wins over generic server/REST guidance for MCP server layout,
SDK usage, tool-handler design, and the MCP-specific error/pagination/logging
conventions below.

**Tracks `go-sdk` v1.7.0** (protocol revision `2026-07-28`). Where a rule differs
between protocol revisions, the revision is named.

## Contents

- [SDK choice](#sdk-choice)
- [Protocol revisions and statelessness](#protocol-revisions-and-statelessness)
- [API-shape rules (from go-sdk `design.md`)](#api-shape-rules-from-go-sdk-designmd)
- [Auth and context injection — via standard HTTP middleware](#auth-and-context-injection-via-standard-http-middleware)
- [Typed tool handlers (flagship)](#typed-tool-handlers-flagship)
- [Two-channel errors](#two-channel-errors)
- [Tool design discipline (agent-UI, not REST wrapper)](#tool-design-discipline-agent-ui-not-rest-wrapper)
- [Pagination and cacheable lists](#pagination-and-cacheable-lists)
- [Logging, cancellation, middleware](#logging-cancellation-middleware)
- [Subscriptions, resources, notifications](#subscriptions-resources-notifications)
- [Production hardening](#production-hardening)
- [What to avoid](#what-to-avoid)
- [Sources](#sources)

---

## SDK choice

**Default: the official [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).**

It is the forward path, tracks the MCP spec, and is the source of every
API-shape rule in this reference. [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go)
is a legitimate alternative with a larger installed base and more community
examples, but it is **not the default** — recommending it as default would
contradict this skill's own API-shape guidance (mark3labs uses variadic
options; the official SDK deliberately does not).

Do not pin a Go MCP SDK in project examples without a one-line rationale.

---

## Protocol revisions and statelessness

The `2026-07-28` revision (shipped in go-sdk v1.7.0) largely rewrote the wire
protocol. **You do not have to act on most of it** — the SDK negotiates the
highest mutually-supported revision at connect time and keeps `2025-11-25`
working — but three consequences change how you build a server.

### 1. Stateless is the new default posture

The `initialize` / `notifications/initialized` handshake is gone. Each request
carries its own `_meta` (protocol version, client info, capabilities), and a new
`server/discover` RPC lets a client learn capabilities up front. Resumability
(`Last-Event-ID`, the standalone GET stream) is removed, as are `ping`,
`logging/setLevel`, `resources/subscribe` and `resources/unsubscribe`.

**Over streamable HTTP the new revision is only served when you opt in:**

```go
handler := mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
    Stateless: true, // required to serve protocol 2026-07-28
})
```

Leave `Stateless` false and clients negotiate down to `2025-11-25` — which works,
but forfeits the new revision. In stateless mode the server ignores session IDs
entirely, answers `DELETE` with `405`, and **rejects any server-to-client
request outright** — there is no session for the client to answer on. That
restriction is exactly why multi-round-trip requests exist (below).

**Design for statelessness anyway.** A server that keeps per-session state in
memory cannot scale horizontally behind a load balancer and loses that state on
every deploy. Put state in the request or in a store — the same
[state-externalism](agents.md) rule agents follow.

### 2. Server-initiated calls became multi-round-trip requests

Elicitation, sampling and roots are no longer fresh server-to-client JSON-RPC
requests. A handler instead returns a result carrying `InputRequests` plus an
opaque `RequestState`; the client fulfils them and **retries the original call**
with `InputResponses` populated. `CallToolResult.NeedsInput()` reports this case,
and the SDK ships middleware that handles the exchange transparently in both
directions, including a shim for legacy clients.

Two implications worth internalizing:

- **A handler can now be called more than once for one logical operation.** Make
  it idempotent, or carry what it needs in `RequestState`.
- **`RequestState` crosses the network to an untrusted peer.** The SDK's own
  documentation is explicit: an unauthenticated server must encrypt, sign and
  verify it. Treat it as a cookie, not as scratch space.

### 3. Roots, sampling and logging are deprecated

Formally deprecated on this revision (SEP-2577). The Go types remain and stay
functional for a documented window of **at least twelve months**, so nothing
breaks — but do not build a new server whose design depends on them. See
[§ Logging](#logging-cancellation-middleware) for what to do instead.

---

## API-shape rules (from go-sdk `design.md`)

These describe the SDK you build on; mirror them in any wrapper or facade you
add over it.

- **Single shared `mcp` package** for the user-facing API (mirrors `net/http`,
  `google.golang.org/grpc`). Functionality not directly MCP-related
  (`jsonschema`, `jsonrpc2`) lives in separate packages; JSON-RPC stays
  internal.
- **Spec-method signature: `(context.Context, *XXXParams) (*XXXResult, error)`.**
  It must **always be valid to pass `nil` for `*Params`** that isn't currently
  needed — forward-compatible if the spec adds a property.
- **`Client`/`Server` are stateful feature registries; `ClientSession`/
  `ServerSession` are logical connections.** A `Server` may hold multiple
  connections; adding a tool notifies all connected peers. Sessions are created
  via `Connect(ctx, Transport)`; `Server.Run(ctx, Transport)` covers the common
  "run until client disconnects" case.
- **Low-level `Transport` interface** (`Connect → Connection{Read, Write,
  Close}`), not higher-level `SendNotification`/`MakeCall`. Lower-level is
  easier to implement for custom transports.
- **Distinct types for client-stdio vs server-stdio transport**
  (`CommandTransport` runs a subprocess; `StdioTransport` binds `os.Stdin`/
  `os.Stdout`) — for future-proofing.
- **HTTP server transports are `http.Handler`s** and accept a
  `getServer func(*http.Request) *Server` callback for per-session
  customization (return the same server for stateless).

---

## Auth and context injection — via standard HTTP middleware

**Never** hook authentication or context injection into transport-level hooks.

Use **standard `net/http` middleware**. The SDK deliberately omits
transport-level hooks for these concerns because HTTP middleware is the
idiomatic, composable place for them.

---

## Typed tool handlers (flagship)

Tool handlers are **typed via generics**:

```go
// mcp/server.go
func AddTool[In, Out any](s *Server, t *Tool, h ToolHandlerFor[In, Out])

// mcp/tool.go — the handler shape
type ToolHandlerFor[In, Out any] func(
    context.Context, *CallToolRequest, In,
) (*CallToolResult, Out, error)
```

`In` and `Out` are inferred from the handler, so call sites read
`mcp.AddTool(server, &mcp.Tool{...}, handler)` without explicit type arguments —
see [`assets/mcp-server/main.go`](../assets/mcp-server/main.go).

> **Do not copy the handler signature out of the SDK's `design/design.md`.** That
> document still shows the pre-1.0 shape
> (`func(ctx, *ServerRequest[*CallToolParamsFor[In]]) (*CallToolResultFor[Out], error)`);
> those types do not exist in the released `mcp` package. The signature above is
> the one in v1.7.0 — **unchanged since v1.6.1**, despite the `2026-07-28`
> protocol rewrite. The typed-handler surface is the stable part of this SDK;
> the churn is at the transport and notification layers.

- `mcp.AddTool[In,Out]` **infers `InputSchema` from `In`** (if nil) and
  `OutputSchema` from `Out` (if nil, unless `Out` is `any`); it will not modify
  schemas you set explicitly.
- **Dual schema approach**: reflection (`jsonschema.For[T]`) **or** explicit
  JSON Schema literals — your choice. Reflection by default; explicit when the
  inferred schema is wrong or incomplete.
- Provide **only** `Server.AddTool` + generic `mcp.AddTool` + `RemoveTools`.
  Do not invent `SetTools` / `AddSessionTool` variants in your wrappers.

### Adapter carve-out (the SDK import rule)

The existing rule — *"business-logic packages never import `net/http`, `os`, or
transport SDKs"* — gets a **narrow carve-out** for MCP handlers:

- **The MCP handler package is an inbound adapter.** It may import `mcp.*` in
  its signatures and is allowed to call business logic.
- **Pure business-logic packages stay SDK-free** and are invoked *by* the
  handler.

This preserves the architectural boundary (business logic knows nothing of
transport) while making typed handlers usable. The carve-out is narrow: only
the handler package, only `mcp.*` types.

---

## Two-channel errors

MCP has **two distinct error channels**; conflating them is the most common
bug.

1. **Tool-execution failure → `*mcp.CallToolResult{IsError: true}` + an
   actionable string.** The agent sees this *inside* the result and can
   self-correct. This is **not** a protocol error.
   - The string is context: explain what failed and what argument shape would
     work. Do not panic, do not return a bare `"error"`.
2. **Protocol / transport failure → wrapped Go error.** Return a normal
   `error` from the handler; it propagates as a `JSONRPCError` carrying
   `Code`/`Message`/`Data`. Read it with `errors.As(err, &jsonrpcErr)` to
   inspect `.Code` — never `==`.

> Tool errors are *for the agent* (self-correction); protocol errors are *for
> the caller* (the host runtime). Never raise a tool failure as a protocol
> error.

---

## Tool design discipline (agent-UI, not REST wrapper)

**MCP is a UI for agents, not a REST wrapper.** Design tools for an agent user,
not a developer user.

- **Outcomes, not operations.** Design tools around the agent's goal, not CRUD
  endpoints. One `track_latest_order(email)` beats three round-trips that
  force the agent to store intermediates in its context window.
- **Orchestrate server-side.** Do multi-step flows in your code, not in the
  LLM's context window.
- **Flatten arguments.** Top-level typed structs with constrained types
  (string enums) + defaults. Avoid nested dicts / `map[string]any` in tool
  args — they raise hallucination and mis-selection.
- **5–15 tools per server; one server, one job.** Curate ruthlessly; split by
  persona (admin/user) when a server grows past ~15 tools.
- **Name tools for discovery.** Service-prefixed, action-oriented:
  `{service}_{action}_{resource}` (e.g. `slack_send_message`,
  `linear_list_issues`).
- **Curate payloads.** Do not dump raw large data into the response — protect
  the context window. Return summaries + pagination, not full dumps.
- **Tool descriptions are context.** Every description specifies *when to use*,
  *how to format args*, *what to expect back*. (Go: doc comment +
  `Tool.Description`.)

---

## Pagination and cacheable lists

- **Keyset pagination by default** — unique ID as key, stable sort, opaque
  cursor string. Offset-based only when keyset is impossible.
- `PageSize` is a `ServerOptions` field; non-negative; zero ⇒ `DefaultPageSize`
  (1000).
- Expose **`iter.Seq2`** iterator helpers for `List*` methods (auto-pagination
  over the SDK).
- For agent-facing results, include `has_more`, `next_offset`, `total_count`.
  Never load all results into memory.

### Freshness hints (`2026-07-28`)

List results (`tools/list`, `prompts/list`, `resources/list`,
`resources/templates/list`, `resources/read`, `server/discover`) embed a
`Cacheable` struct that clients and intermediaries honour to stop polling:

```go
type Cacheable struct {
    TTLMs      int    `json:"ttlMs"`      // 0 ⇒ immediately stale
    CacheScope string `json:"cacheScope"` // "public" (default) | "private"
}
```

- **Set `TTLMs` on list results that are genuinely stable.** A tool list that
  changes only on deploy should not be re-fetched on every turn.
- **`cacheScope: "private"` for anything user-scoped.** The default is `public`,
  which permits a shared intermediary to serve one user's result to another. If
  the list depends on who is asking, saying so is a security requirement, not an
  optimization.

---

## Logging, cancellation, middleware

- **Logging — `log/slog`, but no longer bridged to MCP for new servers.** The
  MCP logging feature (and `mcp.LoggingHandler`, the SDK's `slog.Handler` that
  forwarded records as `notifications/message`) is **deprecated as of protocol
  `2026-07-28`** (SEP-2577). It stays functional for at least twelve months, so
  existing servers are not broken and may keep using it for older peers.

  For anything new: author logs with `log/slog` as usual and **write them to
  stderr**, where the host runtime collects them. For a stdio server stderr is
  the only channel that is not the protocol — writing logs to stdout corrupts
  the JSON-RPC stream — and for an HTTP server it is the ordinary observability
  baseline ([engineering-policy.md](engineering-policy.md)). Do not design a
  server whose only observability path is MCP logging notifications.

  Structured attrs stay the same: method, duration, client_id, error,
  error_type.
- **Cancellation = transparent context cancellation.** Cancelling the caller's
  context sends `notifications/cancelled` and the call **returns immediately
  with `ctx.Err()`** — it does not wait for the server. The server observes
  client cancellation as a cancelled context.
- **Progress** is reported via `NotifyProgress` **only if the peer supplied a
  progress token** in `Meta.ProgressToken`.
- **Middleware = `func(MethodHandler) MethodHandler`** (handler-to-handler).
  Split **Receiving vs Sending** (`AddReceivingMiddleware` /
  `AddSendingMiddleware`). Applied **right-to-left**: `AddMiddleware(m1,m2,m3)`
  yields `m1(m2(m3(handler)))`. Use it for rate limiting, authz, logging,
  tracing.

---

## Subscriptions, resources, notifications

- **`ResourceHandler` returns a `*ReadResourceResult`** (not raw content) for
  forward-spec compatibility.
- **Feature-specific notification handlers** (`ToolListChangedHandler`,
  `ResourceListChangedHandler`, …), not a generic `OnNotification`. Type-safe;
  hides JSON-RPC details. This is still the API you write against — what changed
  underneath is how those notifications reach the client.

### The subscription model changed in `2026-07-28`

- **One `subscriptions/listen` stream replaces free-floating notifications.**
  `tools/list_changed`, `prompts/list_changed`, `resources/list_changed` and
  `resources/updated` are no longer separate notifications; they are multiplexed
  onto a single long-lived stream, each tagged with a subscription id. The SDK
  opens it automatically on `Client.Connect` when the matching list-changed
  handler is set, and servers route notifications only to subscribed sessions.
- **`resources/subscribe` and `resources/unsubscribe` are removed** on this
  revision and rejected with `MethodNotFound`. `SubscribeHandler` /
  `UnsubscribeHandler` still exist in `ServerOptions` and still serve older
  peers, and the **both-or-neither rule still holds** — supplying one without the
  other panics at `NewServer`, so it is a startup crash, not a runtime surprise.
- **Do not design a new server around resource subscriptions.** They work only
  for pre-`2026-07-28` clients. Prefer a tool the agent can call, or the
  freshness hints in [§ Pagination and cacheable lists](#pagination-and-cacheable-lists).

---

## Production hardening

- **Bind `127.0.0.1` for non-networked servers.** Local-only by default.
- **Defense in depth**: Network → Auth → Authz → Validation → Output
  sanitization, layered.
- **Fail safe**: circuit breaker (`sony/gobreaker`), cache fallback, rate
  limiter (`golang.org/x/time/rate`, token bucket).
- **Error classification**: client_error (4xx) / server_error (5xx) /
  external_error (dependency failure, with `retry_after`).
- **Liveness vs readiness probes** — separate endpoints. Readiness checks DB,
  cache, external API; liveness is cheap and always-on.
- **Metrics**: request-count Counter, duration Histogram, active-connections
  Gauge (Prometheus-style).
- **Concrete perf targets** (modelcontextprotocol.info): P95 < 100 ms (simple),
  P99 < 500 ms (complex), error-rate < 0.1%, availability > 99.9%. Track, do
  not blindly adopt — measure your workload.
- **Test layers**: unit / integration / **contract (MCP protocol compliance)**
  / load. Contract tests validate your server against the MCP spec; do not skip
  them.

---

## What to avoid

- **`mark3labs/mcp-go` as the default SDK** without a one-line rationale
  (contradicts the API-shape rules above).
- **`map[string]any` in tool arguments** — raises hallucination and
  mis-selection; use typed structs.
- **Panicking in handlers** — tool failures are `IsError: true` results, not
  panics.
- **Dumping raw large data** into tool responses — curate to protect the
  context window.
- **A generic `OnNotification`** — use feature-specific handlers.
- **Auth via transport hooks** — use standard HTTP middleware.
- **Designing a new server around roots, sampling, logging notifications, or
  resource subscriptions** — all deprecated or removed on protocol
  `2026-07-28`. They still work for older peers; they are not a foundation.
- **Per-session in-memory state on an HTTP server** — it blocks horizontal
  scaling and does not survive a deploy, and stateless mode (required for the
  new revision) rejects server-to-client requests outright.
- **Treating `RequestState` as trusted scratch space** — it round-trips through
  the peer; sign and encrypt it on an unauthenticated server.
- **Re-inventing `SetTools` / `AddSessionTool`** in wrappers — the SDK
  deliberately omits them.
- **Raising tool failures as protocol errors** — they belong in
  `CallToolResult.IsError`.

---

## Sources

- [go-sdk `design/design.md`](https://github.com/modelcontextprotocol/go-sdk/blob/main/design/design.md) —
  API-shape rules, typed handlers, transport/session split, cancellation,
  middleware, pagination, logging bridge. Natively Go; authoritative.
- [go-sdk v1.7.0 release notes](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0)
  and the `mcp` package source at that tag — protocol `2026-07-28`:
  statelessness, `server/discover`, multi-round-trip requests,
  `subscriptions/listen`, cacheable list results, and the roots/sampling/logging
  deprecation. Claims in this reference were verified against the tagged source,
  not the release notes alone.
- [Phil Schmid — MCP best practices](https://www.philschmid.de/mcp-best-practices) —
  tool design discipline (outcomes-not-operations, flatten args, 5–15 tools,
  naming, curate payloads). Language-agnostic; translated to Go.
- [modelcontextprotocol.info best-practices](https://modelcontextprotocol.info/docs/best-practices/) —
  production hardening (liveness/readiness, error classification, perf
  targets, test layers). Language-agnostic; translated to Go.
- [`SipherAGI/gaia-mcp-go` `.cursorrules`](https://github.com/SipherAGI/gaia-mcp-go) —
  generic Go idiom corroboration only; its `mark3labs/mcp-go` recommendation
  is discarded (see SDK choice above).
