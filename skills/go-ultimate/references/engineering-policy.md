# Go Engineering Policy

**Source:** [powerman `go-engineering-policy` v0.0.5](https://github.com/powerman/skills) +
additions (error taxonomy, dependency/CI policy, codegen). Wins over generic Go
style/lint guidance for non-architectural Go conventions.

## Contents

- [`go.mod` version policy](#gomod-version-policy)
- [`package main`](#package-main)
- [Variable scope policy](#variable-scope-policy)
- [Globals and `init()`](#globals-and-init)
- [Style and idioms](#style-and-idioms)
- [`context.Context` — do not alias by default](#contextcontext-do-not-alias-by-default)
- [Errors — taxonomy and rules](#errors-taxonomy-and-rules)
- [Outbound HTTP](#outbound-http)
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

## Globals and `init()`

### No mutable globals

A mutable package-level variable is a hidden input to every function that reads
it: it defeats parallel tests, creates ordering dependencies between packages,
and makes a function's behavior unreproducible from its arguments.

- **Pass dependencies explicitly** — constructor injection into a struct, not a
  package-level `var`. This is the same rule that makes `wire.go` possible.
- **Mutable global function pointers are the worst case** ("swap `timeNow` in the
  test"). Inject a clock or an interface instead; a test that mutates a global
  cannot be `t.Parallel()`.
- Package-level `var` is fine when it is **effectively immutable** after
  initialization: sentinel errors, compiled regexps, lookup tables. Nothing
  writes to them after `init`.

### `init()` is a last resort

- **Never start a goroutine in `init()`.** There is no way to stop it, no way to
  observe its failure, and it starts before `main` has decided whether the
  process should do anything. Background work belongs to an object with an
  explicit lifecycle, started from wiring.
- **`init()` must not do I/O**, read the environment, or mutate global/process
  state. Any of these makes importing the package a side effect, which breaks
  tests and makes failure unreportable — `init()` cannot return an error.
- **`init()` must not depend on ordering** between files or packages. Ordering
  is defined but not obvious, and the next refactor changes it silently.
- Legitimate uses are narrow: registering a driver or codec with a registry, and
  computing a derived constant that cannot be a composite literal. Everything
  else belongs in a constructor.

---

## Style and idioms

These are the small-surface rules that keep a codebase consistent and
Go-idiomatic. Most are derivable from [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
and the [Google Go Style Guide](https://google.github.io/styleguide/go/).

### Least mechanism

**Prefer the simplest construct that does the job.** In descending order of
preference: a core language construct (slice, map, channel, plain function) → the
standard library → a well-established third-party dependency → new machinery of
your own.

This is the principle behind several rules that otherwise look unrelated —
vanilla `testing` over an assertion library, a `Config` struct over functional
options, hand-written wiring before a DI framework, `log/slog` over a logging
abstraction layer. Sophisticated machinery has to earn its place against the
boring version; the boring version does not have to justify itself.

### Declarations

- **`:=` for initialization with a real value**; `var` for zero-value
  initialization.

  ```go
  var buf bytes.Buffer     // zero value is useful
  var users []User         // nil slice, ready to append
  count := len(items)      // real value
  ```

- **Nil slices, not empty slices.** `var s []T` over `s := []T{}`. A nil slice
  appends, ranges, and reports `len` identically; the difference only shows up in
  JSON encoding, and an API whose behavior depends on nil-vs-empty is a bug
  waiting to be reported. Test emptiness with `len(s) == 0`, never `s == nil`.
- **Composite literals name their fields**: `User{Name: n, Email: e}`, not
  `User{n, e}`. Positional literals break silently when a field is inserted
  upstream. (The one tolerable exception is a test table with two or three
  obvious columns.)
- **Do not shadow predeclared identifiers** — no variable named `error`, `string`,
  `len`, `new`, `max`, `min`, `copy`. The code still compiles, which is what makes
  it dangerous.

### Type assertions

**Always use the comma-ok form.** A bare `v := x.(T)` panics on a type it did not
expect, and the input that triggers it is usually external.

```go
// GOOD
v, ok := x.(T)
if !ok {
    return fmt.Errorf("expected %T, got %T", T{}, x)
}

// BAD — panics on unexpected input.
v := x.(T)
```

The exception is a type switch, where the failure case is already an explicit
branch.

### Slices and maps at API boundaries

Slices and maps are references. Storing or returning one without copying hands
the other side a live handle to your internal state.

- **Copy on the way in** when you retain the argument beyond the call.
- **Copy on the way out** when returning something a caller must not mutate.

```go
func NewPolicy(rules []Rule) *Policy {
    p := &Policy{rules: slices.Clone(rules)} // caller may reuse its slice
    return p
}

func (p *Policy) Rules() []Rule {
    return slices.Clone(p.rules) // caller cannot corrupt p
}
```

If copying is too expensive to be reasonable, document the aliasing explicitly —
an undocumented shared slice is a data race that only shows up under load.

### Struct tags and format strings

- **Every struct that is marshaled carries field tags** (`json`, `yaml`, `db`).
  Without them the wire format is derived from Go identifier names, so a rename
  silently changes the payload.
- **Declare format strings as constants** when reused, and end custom
  `Printf`-style functions with `f` (`logf`, `wrapf`) — that suffix is what lets
  `go vet` check the arguments. A `Printf`-style function not ending in `f` is
  invisible to the vet analyzer.

### Signal boosting

When code deliberately departs from the idiom a reader expects, **say so in a
comment**. The canonical case is checking `err == nil` to take the happy path:

```go
// Only proceed when the lookup succeeded; a miss is expected here and not an error.
if err == nil {
    return cached, nil
}
```

Without the comment, the next reader assumes it is a typo for `err != nil` and
"fixes" it.

### Error strings
- Lowercase, no trailing punctuation: `fmt.Errorf("something failed")`, not
  `fmt.Errorf("Something failed.")`. (The caller wraps with context; the full
  chain reads as a sentence.)
- **No in-band errors.** Return `(value, ok)` or `(value, error)`, never a
  sentinel like `-1` or `nil` to signal failure on a value-returning function.
- **No `failed to` / `error while` prefixes.** Every layer adds one and the
  result reads `failed to save user: failed to exec query: failed to dial`. The
  value is already an error; say what was being attempted:
  `fmt.Errorf("save user %d: %w", id, err)`.

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

### Generics — when not to
The skill uses generics deliberately at typed boundaries (see
[mcp-server.md](mcp-server.md), [agents.md](agents.md)), which makes restraint
elsewhere worth stating:
- **Write the concrete version first.** Generalize on the second real caller, not
  the imagined one.
- **A one-method interface usually beats a type parameter** when the callee only
  needs behavior. Reach for generics when the *type relationship* between
  parameters and results is the point (`func Map[T, U any]([]T, func(T) U) []U`),
  not to avoid `any`.
- **Do not parameterize over types that only ever get one instantiation** — that
  is a struct with extra ceremony.

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

### Signature rules
- **`error` is the last return value**, always.
- **Exported functions return the `error` interface**, never a concrete error
  type. Returning `*MyError` makes every caller's `err != nil` check wrong for
  the typed-nil reason below, and it pins the concrete type into your public API.
- **`os.Exit` and `log.Fatal*` only in `main()`**, and preferably once. They skip
  every `defer`, so anywhere else they abandon open files, unflushed buffers and
  in-flight work. Library and business code returns errors.
- **`Must*` names a helper that panics on failure** (`template.Must`,
  `regexp.MustCompile`). The prefix is the whole warning label — use it only for
  package-level initialization of values that are compile-time constant in
  practice, never for anything that depends on runtime input.

### Wrapping
- Wrap with `fmt.Errorf("doing X: %w", err)` at each layer boundary.
- Wrap once per layer; do not re-wrap an already-wrapped error in the same layer.

### `%w` vs `%v` — wrapping is an API decision

`%w` makes the wrapped error **part of your package's contract**: callers can
match it with `errors.Is`/`errors.As`, so it can no longer be swapped out without
a breaking change. `%v` flattens it to text and keeps it an implementation
detail.

| Use | When |
|---|---|
| `%w` | Callers legitimately need to react to the cause — a sentinel they must handle (`ErrNotFound`), or a typed error they extract fields from. |
| `%v` | The cause is an implementation detail: a third-party driver's error, anything at a system boundary where the underlying library is replaceable. |

```go
// %w — ErrNotFound is documented API; the caller is expected to branch on it.
return fmt.Errorf("load user %d: %w", id, ErrNotFound)

// %v — the caller must not couple to pgx's error types.
return fmt.Errorf("load user %d: %v", id, pgErr)
```

Default to `%w` **within** a bounded context, where you own both sides. Prefer
`%v` when re-exporting a dependency's failure across your public boundary, unless
you are deliberately promoting it to contract. Place `%w` last in the format
string so the chain reads as nested context.

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

### `panic` and `recover`

- **Panic only on programmer error** — a violated precondition no caller input
  should be able to reach (a nil dependency that wiring must supply, an
  impossible switch branch). Expected failures are errors, always.
- **A panic must never cross a public API boundary.** If a package can panic
  internally, recover at its edge and return an error.
- **A long-running server needs a recover middleware** in its inbound adapter:
  one panicking handler must not take the process down. Log the panic with its
  stack, return 500, keep serving. `net/http` does recover per connection, but it
  kills the response silently and logs nothing structured — do it explicitly.
  See [`assets/webservice/internal/in/http/`](../assets/webservice/internal/in/http/).
- `recover()` is only meaningful directly inside a deferred function. It does not
  reach across goroutines: a panic in a goroutine you spawned takes down the
  process no matter what the spawner deferred.

---

## Outbound HTTP

- **Never use `http.DefaultClient`, `http.Get` or `http.Post` in production
  code** — they have no timeout, so one hung upstream blocks the caller forever.
- Construct an explicit client, and bound each call with a context as well. The
  client timeout is the backstop; the context is the per-request deadline that
  also propagates cancellation.

```go
var client = &http.Client{
    Timeout: 10 * time.Second, // whole exchange: dial + headers + body
    Transport: &http.Transport{
        MaxIdleConnsPerHost: 32,       // default 2 — too low for a hot upstream
        IdleConnTimeout:     90 * time.Second,
    },
}

ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
```

- **One shared client per upstream**, stored on the adapter. Building a client
  per request defeats connection pooling and leaks file descriptors.
- **Always close and drain the body**: `defer resp.Body.Close()`. A body left
  unread to EOF cannot return its connection to the pool (`bodyclose` is already
  in the mandated linter set).
- Server-side, set `ReadHeaderTimeout` at minimum — a missing one is a trivial
  slowloris.

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

- **Never start a goroutine in `init()`** — see
  [§ Globals and `init()`](#globals-and-init).

### Bounded concurrency

**Fan-out over a slice of unknown length is unbounded concurrency.** One request
with 100k items becomes 100k goroutines, 100k open connections, and an
out-of-memory kill.

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(runtime.GOMAXPROCS(0) * 4) // cap in flight; tune to the bottleneck
for _, item := range items {
    g.Go(func() error { return process(ctx, item) })
}
```

Use `errgroup.SetLimit`, a worker pool, or `golang.org/x/sync/semaphore`. The
limit belongs to whatever is actually scarce — upstream connections, database
pool size, CPU — not to a round number.

### Channel sizing

**Channels are unbuffered, or buffered with size 1.** Any other size needs a
written justification.

An unbuffered channel makes backpressure explicit: the sender blocks when the
receiver is behind, which is information. A large buffer hides that signal, turns
a queue-depth problem into a latency problem you cannot see, and loses everything
in flight when the process dies. Size 1 is the useful special case: a
non-blocking handoff, or a signal channel a sender must never block on.

If a buffer size is a tuning parameter, it is a queue — make it one explicitly,
with a depth metric ([production-readiness.md](production-readiness.md)).

### Mutexes and atomics

- **The zero value of `sync.Mutex` is ready to use.** Declare it as a value field
  (`mu sync.Mutex`), never a pointer, and never call `new(sync.Mutex)`.
- **Do not embed a mutex** — not even unexported. Embedding promotes `Lock`/
  `Unlock` into the struct's method set; if the struct is exported, callers can
  lock your internal state, and if it is marshaled, the mutex becomes part of the
  wire shape. Name the field.
- Declare the mutex immediately above the fields it guards, and say what it
  guards in a comment. A mutex whose scope is not written down gets it wrong on
  the next edit.
- **Atomics for a single independent value** (a counter, a flag) — `sync/atomic`'s
  typed values (`atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]`) rather than
  the loose functions. **A mutex for an invariant across several fields**: two
  atomics updated in sequence are two separate observable states, which is
  usually the bug.

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

### Form

- **`MixedCaps` / `mixedCaps`. No underscores.** Not in package names, type
  names, function names, or variables. The exceptions are `_test.go` file-level
  test function grouping, generated code, and low-level interop.
- **Initialisms keep a consistent case**: `HTTPClient`, `userID`, `parseJSON`,
  `ServeHTTP`, `xmlReader`. Never `HttpClient`, `userId`, `parseJson`. When an
  initialism starts an unexported name it is all-lowercase: `urlPath`, not
  `uRLPath`.
- **Constants are `MixedCaps` too** — `MaxRetries`, not `MAX_RETRIES` or
  `kMaxRetries`. Go has no separate constant naming convention.

### Length and content

- **Name length is proportional to scope.** `i`, `r`, `err` inside a five-line
  loop; a descriptive name for a package-level value. A long name in a tiny scope
  is noise; a short name in a large scope is a puzzle.
- **Omit the type from the name** — `users`, not `userSlice`; `timeout`, not
  `timeoutDuration`. The type is already in the signature.
- **Do not repeat the package name in its identifiers.** From outside it reads
  `user.User`, `http.HTTPServer`. Name it for how it will be called:
  `user.Account`, `http.Server`.
- **No `Get` prefix on accessors** — `u.Name()`, not `u.GetName()`. Reserve `Get`
  for something that genuinely fetches (`GetUser(ctx, id)` hitting a store).
  Verb-like names for actions, noun-like names for values.

### Packages

- Short, high-signal package names: `dom`, `app`, `dal`, `port`, `srv`, `svc`, `sub`, `repo`.
- **Never `util`, `common`, `helpers`, `shared`, `base`, `misc`.** A package named
  for its lack of a theme accumulates unrelated code forever and becomes an
  import-cycle magnet. If a helper has no obvious home, that is a signal about the
  design, not a reason to create `util`.
- Lowercase, single word, no underscores, singular.
- Do not repeat the directory noun in filenames — `order/app.go`, not `order/order.go`.
- Generic filenames where the package gives context: `adapter.go`, `app.go`, `port.go`, `wire.go`.

### Receivers and constructors

- Use `New` as the constructor name when the package already explains what is built.
- Receiver names are short and **identical across every method of a type** — if
  one method uses `c`, none uses `client`.
- **Receiver types are consistent too**: if any method needs a pointer receiver,
  all of them take pointer receivers.
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

> The concrete shape — required repository files, ignore baselines, workflow
> YAML, tool pinning, release stamping — is in
> [repo-and-ci.md](repo-and-ci.md). This section is the policy it enforces.

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
- **`govulncheck ./...`** — Go's official vulnerability scanner, mandatory. It is
  low-noise because it reports only vulnerabilities actually reachable from your
  call graph, not every advisory touching your module graph.

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
  vuln:
    - govulncheck ./...
  tidy:
    - go mod tidy && git diff --exit-code go.mod go.sum
  build:
    - go build ./...
```

---

## Code generation

- Use `//go:generate` directives to drive generation; do not check in generated
  code without the directive that produced it.
- **Pin every generator with the `tool` directive** (Go 1.24+), never the old
  `tools.go` blank-import hack. Without a pin each developer runs a different
  `mockgen` and the generated code churns from machine to machine — which matters
  here, because this skill mandates generated mocks for every In/Out port.

  ```console
  $ go get -tool go.uber.org/mock/mockgen@latest   # records it in go.mod
  $ go tool mockgen -version                       # runs the pinned version
  ```

  ```go
  //go:generate go tool mockgen -destination=mocks/port_mock.go -package=mocks example.com/proj/internal/port App,Repo
  ```

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
- **Never log a secret.** Give credential-bearing types a redacting
  `LogValue()`/`String()` so an accidental `slog.Any("cfg", cfg)` cannot leak
  them.

> This is the floor for any Go program. What a **service** additionally needs —
> which metrics, label cardinality, health vs readiness, SLOs — is in
> [production-readiness.md](production-readiness.md).

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
- [Google Go Style Guide / Decisions / Best Practices](https://google.github.io/styleguide/go/) (CC-BY-3.0) — Least Mechanism, naming form (MixedCaps, initialisms, constants), name length vs scope, `Get`-prefix and package-name-repetition rules, `util`/`common` prohibition, declaration style (`:=` vs `var`), nil-slice semantics, `%w` vs `%v` as an API decision, `error`-last and error-interface returns, `Must*` convention, signal boosting.
- [Uber Go Style Guide](https://github.com/uber-go/guide) (Apache-2.0) — comma-ok type assertions, channel sizing, `init()` restrictions, no mutable globals, copy slices/maps at boundaries, mutex value/no-embed rules, predeclared-identifier shadowing, struct tags, `Printf`-style `f` suffix and const format strings, `os.Exit`-only-in-main, no `failed to` prefixes.
- [metalagman `go-senior-developer`](https://github.com/metalagman/agent-skills) — bounded concurrency, atomics-vs-mutex split, secret redaction (rules extracted, prose rewritten).
- Error taxonomy, dependency/CI policy, codegen, graceful shutdown, observability are ultimate-skill additions filling gaps in the source skills.
