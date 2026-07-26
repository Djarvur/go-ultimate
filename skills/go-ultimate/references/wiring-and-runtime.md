# Reference: wiring, Runtime, and the serve lifecycle

Read this when implementing or reviewing `wire.go` for a backend service or a CLI,
deciding what an application's wiring should return,
or wiring the process lifecycle in `main.go`.

This is **one reference implementation**, not a mandate. The mandatory rules are in SKILL.md
(`wire.go` is wiring-only and starts nothing; `main` owns process lifecycle).
The shapes below are illustrations; adapt the names and types to your own building blocks.

## The split: wire builds, a serve loop runs

Two responsibilities are kept apart:

- **`wire.go`** builds the dependency graph and returns _already-configured_ objects.
  It starts nothing.
- A small **serve loop** in `main` (or an integration test) runs the returned startable units
  and handles graceful shutdown.

This split is what lets the _same_ wiring serve a process, drive a CLI command,
and back an integration test without modification.

## Recommended `Wire` signature

```go
func Wire(ctx, startupCtx context.Context, cfg Config) (*Runtime, []Unit, error)
```

- `ctx` — the long-lived base/application context;
  it lives for the whole process, and the startable units close over it.
- `startupCtx` — a _secondary_ context bounding wiring work only
  (a startup deadline, plus cancellation if a sibling's wiring fails).
  Used for dialing DBs, etc.; never stored for request-time use.
- `cfg` — the application's _business_ configuration (not CLI/env grammar),
  validated inside `Wire` so every entrypoint surfaces the same field-attributed errors.
- returns the assembled `*Runtime` plus its startable units.

A `Unit` can be just a startable function (e.g. `func(context.Context) error` or `func()`)
which closes over `Wire`'s `ctx` and should be called in its own goroutine.
If it gets another context from a serve loop it should use `contextx.MergeCancel()`
to merge only cancellation from given context into closed over `Wire`'s `ctx`.
Returning descriptors — not started goroutines — is what keeps `wire.go` startless.

## The `Runtime` value

`Wire` returns a `Runtime`: the assembled application, designed to serve every consumer
(server `main`, CLI, integration tests) with minimal adaptation.

```go
type Runtime struct {
	App  port.App // The application boundary; CLI/tests callers use it.
	repo *Repo    // Owned resources kept private, released in Close().
	// health/readiness exposed via a method or an embedded provider
}

// Close releases every resource the wiring created.
// Tolerate partially-wired state (nil checks) so Wire can call Close() on its own error path.
func (r *Runtime) Close() error { /* join-close owned resources */ }
```

Conventions worth copying:

- Expose the application _boundary_
  (`port.App`, or an in-process entry adapter — see `modular-monolith.md`),
  never the concrete `internal/app` type.
- Keep owned resources (DB pools, clients) private and release them all in `Close()`.
- Expose health/readiness so a consumer (or an aggregate) can read it uniformly.
- On its own error path, `Wire` should `Close()` what it already built
  (a deferred error-path close).

## `main` owns process concerns only

`main` parses flags/env, sets up logging, maps grammar → business `Config`, calls `Wire`,
runs the serve loop, and handles signals + the shutdown deadline. It builds no graph itself.

```go
ctx, shutdown := context.WithCancel(ctx)
defer shutdown()
ctx = withSignals(ctx) // SIGINT/SIGTERM/…

startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
defer cancel()
rt, units, err := Wire(ctx, startupCtx, cfg)
if err != nil {
	return err
}
defer rt.Close()

return run(ctx, shutdown, units...) // run in parallel; first unit to return triggers shutdown
```

The serve loop runs units in parallel and, when any one returns, cancels the rest and waits
for them to stop — so the _first_ failure is the reported cause and shutdown is orderly.
A separate watchdog can force exit if graceful shutdown overruns its deadline.

## CLI commands reuse the same wiring

A CLI command wires the same way and uses the application boundary directly,
with no `in/cli` adapter and no servers:

```go
rt, _, err := Wire(ctx, ctx, cfg) // no long-running units needed
if err != nil {
	return err
}
defer rt.Close()
result, err := rt.App.SomeUseCase(ctx, args)
```

When the command (or any caller outside the app's own network adapters) needs request-scoped
inputs such as auth or request metadata, prefer entering through the in-process in-adapter
rather than `App` directly — see `modular-monolith.md`.
