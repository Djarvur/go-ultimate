# Modern Go — Version-Gated Syntax

**Source:** condensed from [JetBrains `use-modern-go`](https://github.com/JetBrains/go-modern-guidelines/blob/3a7f3eecd29f/claude/modern-go-guidelines/skills/use-modern-go/SKILL.md)
(permalink to the revision this was condensed from — upstream has since replaced
the markdown catalog with a CLI; see [§ Source](#source)).

## Detecting the target version

Run the bundled detector, passing the target project's path as an argument:

```
go run <skill-dir>/scripts/goversion/main.go <project-path>
```

where `<skill-dir>` is the absolute path to the installed `go-ultimate` skill
(e.g. `~/.zcode/skills/go-ultimate`). The detector prints the bare `go`
directive value (e.g. `1.24.3`, **no** `go ` prefix) and exits 0. When no
`go.mod` is found or it has no `go` directive, it falls back to the Go runtime
version (the version the detector binary was compiled with) instead of failing —
so there is always a version on stdout.

Use ALL features up to and including the detected `go` version. Never use
features from a newer Go than the target. Never use an outdated pattern when a
modern alternative exists in the target version.

State the version once: *"This project is using Go X.XX, so I'll use features up
to and including this version."* Then proceed — do not list features or ask for
confirmation on each one.

> Pass the **file** path, not the directory. `go run <skill-dir>/scripts/goversion/main.go`
> works from inside any module; `go run <skill-dir>/scripts/goversion` (the package
> form) fails with "directory … outside main module or its selected dependencies".
>
> `go run` still compiles in the *target* module's build context, so a broken
> `go.mod`, an unavailable `toolchain` directive, or `-mod=vendor` can make the
> detector fail to start. The detector is a stdlib-only single file, so in that
> case copy the `main.go` into a scratch directory and run it there, or read the
> `go` directive out of `go.mod` directly.

## Contents

- [Detecting the target version](#detecting-the-target-version)
- [Catalog by version (use the highest applicable)](#catalog-by-version-use-the-highest-applicable)
- [Source](#source)

---

## Catalog by version (use the highest applicable)

### Go 1.13+
- `errors.Is(err, target)` instead of `err == target` (works with wrapped errors).

### Go 1.18+
- `any` instead of `interface{}`.
- `strings.Cut` / `bytes.Cut` instead of `Index` + slice.

### Go 1.19+
- `atomic.Bool` / `atomic.Int64` / `atomic.Pointer[T]` instead of `atomic.StoreInt32` etc.
- `fmt.Appendf(buf, ...)` instead of `[]byte(fmt.Sprintf(...))`.

```go
// Type-safe atomics replace error-prone atomic.Store*/Load* APIs.
var flag atomic.Bool
flag.Store(true)
if flag.Load() { /* ... */ }

var ptr atomic.Pointer[Config]
ptr.Store(cfg)
```

### Go 1.20+
- `strings.Clone` / `bytes.Clone` to copy without sharing memory.
- `strings.CutPrefix` / `strings.CutSuffix`.
- `errors.Join(err1, err2)` to combine multiple errors.
- `context.WithCancelCause` / `context.Cause` to get the error that caused cancellation.

### Go 1.21+
**Built-ins:** `min` / `max` / `clear`.
**`slices` package:** `Contains`, `Index`, `IndexFunc`, `Sort`, `SortFunc`, `Min`, `Max`, `Reverse`, `Compact`, `Clip`, `Clone`.
**`maps` package:** `Clone`, `Copy`, `DeleteFunc`.
**`sync`:** `sync.OnceFunc`, `sync.OnceValue` instead of `sync.Once` + wrapper.
**`context`:** `AfterFunc`, `WithoutCancel`, `WithTimeoutCause`, `WithDeadlineCause`.

```go
// WithoutCancel keeps values (trace-id, auth) but drops cancellation — for work
// that must outlive the request that started it. Always give it its own timeout.
bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
defer cancel()
go emitAudit(bg, event)
```

### Go 1.22+
- `for i := range n` and `for i := range len(items)` instead of `for i := 0; i < len(items); i++`.
- Loop variables are safe to capture in goroutines (each iteration gets its own copy).
- `cmp.Or(flag, env, config, "default")` — first non-zero value.
- `reflect.TypeFor[T]()`.
- Enhanced `http.ServeMux` patterns: `mux.HandleFunc("GET /api/{id}", h)` + `r.PathValue("id")`.
- `math/rand/v2` instead of `math/rand`: better generators, no global seeding ritual,
  `rand.N[T]` for any integer type. (Neither is a CSPRNG — `crypto/rand` for anything
  security-bearing.)

```go
// cmp.Or replaces the classic if-else chain for "first non-zero value".
name := cmp.Or(os.Getenv("NAME"), "default")
```

### Go 1.23+
- `maps.Keys(m)` / `maps.Values(m)` return iterators.
- `slices.Collect(iter)` / `slices.Sorted(iter)` to materialize iterators.
- `time.Tick` is safe to use freely (GC now reclaims unreferenced tickers;
  `Stop` is no longer needed to help the GC, so there is no longer any reason
  to prefer `NewTicker` when `Tick` will do).
- `unique.Make[T]` to intern comparable values that repeat heavily (labels, tags,
  interned strings): one canonical copy, and `unique.Handle` compares by pointer.

```go
// Three idioms for working with map iterators.
keys := slices.Collect(maps.Keys(m))       // not: for k := range m { keys = append(keys, k) }
sortedKeys := slices.Sorted(maps.Keys(m))  // collect + sort in one step
for k := range maps.Keys(m) { process(k) } // iterate directly, no allocation
```

### Go 1.24+
**ALWAYS** apply these:
- `t.Context()` instead of `context.WithCancel(context.Background())` in tests.
- `omitzero` (not `omitempty`) for `time.Duration`, `time.Time`, structs, slices, maps in JSON tags.
- `b.Loop()` (not `for i := 0; i < b.N; i++`) in benchmarks.
- `strings.SplitSeq` / `strings.FieldsSeq` / `bytes.SplitSeq` / `bytes.FieldsSeq` when iterating split results in a for-range.
- `os.Root` (`os.OpenRoot(dir)`) whenever a path comes from outside the program:
  every operation is confined to that subtree, so `../` escapes stop being your
  problem. Prefer it to hand-rolled `filepath.Clean` checks.
- `runtime.AddCleanup` instead of `runtime.SetFinalizer` — attachable more than
  once, does not resurrect the object, works with cycles.
- Generic type aliases are fully supported: `type Set[T comparable] = map[T]struct{}`.
- The `tool` directive in `go.mod` instead of the `tools.go` blank-import hack —
  see [engineering-policy.md § Code generation](engineering-policy.md).

```go
// 1.24+ benchmark
func BenchmarkFoo(b *testing.B) {
    for b.Loop() {
        doWork()
    }
}

// 1.24+ JSON
type Config struct {
    Timeout time.Duration `json:"timeout,omitzero"`
}
```

### Go 1.25+
**ALWAYS** use `wg.Go(fn)` instead of `wg.Add(1)` + `go func(){ defer wg.Done(); ... }()`.

```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Go(func() {
        process(item)
    })
}
wg.Wait()
```

- `testing/synctest` for testing concurrent code: inside a bubble the `time`
  package runs on a fake clock, so timeouts, tickers and retry/backoff become
  deterministic instead of `time.Sleep`-flaky. See
  [testing.md § Testing concurrent code](testing.md).

### Go 1.26+
- `new(val)` returns a pointer to any value: `new(30)` → `*int`, `new(true)` → `*bool`, `new(T{})` → `*T`.
  Do **not** use the `x := val; &x` pattern. Do **not** write redundant casts like `new(int(0))`.
- `errors.AsType[T](err)` instead of `errors.As(err, &target)`.

```go
// 1.26+
cfg := Config{
    Timeout: new(30),
    Debug:   new(true),
}
if pathErr, ok := errors.AsType[*os.PathError](err); ok {
    handle(pathErr)
}
```

---

## Source

This reference condenses the JetBrains `use-modern-go` SKILL.md as it stood at
commit [`3a7f3eecd29f`](https://github.com/JetBrains/go-modern-guidelines/blob/3a7f3eecd29f/claude/modern-go-guidelines/skills/use-modern-go/SKILL.md),
which carried the full per-version catalog with before/after examples inline.

**Upstream restructured on 2026-08-11**: the skill moved to
[`plugin/skills/use-modern-go/SKILL.md`](https://github.com/JetBrains/go-modern-guidelines/blob/main/plugin/skills/use-modern-go/SKILL.md)
and was rewritten from a static markdown catalog into a thin wrapper that shells
out to a Modern Go Guidelines CLI (`list` / `explain` subcommands) which resolves
guidelines at runtime. **The per-version catalog no longer exists upstream as
readable markdown.**

Consequences for this skill:

- **The catalog below is now owned and maintained here**, the same way the
  danicat material is (see SKILL.md `source-notes`). New Go releases must be
  folded in by hand rather than diffed against upstream.
- go-ultimate deliberately does **not** adopt the CLI approach: it would add a
  runtime dependency on a third-party binary to a skill that is otherwise
  self-contained, and version detection is already covered by the bundled
  [`scripts/goversion/main.go`](../scripts/goversion/main.go). Revisit only if
  the catalog becomes too costly to maintain by hand.
- The upstream pin in
  [`scripts/source-versions.json`](../scripts/source-versions.json) tracks the
  new path, so a future change to the CLI-based skill is still detected.
