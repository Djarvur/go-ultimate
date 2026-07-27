# Go Engineering Policy

**Source:** [powerman `go-engineering-policy` v0.0.5](https://github.com/powerman/skills) +
additions (error taxonomy, dependency/CI policy, codegen). Wins over generic Go
style/lint guidance for non-architectural Go conventions.

## Contents

- [`go.mod` version policy](#gomod-version-policy)
- [`package main`](#package-main)
- [Variable scope policy](#variable-scope-policy)
- [Style and idioms](#style-and-idioms)
- [`context.Context` — do not alias by default](#contextcontext-do-not-alias-by-default)
- [Errors — taxonomy and rules](#errors-taxonomy-and-rules)
- [Concurrency](#concurrency)
- [Constructor pattern — Resolvable Config Struct (not Functional Options)](#constructor-pattern-resolvable-config-struct-not-functional-options)
- [Naming](#naming)
- [`internal/` discipline](#internal-discipline)
- [Dependencies and CI](#dependencies-and-ci)
- [Code generation](#code-generation)
- [Graceful shutdown](#graceful-shutdown)
- [Observability baseline (minimum)](#observability-baseline-minimum)
- [Architectural Decision Records (ADRs)](#architectural-decision-records-adrs)
- [Sources](#sources)

---

## `go.mod` version policy

| Project type | Go version in `go.mod` | Upgrade cadence |
|---|---|---|
| Application (CLI, service) | **Latest** stable Go | As soon as a new version is released. |
| Library / module | **Latest − 1** | Only when a needed feature requires it. |

Set `toolchain` only when you need to pin; otherwise let the toolchain directive
be implicit.

---

## `package main`

- **Single app** → `main.go` in repo root.
- **Multiple apps or helper apps** → `cmd/<app-name>/main.go`.
- `main()` manages **only**: flags, env, process metrics, signal handling,
  graceful shutdown, and consuming values prepared by `wire.go` (services) or
  `run` (CLIs).
- **`main()` must contain no code that should be tested.** All testable logic
  lives in `run(ctx, cfg) error` (CLI) or `wire.go` + `app` (service).

See [project-layouts.md](project-layouts.md) § CLI for the `run` pattern.

---

## Variable scope policy

**Use an `if` init statement ONLY when:**
- Converting an existing value to the form/type required by the condition.
- Intentionally shadowing a variable to avoid aliases or scope clutter.

Otherwise, prefer a separate statement + `if`.

```go
// GOOD — separate operation and condition.
err := validate(input)
if err != nil {
    // ...
}

// BAD — operation unrelated to the condition's form.
if err := validate(input); err != nil {
    // ...
}

// GOOD — needs another representation for the condition.
if v, ok := m[key]; ok {
    // use v
}

// BAD — scope cluttering (v not used after the if).
v, ok := m[key]
if ok {
    // use v
}
// no further use of v
```

---

## Style and idioms

These are the small-surface rules that keep a codebase consistent and
Go-idiomatic. Most are derivable from [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
and the [Google Go Style Guide](https://google.github.io/styleguide/go/).

### Error strings
- Lowercase, no trailing punctuation: `fmt.Errorf("something failed")`, not
  `fmt.Errorf("Something failed.")`. (The caller wraps with context; the full
  chain reads as a sentence.)
- **No in-band errors.** Return `(value, ok)` or `(value, error)`, never a
  sentinel like `-1` or `nil` to signal failure on a value-returning function.

### Control flow
- **Handle errors first, return early.** No `else` after an error return.
  ```go
  // GOOD
  if err != nil { return err }
  doWork()

  // BAD
  if err != nil { return err } else { doWork() }
  ```

### `context.Context`
- Pass as the **first parameter**: `func F(ctx context.Context, ...)`.
- **Do not store `context.Context` in a struct** (with the narrow exception of
  request-scoped objects that wrap a context for propagation). It is a
  parameter, not state.

### Receivers
- Short, consistent receiver names: `c` for `Client`, `s` for `Server`. One
  letter is normal if unambiguous. **Never** `this`, `self`, or `me`.

### Value vs pointer
- Pass small structs and `time.Time` **by value**. Use pointers only when
  mutating, when the struct is large, or when `nil` is a meaningful value.
- **Never copy a struct that contains a `sync.Mutex`** (or any `sync.*` primitive).
  Pass it by pointer. `go vet` catches this.

### Zero values
- **Make zero values useful** so types like `sync.Mutex`, `bytes.Buffer`, and
  your own structs work without explicit initialization where reasonable.

### Concurrency philosophy
- **Share memory by communicating; do not communicate by sharing memory.**
- **Use channels to orchestrate and mutexes to serialize** — pick the tool that
  matches the structure, not as a religious choice.

### Imports
- Group imports: (1) standard library, (2) third-party, (3) local/project —
  separated by blank lines. `goimports` enforces this.
- **No dot imports** except in `_test.go` to break circular deps, and only there.

### Linting discipline
- Use `//nolint:lintername` sparingly and **always include a comment justifying
  the exclusion**. A naked `//nolint` is a smell.
  ```go
  //nolint:gosec // Replay-attack risk is mitigated by token TTL
  func unsafeProcess() { ... }
  ```

---

## `context.Context` — do not alias by default

Keep the full type name `context.Context` for grep-ability and IDE navigation.

```go
// DEFAULT
func doSomething(ctx context.Context, args Args) error { ... }
```

If the project **already** uses a short alias consistently, match it — do not
mix. Do not introduce a `Ctx` alias in a project that does not already use one.

---

## Errors — taxonomy and rules

### Comparison
- **Never** compare errors with `==`. Use `errors.Is(err, target)` (handles wrapping).
- Use `errors.As(err, &target)` for typed errors (or `errors.AsType[T]` on Go 1.26+).

### Wrapping
- Wrap with `fmt.Errorf("doing X: %w", err)` at each layer boundary.
- Wrap once per layer; do not re-wrap an already-wrapped error in the same layer.

### Sentinel errors
- Define package-level sentinels with `errors.New` for expected error conditions
  callers must handle:
  ```go
  var ErrNotFound = errors.New("not found")
  ```
- Keep sentinels few and stable — they are public API.

### Typed errors
- Use a typed error when callers need to extract structured data:
  ```go
  type ValidationError struct {
      Field   string
      Message string
  }
  func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
  ```
- Check with `var ve *ValidationError; if errors.As(err, &ve) { ... }`.

### Aggregation (Go 1.20+)
- Use `errors.Join(err1, err2)` to combine multiple errors (e.g. multi-field
  validation).

### Idioms
- Return `nil` for "no error" — never a typed nil pointer wrapped in a non-nil
  error interface (the classic `if err != nil { return err }` returning a nil
  `*MyError`).
- Errors are values. Do not log and return the same error — log at the top of
  the stack or handle it, not both.

---

## Concurrency

- **Every goroutine must have a clear exit condition** — a `context.Context`
  cancellation, a `sync.WaitGroup`, or a done channel. No fire-and-forget.
- **Use `errgroup`** (`golang.org/x/sync/errgroup`) for parallel fan-out that
  should fail-fast on first error.
- **Always run tests with `-race`** in CI.
- Do not start goroutines from inside library code unless the API contract
  explicitly documents the lifecycle; prefer letting the caller spawn.

```go
g, ctx := errgroup.WithContext(ctx)
for _, item := range items {
    // No `item := item` — since Go 1.22 each iteration gets its own copy.
    g.Go(func() error { return process(ctx, item) })
}
if err := g.Wait(); err != nil { ... }
```

> On Go 1.25+, `sync.WaitGroup` gained `wg.Go(func(){ ... })` which spawns the
> goroutine and tracks it in one call — use it directly when you do not need
> errgroup's cancel-on-first-error behavior. Use `errgroup` when you do.

---

## Constructor pattern — Resolvable Config Struct (not Functional Options)

Prefer a `Config` struct with these rules over Functional Options:

1. **Zero value of an optional field ALWAYS means "unset, use default".**
   - Plain exported field when zero means "unset/use default".
   - When zero is a *valid explicit* value you must distinguish from "unset",
     use one of:
     - a **sentinel value** (e.g. `const NoTimeout time.Duration = -1`),
     - a **pointer field** (e.g. `*int` — `nil` = default, `new(0)` = explicit zero),
     - a **separate flag/enum** (e.g. `TLSMode{Default,Enabled,Disabled}`).
2. **Implement `Clone() Config`** if `Config` contains any references
   (pointer/map/slice/etc.).
3. **Implement idempotent `Resolve() (Config, error)`** that calls `Clone()`
   (if implemented), applies defaults, normalizes, and validates.

```go
type TLSMode uint8

const (
    TLSDefault TLSMode = iota
    TLSEnabled
    TLSDisabled
)

type Config struct {
    AppName string           // no default semantics
    Host    string           // "" = default
    Port    int              // 0  = default
    TLS     TLSMode          // TLSDefault = default
    Timeout time.Duration    // 0 = default; NoTimeout = disable
}

func (c Config) Clone() Config {
    // deep-copy any reference fields here
    return c
}

func (c Config) Resolve() (Config, error) {
    c = c.Clone()
    if c.Host == "" { c.Host = "localhost" }
    if c.Port == 0 { c.Port = 8080 }
    if c.TLS == TLSDefault { c.TLS = TLSEnabled }
    c.AppName = strings.TrimSpace(c.AppName)

    var err error
    if c.AppName == "" {
        err = errors.Join(err, ErrNoAppName)
    }
    return c, err
}

func New(cfg Config) (*Client, error) {
    cfg, err := cfg.Resolve()
    if err != nil {
        return nil, err
    }
    return &Client{cfg: cfg}, nil
}
```

**Functional Options are acceptable** only for genuinely streaming/fluent builder
APIs where the struct does not fit — rare. Default to the struct.

---

## Naming

- Short, high-signal package names: `dom`, `app`, `dal`, `port`, `srv`, `svc`, `sub`, `repo`.
- Do not repeat the directory noun in filenames — `order/app.go`, not `order/order.go`.
- Generic filenames where the package gives context: `adapter.go`, `app.go`, `port.go`, `wire.go`.
- Use `New` as the constructor name when the package already explains what is built.
- Exported identifiers have doc comments starting with the identifier name.

---

## `internal/` discipline

- Use `internal/` aggressively — anything that is not a published API contract
  goes there.
- For services: `internal/port`, `internal/app`, `internal/in`, `internal/dal`,
  `internal/out` (see [architecture.md](architecture.md)).
- For libraries: private implementation under `internal/`; public API at module root.
- Repo-internal helpers shared by multiple apps may live in repo-root `internal/`
  when they are not part of any single app's public boundary.

---

## Dependencies and CI

### Linting (mandatory in CI)
- **`go vet ./...`** — always.
- **`golangci-lint run`** — recommended config:
  ```yaml
  # .golangci.yml
  linters:
    enable:
      - errcheck
      - govet
      - ineffassign
      - revive
      - gofmt
      - goimports
      - unused
      - misspell
      - unconvert
      - staticcheck
      - gocritic
      - bodyclose
  ```
- `go test -race ./...` — race detector mandatory.
- `go mod tidy -diff` (or `go mod tidy && git diff --exit-code go.mod go.sum`) —
  fail CI if modules are not tidy.

### Dependency policy
- Pin to the latest minor of each direct dependency; let dependabot/renovate
  open PRs for patch and minor.
- Review major upgrades manually.
- Avoid `replace` directives except for local development or forced security
  patches; document each one.

### CI shape (minimum)
```text
on: push/pull_request
jobs:
  test:
    - go test -race -cover ./...
  lint:
    - go vet ./...
    - golangci-lint run
  tidy:
    - go mod tidy && git diff --exit-code go.mod go.sum
  build:
    - go build ./...
```

---

## Code generation

- Use `//go:generate` directives to drive generation; do not check in generated
  code without the directive that produced it.
- **Mocks** — `go.uber.org/mock/mockgen` (see [testing.md](testing.md)).
- **Wire (DI)** — google/wire for service wiring when the dependency graph grows
  large; otherwise hand-written `wire.go` is fine.
- **Protobuf / schema-driven code** — generated into a build-managed directory,
  checked in or generated in CI per project policy; never hand-edited.
- **Generated files carry a `Code generated ... DO NOT EDIT.` header.** Linters
  skip them via `//nolint` or config.

---

## Graceful shutdown

- Use `signal.NotifyContext` to derive a cancellable context from OS signals
  (`os.Interrupt`, `syscall.SIGTERM`).
- Propagate that context through `wire.go` / `run(ctx, cfg)`.
- On cancellation: stop accepting new work, finish in-flight, close resources in
  reverse-init order, apply a shutdown timeout (e.g. 30 s).
- The composition layer (`apps/apps.go` for monoliths, or `Runtime.Close()` for
  single apps) owns the single aggregate `Close`.

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
// ... wire / run ...
```

---

## Observability baseline (minimum)

- **Logging** — `log/slog` (Go 1.21+) with structured key-value pairs. Define
  log levels at startup from env. No `fmt.Println` in libraries.
- **Metrics** — one Prometheus-style registry; expose `/metrics` for services.
- **Tracing** — propagate `trace-id`/`request-id` through context; for inbound
  adapters, generate/extract it; for outbound, inject it. Defer full OTEL
  adoption to a project decision, but the context plumbing must exist from day one.

---

## Architectural Decision Records (ADRs)

Document major design choices so the team can track *why* a decision was made,
not just *what* was decided. ADRs age well; commit messages and code comments do
not.

### When to write an ADR

Write one when:
- selecting or changing a database, storage engine, or message broker;
- choosing a major framework or library that pervades the codebase (e.g. HTTP
  router, DI tool, ORM);
- defining a new API standard or wire format;
- introducing a dependency that significantly affects build size, complexity, or
  maintenance burden;
- making an architectural call that future contributors are likely to question
  or re-litigate (e.g. "no `pkg/` directory", "bounded-context hexagonal").

A rule this skill already mandates does *not* need an ADR — only deviations and
genuinely project-specific decisions do.

### Template

```markdown
# [Short Title, e.g. Use Postgres for Metadata]

**Status:** [Proposed | Accepted | Deprecated | Superseded]
**Date:** [YYYY-MM-DD]
**Author:** [Name]

## Context
[Describe the problem or opportunity. What options were considered?
Why is a decision needed now?]

## Decision
[Describe the decision clearly. "We will use X to do Y."]

## Consequences
**Positive:**
*   [Benefit 1]
*   [Benefit 2]

**Negative:**
*   [Drawback 1]
*   [Drawback 2]

**Risks:**
*   [Risk 1]
```

Store ADRs under `docs/adr/` (or `adr/`) numbered with a zero-padded prefix
(`0001-use-postgres.md`). When a decision is superseded, mark the old ADR
`Superseded by 0007-...` rather than deleting it — the history is the point.

---

## Sources

- [powerman `go-engineering-policy` v0.0.5](https://github.com/powerman/skills) — `go.mod`, `package main`, var-scope, Resolvable Config Struct, testing, `internal/` discipline.
- [danicat `go-best-practices`](https://github.com/danicat/skills) — concurrency rule, the Style & Idioms section (error strings, early return, in-band errors, ctx-not-in-struct, receivers, value-vs-pointer, zero values, import grouping, dot imports, `//nolint` justification), and the ADR template + criteria.
- Error taxonomy, dependency/CI policy, codegen, graceful shutdown, observability are ultimate-skill additions filling gaps in the source skills.
