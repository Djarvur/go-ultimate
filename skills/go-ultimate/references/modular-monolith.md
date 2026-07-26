# Reference: modular monolith composition and in-process calls

Read this when building or modifying a repository with multiple `apps/*`,
wiring those apps into one binary,
or designing how one embedded application calls another in-process.

This is **one reference implementation**, not a mandate.
The mandatory rules are in SKILL.md (each app is an application module;
cross-app data ownership holds; collaborate through a composition root).
The shapes below are platform-agnostic illustrations.

## The composition layer is an active accumulator, not a passive struct

A naive composition root is a struct of boundary fields.
In practice a richer **accumulator** earns its place:
it wires the included apps in parallel,
collects their startable units / health / closers,
injects cross-app dependencies once all apps exist,
and exposes a single aggregate `Close` and health.

It wires only the apps a binary _registers_,
so different binaries can embed different subsets of the available applications.

```go
// apps.go — owns no bounded context; generalises the App-with-Wire-Close shape to N apps.
type Apps struct {
	// aggregate health, set after Wire
	// startup context + a parallel-wiring group
	units   []Unit
	closers []func() error
	// one typed Runtime handle per included app, filled by Wire,
	// used to read in-process surfaces and to inject late dependencies
}
```

### Registration is per-app and explicit

Each app gets a typed `Add*` method that _schedules_ (does not run) its `Wire`,
keyed by the app's name.
Duplicate registration is a wiring bug → panic.
`main` registers exactly the apps this binary should embed.

The composition layer validates the dependency graph's _completeness_, not just duplicates:
when it injects late cross-app dependencies and finds a registered app whose required
sibling was never registered, that is also a wiring bug in `main` → panic.
Both panics catch mistakes in how a binary composes apps, before any request is served.

### `Wire` runs apps in parallel, then injects late dependencies

```go
func (a *Apps) Wire() ([]Unit, error) {
	// run each scheduled app's Wire concurrently;
	// on any error, Close the partially-wired apps and return.
	// Then, with every app's Runtime available, inject cross-app dependencies:
	a.exampleRT.Setup(a.authRT.InprocApp) // see two-phase init below
	// finally publish the aggregate health.
}
```

## Late cross-application dependency injection (two-phase init)

Apps are wired in parallel and may depend on each other — even cyclically (A↔B).
So a sibling's live surface does not exist when an app's `Wire` runs.
Resolve this with explicit two-phase init,
**not** Wire parameters or pointer-to-interface promises:

1. `Wire` builds the app and a placeholder for the sibling dependency.
2. After all apps are wired, the composition layer calls each app's `rtSomeApp.Setup(...)`
   to inject the now-available sibling surface.

The write in `Setup` happens before any serve goroutine starts,
so it publishes safely without further synchronization.

```go
type Runtime struct {
	InprocApp port.InprocApp              // this app's own in-process in-port, exposed to siblings
	sibling   struct{ sibport.InprocApp } // placeholder for a sibling, filled by Setup
}
func (r *Runtime) Setup(s sibport.InprocApp) { r.sibling.InprocApp = s }
```

## What the in-process inbound adapter does

A caller that reaches the app in-process (a sibling app or CLI) goes through the in-process
inbound adapter — the peer of the gRPC/HTTP adapters — not `port.App` directly.
Before delegating to `port.App` it does what a network inbound adapter does:

- run the app's middleware to prepare the execution environment business logic expects
  (context setup, instrumentation, logging, deadlines, admission/rate control that may even
  reject the call);
- propagate out-of-band request metadata (request-id, trace-id, deadline, auth tokens),
  including a hop budget that bounds in-process call depth the way a network TTL bounds
  forwarding, so it keeps flowing when the business logic calls further applications;
- scope the context as a transport would (carry the caller's cancellation/deadline,
  but start identity/observability from the callee's side).

### What can be generated

The adapter is mechanical: per `App` method it runs the fixed steps, then delegates.
So `InprocApp` (an interface mirroring `App` but dropping the request-scoped inputs the adapter
resolves from the out-of-band metadata) and its implementation can be
**code-generated from the `App` interface**.
The hand-written per-app glue is then just the hooks
(what middleware to run, what metadata to stamp),
which live in `wire.go` and may freely use the app's internal packages.
Where that generated implementation lives is an implementation detail —
only the `InprocApp` interface is part of the public contract.

## Shared packages between apps

`internal/dom` and `internal/xport` are the two shared **leaf** packages a `port` may import:

- **`internal/dom`** — shared pure business types (identity, money, etc.).
  No dependency on ports/adapters/infrastructure. The default home for shared types.
- **`internal/xport`** — the cross(-app) `port` tier:
  contracts a `port` must reference but no single app's `port` owns —
  cycle-forced DTOs (a smell marker — minimize) and cross-cutting DIP interfaces.
  A leaf: interfaces and DTO/value types only, no implementations.

## What imports what

- An app's **app-code** (adapters, `wire.go`) may import a sibling's `port` —
  its contract types, errors, and `InprocApp` interface (the published contract).
  Its own **`port`** may not: a `port` stays on the leaf tier
  and never imports another app's `port`.
- The _live_ sibling dependency is injected by the composition layer after wiring (`Setup`),
  not reached as a global. Inject the specific sibling `InprocApp`(s) an app needs
  (the full interface), not the whole accumulator with all apps.
