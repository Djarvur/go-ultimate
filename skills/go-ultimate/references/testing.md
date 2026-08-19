# Testing

**Source:** powerman `go-engineering-policy` testing section, **de-`powerman/check`-ified**
(vanilla `testing` for portability). Mocking policy from powerman.

---

## Defaults

- **Vanilla `testing`** with `t.Errorf` / `t.Fatalf`. No assertion library by
  default.
- **Exception:** if the project already depends on `testify`, use testify's
  `assert`/`require` for consistency. Do not introduce testify into a project
  that does not already use it.
- **External test package** — `package xxx_test`, in a `xxx_test.go` file beside
  the code under test. See [§ Test placement](#test-placement) for the two
  exceptions.
- **Tests must only test the project's own code**, not stdlib or third-party
  libraries. Mock external dependencies (e.g. `os`, filesystem, network).

## Test placement

Go allows two packages in one directory — `foo` and `foo_test` — which gives
three possible homes for a test. Rank them in this order:

**1. External test package (default).** `package foo_test` in `foo_test.go`. It
sees exactly what a real consumer sees, so it cannot accidentally couple to
implementation details, and it doubles as a usage example.

**2. `export_test.go`** when the external test needs an unexported identifier. A
`_test.go` file in `package foo` that re-exports internals **to the test binary
only** — it never reaches the production binary:

```go
// export_test.go — package foo
var ParseDirective = parseDirective // visible to foo_test, absent from the build
```

**3. Internal test** — `package foo` in `foo_internal_test.go` — only when the
behavior cannot be reached through the public surface at all: a non-trivial
private algorithm, parser invariants, a state machine with no observable
intermediate states.

`_internal_test.go` is a **naming convention for the reader, not a Go rule**. The
compiler cares only about the package clause; the filename changes nothing.

> **Never export an identifier from a production file for tests** — no
> `ParseForTest`, no `ExportedForTesting`. Those become public API and ship in
> the binary. That is what `export_test.go` exists to prevent.

Needing an internal test is a signal about the public surface, not permission for
one. If behavior cannot be tested from outside, first ask whether it should be
reachable from outside.

## Naming

- Use the same naming convention as examples:
  - `Test` — generic test.
  - `TestF` — tests function `F`.
  - `TestT` — tests type `T`.
  - `TestT_M` — tests method `T.M`.
  - Optional suffix `_suffixCamelCase` for variants: `TestF_authorizedUser`,
    `TestF_unknownUser`.
- Place test functions in the **same order** as the tested code in the file.

## Idioms

- Begin most tests with `t.Parallel()` (after the early `t.Helper()`/setup if any).
- **`t.Setenv` and `t.Chdir` cannot be used in a parallel test** — both mutate
  process-wide state and panic if the test, or any ancestor, called
  `t.Parallel()`. This is the one legitimate exception to parallel-by-default.
  Better still: inject configuration instead of reading the environment inside
  the code under test, and the test stays parallel.
- Use **table-driven tests** for multi-case logic.
- Use test helpers (`t.Helper()` + a helper func) to reduce duplication within
  and between tests.
- **Release resources in helpers with `t.Cleanup`, never `defer`.** A `defer`
  inside a helper fires when the *helper* returns, not when the test ends — the
  resource is gone before the test uses it, and the code looks correct.
  Cleanups run LIFO after the test and its subtests finish.
- `t.TempDir()` for scratch directories: created per test, removed automatically,
  unique per parallel subtest.
- Verify **behavior**, not implementation details.

```go
package app_test

import (
    "testing"
    "errors"
    "example.com/proj/internal/app"
)

func TestDoThing(t *testing.T) {
    t.Parallel()

    tests := []struct{
        name    string
        input   string
        wantErr error
    }{
        {name: "valid",   input: "ok"},
        {name: "empty",   input: "",    wantErr: app.ErrEmpty},
        {name: "too big", input: "!!!", wantErr: app.ErrTooBig},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            err := app.DoThing(tt.input)
            if tt.wantErr != nil {
                if !errors.Is(err, tt.wantErr) {
                    t.Errorf("DoThing(%q) err = %v, want %v", tt.input, err, tt.wantErr)
                }
                return
            }
            if err != nil {
                t.Errorf("DoThing(%q) unexpected err: %v", tt.input, err)
            }
        })
    }
}
```

## Runnable examples

`Example` functions in the external test package are **tests and documentation at
once**: `go test` runs them and compares stdout to the `// Output:` comment, and
`go doc`/pkg.go.dev renders them on the package page. Documentation that cannot
silently go stale.

```go
func ExampleClient_Get() {
    c := api.NewClient(api.Config{})
    u, _ := c.Get(context.Background(), 1)
    fmt.Println(u.Name)
    // Output: Ada
}
```

- Naming binds the example to its subject: `Example`, `ExampleT`, `ExampleF`,
  `ExampleT_M`, plus an optional lowercase suffix (`ExampleClient_Get_notFound`).
- **An example without an `// Output:` comment is compiled but never run** — it
  checks nothing. Use `// Unordered output:` when map iteration makes ordering
  unstable.
- Every exported package gets at least one example showing the primary use case.
  For libraries this is mandatory — see [libraries.md](libraries.md).

## Contract tests

When an API is defined by a spec (OpenAPI, AsyncAPI, protobuf), **the spec is the
test fixture**. Validate real request/response pairs against the schema rather
than asserting on a hand-written copy of what you believe the spec says.

- Generate the server stubs and clients from the spec — a generated client that
  compiles is already a compatibility check
  ([engineering-policy.md § Code generation](engineering-policy.md)).
- Run consumer/provider checks where a real consumer exists, so a
  backward-incompatible change fails in your CI rather than theirs.
- A hand-maintained mirror of the schema inside the tests drifts and then asserts
  the wrong contract confidently. Point at the spec file itself.

## Benchmarks

`go test -bench=. -benchmem` is the only acceptable evidence for a performance
claim. Benchmark before and after, and put the diff in the PR description.
Profiling tooling and when to reach for it are in
[production-readiness.md § Performance work](production-readiness.md).

## Testing concurrent code

**Use `testing/synctest` (Go 1.25+) for anything involving time or goroutine
coordination.** Inside a bubble the `time` package runs on a fake clock that
advances only when every goroutine in the bubble is blocked, so timeouts,
tickers, retry/backoff and graceful shutdown become deterministic assertions
instead of `time.Sleep` guesswork.

```go
func TestClient_retryBackoff(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        // time.Sleep / context deadlines inside here are instant and exact.
        err := client.Do(t.Context(), req)
        // ...
    })
}
```

`time.Sleep` in a test is a smell: on Go 1.25+ reach for `synctest`; below it,
inject a clock. Never "fix" a flaky concurrency test by raising a sleep.

**Assert that goroutines actually exit.** Non-negotiable principle 6 says every
goroutine has an exit condition; make it enforceable rather than aspirational.
`go.uber.org/goleak` in `TestMain` is the usual way:

```go
func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }
```

`synctest` gives this for free inside a bubble: it fails the test if a goroutine
is still running when the bubble ends.

Run the race detector locally, not only in CI — `go test -race ./...`.

## Fixtures, `testdata/` and golden files

- Test inputs live in **`testdata/`**, which the Go toolchain ignores when
  building and when matching packages.
- For output that is large or awkward to write inline (rendered templates,
  generated code, formatted reports), compare against a **golden file** in
  `testdata/` and regenerate it behind a flag:

  ```go
  var update = flag.Bool("update", false, "rewrite golden files")

  func checkGolden(t *testing.T, name string, got []byte) {
      t.Helper()
      path := filepath.Join("testdata", name)
      if *update {
          if err := os.WriteFile(path, got, 0o644); err != nil {
              t.Fatalf("write golden: %v", err)
          }
          return
      }
      want, err := os.ReadFile(path)
      if err != nil {
          t.Fatalf("read golden: %v", err)
      }
      if !bytes.Equal(got, want) {
          t.Errorf("output differs from %s (re-run with -update to accept)", path)
      }
  }
  ```

- Review golden-file diffs like source. A golden test whose file is regenerated
  without reading the diff asserts nothing.

## HTTP adapters

Test inbound HTTP adapters with **`net/http/httptest`** and a mock `port.App` —
no live listener, no port binding:

- `httptest.NewRequest` + `httptest.NewRecorder` to drive a handler directly.
- `httptest.NewServer` when the code under test is an outbound *client* and needs
  a real URL to call.

Assert on status, headers and body shape — the adapter's whole job. Business
assertions belong in the `app` tests.

## Fuzzing

Anything that parses untrusted input — wire formats, config files, user-supplied
paths or queries — gets a `FuzzXxx` alongside its unit tests. The seed corpus
lives in `testdata/fuzz/`, and any crash the fuzzer finds is committed there as a
regression case.

Fuzzing runs unbounded by default, so it belongs on a schedule (`-fuzztime`), not
in every PR. Seed-corpus execution still happens on a normal `go test` run, which
is what keeps found crashes from regressing.

## Coverage

Coverage is a diagnostic, not a target: chasing a percentage produces tests that
assert nothing. Use it to find what is *not* exercised — an uncovered error branch
is a real gap, an uncovered generated file is not.

For integration tests that exercise a built binary rather than a package, build
it with `go build -cover` and collect profiles via `GOCOVERDIR`.

## Mocking

- **`go.uber.org/mock/mockgen` (gomock)** by default — for interaction-based
  tests where assertions are about exact calls, arguments, call counts,
  matchers, or ordering.
- **`github.com/derision-test/go-mockgen`** for stateful fakes — when tests read better
  as world-state transitions than as long `EXPECT()` chains, especially when a
  dependency needs default behavior plus queued per-call overrides and optional
  post-hoc inspection of call history. List interface names explicitly with
  `-i` in `//go:generate`, because stateful mocks are expected to be rare.
- **testify mocks** only if the project already uses testify.
- Generate mocks for **all In/Out ports** in service architectures (see
  [architecture.md](architecture.md)).

`//go:generate` directive example — through `go tool` so every machine runs the
version pinned in `go.mod` (see
[engineering-policy.md § Code generation](engineering-policy.md)):

```go
//go:generate go tool mockgen -destination=mocks/port_mock.go -package=mocks example.com/proj/internal/port App,Repo
```

### Naming test doubles

A consistent name says what the double *does*, so a reader does not have to open
it:

- **Package** — append `test` to the package being faked: `creditcardtest`,
  `storagetest`. It ships beside the real package and is importable by other
  packages' tests.
- **Type** — name it for its behavior, not its mechanism: `AlwaysCharges`,
  `RejectsExpired`, `EmptyStore`. A generic `Stub`/`Fake` is fine when the
  package holds exactly one.
- **Variable** — prefix with the double's kind: `spyCC`, `stubClock`, `mockRepo`.

Do not name a double after the interface it implements (`MockPortApp`) and stop
there — that says how it was generated, not what the test expects of it.

## Test strategy by layer (services)

| Layer | Test type | Details |
|---|---|---|
| Business logic (`internal/app`) | Unit | Mock all Out ports via gomock. |
| Inbound adapters (`internal/in/*`) | Unit | Mock `port.App`; drive HTTP with `httptest`. |
| DAL adapter (`internal/dal`) | Integration | Real temporary database (e.g. testcontainers or per-test container). |
| Other Out adapters (`internal/out/*`) | Integration | Fake or real network services started by the test. |
| Service as a whole | Smoke | Each external API entrypoint; each subscriber/event-consumer type. |

- For importable apps, integration tests may import the app and start it via
  public wiring APIs — that is a valid use case and shapes the layout choice.

## What not to do

- Do not test stdlib or third-party libraries ("testing that `os.Open` works").
- Do not assert on implementation details (private state, internal call order)
  when behavior would do.
- Do not skip `t.Parallel()` just because it is easier — parallelism catches
  hidden shared state. (`t.Setenv`/`t.Chdir` are the exception, not an excuse.)
- Do not write a single mega-test for a function; split by case via `t.Run`.
- Do not export identifiers from production files for tests — use
  `export_test.go` (see [§ Test placement](#test-placement)).
- Do not `defer` cleanup inside a test helper — it fires too early; use
  `t.Cleanup`.
- Do not sleep to synchronize a concurrency test — use `synctest`, or inject a
  clock. Raising a sleep to stop flakiness hides the bug.
- Do not chase a coverage percentage.

---

## Sources

- [powerman `go-engineering-policy`](https://github.com/powerman/skills) — testing section, gomock + go-mockgen.
- [Google Go Best Practices](https://google.github.io/styleguide/go/best-practices) (CC-BY-3.0) — test-double naming (package/type/variable), runnable examples as documentation.
- [metalagman `go-senior-developer`](https://github.com/metalagman/agent-skills) — contract tests against OpenAPI/AsyncAPI, benchmark discipline (rules extracted, prose rewritten).
- Assertion-library choice (`vanilla` default, testify as fallback) is this skill's own resolution of a donor disagreement — the donors split on whether to prefer an assertion library.
