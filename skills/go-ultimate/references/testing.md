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
  the code under test.
- **Tests must only test the project's own code**, not stdlib or third-party
  libraries. Mock external dependencies (e.g. `os`, filesystem, network).

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
- Use **table-driven tests** for multi-case logic.
- Use test helpers (`t.Helper()` + a helper func) to reduce duplication within
  and between tests.
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

`//go:generate` directive example:

```go
//go:generate mockgen -destination=mocks/port_mock.go -package=mocks example.com/proj/internal/port App,Repo
```

## Test strategy by layer (services)

| Layer | Test type | Details |
|---|---|---|
| Business logic (`internal/app`) | Unit | Mock all Out ports via gomock. |
| Inbound adapters (`internal/in/*`) | Unit | Mock `port.App`. |
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
  hidden shared state.
- Do not write a single mega-test for a function; split by case via `t.Run`.

---

## Sources

- [powerman `go-engineering-policy`](https://github.com/powerman/skills) — testing section, gomock + go-mockgen.
- Assertion-library choice (`vanilla` default, testify as fallback) is the ultimate-skill resolution; see [`../../skills-analysis.md`](../../skills-analysis.md) §3 conflict #3.
