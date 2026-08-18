# Libraries / Modules

**Source:** [powerman `go-engineering-policy` § Library Documentation](https://github.com/powerman/skills)
+ ultimate-skill additions for semver, public API stability, and major-version layout.

**Applies to:** code meant to be imported by other modules. For non-library
projects, see [project-layouts.md](project-layouts.md).

## Contents

- [Layout](#layout)
- [Library documentation — README vs `doc.go`](#library-documentation-readme-vs-docgo)
- [`go.mod` version policy](#gomod-version-policy)
- [Semver and public API stability](#semver-and-public-api-stability)
- [Public API discipline](#public-api-discipline)
- [Testing libraries](#testing-libraries)
- [Sources](#sources)

---

## Layout

```text
library/
├── go.mod                       module github.com/user/library
│                                go directive = latest-1
├── doc.go                       Package doc — contracts, invariants, concurrency.
├── *.go                         Public API at module root in package <name>.
├── internal/                    Private implementation.
├── example_test.go              Runnable examples (rendered on pkg.go.dev).
├── README.md                    Rationale, comparison, quick start, non-goals.
└── LICENSE
```

Rules:
- Public API at the module root in `package <name>` — **never** under `pkg/`.
- Everything else under `internal/`.
- Major versions v2+ on a `/vN` suffix path (separate `go.mod` with `/v2` in the
  module path) or on a release branch.

---

## Library documentation — README vs `doc.go`

**Split content by audience, not by topic.** `pkg.go.dev` renders both, so full
duplication is never needed; only the headline positioning stance may live in
both. Everything else is single-homed by the audience criterion.

### README — for the evaluating visitor ("should I use this?")

- **Rationale section first, before any code.** What is wrong with the typical
  approach in the existing ecosystem, and why this library solves the task from
  a different direction. A one-line description + feature list does not explain
  why the library exists.
- **Honest comparison with alternatives:**
  - Compare feature-by-feature **only** against libraries solving the same problem.
  - For libraries solving a *different* problem, say so explicitly and give a
    "pick them if …, pick this if …" rule.
  - Do **not** build a feature table where competitors lose by definition (they
    are not about this functionality).
- **Non-goals are positioning stances** — answers to "why is feature X missing",
  with reasoning.
- **Quick start must show the payoff, not just the API surface.** When the API
    separates roles (e.g. a private writer vs a read-only view), split the
    example into those roles and put state-changing calls in the role that owns
    them. Use comments to teach key semantics inline.
- **Guidance sections** must be understandable without knowing the author's
  other projects — generalize ("several services in one binary") instead of
  project-specific framing.

### `doc.go` — for the package user ("how do I use this correctly?")

- **Rationale states** what tasks the package solves and why this design — no
  ecosystem surveys or comparisons with alternatives (that is README material).
- **Document contracts and semantics** the user must follow: invariants,
  conventions, concurrency guarantees.
- **Non-goals in `doc.go` are usage guidance**, phrased as instructions (what to
  do instead), not positioning.
- No project-specific jargon from the repo the library was extracted from.

### Examples

Examples in README, doc comments, and `example_test.go` must promote the
library's recommended usage conventions (naming, embedding style) from the first
release, **consistently across all three places.**

```go
// example_test.go
package library_test

import "fmt"
import "github.com/user/library"

func ExampleNew() {
    x := library.New(library.Config{ /* ... */ })
    fmt.Println(x)
    // Output: ...
}
```

---

## `go.mod` version policy

- Libraries use **latest − 1** as the `go` directive. Upgrade only when a needed
  feature requires it. This maximizes downstream compatibility.
- Set the module path to `github.com/user/library` for v0/v1; append `/v2`,
  `/v3`, … for major versions ≥ 2.

---

## Semver and public API stability

- **v0** — anything goes. The API is not stable; document that clearly.
- **v1** — public API is stable. Breaking changes require a major bump.
- **v2+** — release on a `/vN` path; users must change their import path to upgrade.

### What counts as a breaking change
- Removing or renaming an exported identifier.
- Changing a function/method signature (parameter or result types, arity).
- Adding a *required* function result where there was none.
- Changing the documented behavior of an exported identifier.
- Adding a new required field to a public struct that has no zero-value default
  (use the Resolvable Config Struct pattern — see
  [engineering-policy.md](engineering-policy.md) — so new optional fields stay
  non-breaking).

### What is NOT a breaking change
- Adding a new exported identifier.
- Adding a new optional field to a public struct (zero-value = "use default").
- Adding a new method to an interface *if* all known implementations live in
  your module and you update them. (Caution: this is still risky for interfaces
  intended for user implementation — document whether they are for you or for
  users.)

### Deprecation
- Mark deprecated identifiers with a `// Deprecated:` comment (rendered on
  `pkg.go.dev` and flagged by `staticcheck`). Keep them working for at least one
  major version before removal.

```go
// Deprecated: Use NewWithConfig instead; Old will be removed in v3.
func Old() *T { ... }
```

---

## Public API discipline

- Public API = anything not under `internal/`.
- Keep the public surface **minimal** — every export is a maintenance promise.
- **Return concrete structs** from constructors; accept interfaces as
  parameters (this skill's rule — see [engineering-policy.md](engineering-policy.md)).
  Do not return interfaces, which locks callers into a single implementation
  and prevents returning a pointer-to-struct literal.
- Document every exported identifier with a doc comment starting with the
  identifier name.
- Do not leak internal types through the public API (return error as `error`,
  not as `*internalError`).
- **Assert interface compliance at compile time** where a type is meant to
  satisfy an interface it does not name:

  ```go
  var _ io.ReadCloser = (*Stream)(nil)
  ```

  Zero cost, no allocation, and it turns "the caller's build broke" into "our
  build broke" when a method signature drifts.
- **Never take a pointer to an interface.** `*io.Reader` is almost always a
  mistake: an interface value is already a reference pair. Use a pointer only for
  the concrete type, when its methods must mutate it.

### Document what the signature cannot say

A doc comment that restates the signature is noise. These four facts cannot be
inferred and must be written:

- **Concurrency safety** — "safe for concurrent use by multiple goroutines", or
  explicitly not. Callers otherwise guess, and guess wrong.
- **Ownership and cleanup** — who closes what, and whether a returned value
  aliases internal state. Anything returning an `io.Closer` says when to close it.
- **Sentinel errors a caller is expected to match**, named in the doc comment of
  the function that returns them.
- **Non-obvious parameter constraints** — units, valid ranges, whether nil is
  allowed and what it means.

---

## Testing libraries

- Tests live in `package library_test` (external test package).
- `example_test.go` for runnable examples.
- Cover the public API thoroughly; mock external deps.
- See [testing.md](testing.md).

---

## Sources

- [powerman `go-engineering-policy` § Library Documentation](https://github.com/powerman/skills) — README vs `doc.go` split by audience.
- Semver, major-version layout, public API stability, deprecation rules are ultimate-skill additions.
- [Uber Go Style Guide](https://github.com/uber-go/guide) (Apache-2.0) — compile-time interface assertion, no pointers to interfaces.
- [Google Go Best Practices](https://google.github.io/styleguide/go/best-practices) (CC-BY-3.0) — documenting concurrency safety, cleanup ownership, sentinel errors and parameter constraints.
- Aligned with [engineering-policy.md](engineering-policy.md) (`go.mod` policy, Resolvable Config Struct, "accept interfaces, return structs").
