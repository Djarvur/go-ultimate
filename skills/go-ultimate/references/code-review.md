# Go Code Review

**Source:** [teetsh-org `golang-code-review`](https://github.com/teetsh-org/claude-skills) structure
+ [danicat `go-best-practices`](https://github.com/danicat/skills) review checklist,
**with all Teetsh-specific patterns stripped** (`school_id`, `pkg/externals/tracker`,
`domain/repo/service/handler`, etc.). The structured-feedback format is generic.

## Contents

- [When to use this reference](#when-to-use-this-reference)
- [Review process](#review-process)
- [Review areas](#review-areas)
- [Output format (mandatory)](#output-format-mandatory)
- [When NOT to comment](#when-not-to-comment)
- [Multi-file review strategy](#multi-file-review-strategy)
- [After the review](#after-the-review)
- [Sources](#sources)

---

## When to use this reference

When reviewing Go code: full PR reviews, single file/function reviews,
architecture evaluation, test-quality checks, or auditing existing code.

---

## Review process

### 1. Understand the context
- Read the PR description / change purpose.
- Identify the domain/package being modified.
- Note whether the change touches core business logic, API endpoints, or tests.

### 2. Scope by review type
- **Full PR** — architecture + implementation + tests + security.
- **Architecture** — package structure, interfaces, separation of concerns,
  boundaries (cross-reference [architecture.md](architecture.md) for services).
- **Test** — coverage, behavior vs implementation, naming, isolation.
- **Single function/file** — implementation quality, naming, error handling.

### 3. Apply the review areas below.

### 4. Emit structured feedback (mandatory format).

---

## Review areas

### Idiomatic Go
- `any` over `interface{}` (Go 1.18+).
- `errors.Is/As` over `==`.
- Modern patterns up to the project's Go version (see [modern-go.md](modern-go.md)).
- "Accept interfaces, return structs."
- **Return a single struct instead of multiple values** when the values are
  related data (e.g. `UserResult{Name, Email, CreatedAt}` rather than three
  separate returns). Reserve multiple returns for the conventional `(value, error)`
  shape and for genuinely independent values.
- Interfaces defined at the point of use, not the point of implementation.
- Small, single-method interfaces preferred.
- `defer` for resource cleanup.
- Comments explain *why*, not *what*.

### Errors
- Wrapped at each layer boundary: `fmt.Errorf("doing X: %w", err)`.
- Sentinel errors for expected conditions callers handle (`var ErrXxx = errors.New(...)`).
- `errors.Join` for aggregations (Go 1.20+).
- No `==` for error comparison.
- No log-and-return-the-same-error.

### Concurrency
- Every goroutine has an exit condition (Context/WaitGroup/done channel).
- `errgroup` for parallel fan-out that fails fast.
- **Channels are closed by the sending goroutine, never the receiver.** Closing
  a channel from the consumer is a data race waiting to happen.
- **`select` blocks that wait on channels handle timeouts / cancellation** to
  prevent deadlocks (`case <-ctx.Done()`, `case <-time.After(...)`).
- No goroutine leaks: cancellation propagates, wait groups decrement.
- No shared mutable state without synchronization; no copied mutexes (`go vet`).
- `t.Parallel()` in tests to surface hidden shared state.

### Security (when relevant: auth, payments, user input, external APIs)
- Input validation and sanitization at the boundary.
- Parameterized SQL — no string concatenation. `db.Query("... WHERE id = $1", id)`.
- `crypto/rand` for tokens/secrets; `bcrypt`/`argon2` for passwords.
  (`math/rand` and `math/rand/v2` are **not** CSPRNGs.)
- TLS configuration explicit, not default-InsecureSkipVerify.
- Authorization checks at the entrypoint, not buried in business logic.
- **Paths from outside the program are confined** — `os.Root` / `os.OpenRoot`
  (Go 1.24+) rather than hand-rolled `filepath.Clean` checks, which are easy to
  get subtly wrong.
- **`os/exec` takes arguments as a slice**, never an interpolated shell string;
  do not invoke a shell unless the shell is the actual requirement.
- **Unbounded reads are bounded** — `http.MaxBytesReader` on request bodies, and
  a cap on decompressed size for any archive or compressed payload.

### Resource handling
- **No `http.DefaultClient` / `http.Get` / `http.Post` in production code** —
  no timeout. Explicit `*http.Client` with `Timeout`, plus a per-request context
  deadline. See [engineering-policy.md § Outbound HTTP](engineering-policy.md).
- Response bodies closed **and** drained, so connections return to the pool
  (`bodyclose`).
- Servers set `ReadHeaderTimeout` at minimum.
- Panics only on programmer error, never across a public API boundary; inbound
  adapters of long-running servers have a recover middleware.

### Testing
- Tests cover success and error paths.
- Isolated test state — no reliance on global mutable state or test order.
- Behavior verified, not implementation details.
- Vanilla `testing` (`t.Errorf`/`t.Fatalf`) — see [testing.md](testing.md).
- Test names describe what is tested (`TestF_emptyInput`).
- `package xxx_test` external test package.
- `t.Parallel()` default.

### Architecture (services only)
- Adapters thin — no business decisions in `internal/in/*` or `internal/out/*`.
- Business-logic package does not import `net/http`, `os`, or transport SDKs.
- One `App` port per bounded context.
- `wire.go` does wiring only; no server starts in it.
- `internal/app` not exposed publicly; `port.App` is the boundary.

### Layout / project hygiene
- No `pkg/` directory.
- `internal/` used aggressively for private code.
- `cmd/<name>/` for multiple binaries; `main.go` at root for single binary.
- `main()` contains no testable logic — `run(ctx, cfg) error` pattern for CLIs.

---

## Output format (mandatory)

Structure every review as:

```markdown
## Critical Issues
[Must fix before merge: security holes, data-loss risk, concurrency bugs, resource leaks]

## Important Issues
[Should fix: architecture problems, missing error handling, wrong business logic, test gaps]

## Suggestions
[Optional: clarity, performance, idiomatic Go]

## Positive Feedback
[What was done well — be specific]
```

For **each** issue, use the **What / Why / How** format with before/after code:

```markdown
### <Area>: <Brief description>

**Problem**: <What is wrong and why it matters>

**Current code**:
\`\`\`go
<problematic code>
\`\`\`

**Suggested fix**:
\`\`\`go
<corrected code>
\`\`\`

**Reference**: <link to the relevant rule, e.g. engineering-policy.md § Errors>
```

### Worked example

```markdown
### Function Design: mixed abstraction levels in GetUserData

**Problem**: the function mixes high-level flow with low-level SQL scanning,
making intent hard to follow and error wrapping inconsistent.

**Current code**:
\`\`\`go
func GetUserData(id int) (*User, error) {
    query := "SELECT * FROM users WHERE id = ?"
    row := db.QueryRow(query, id)
    var user User
    err := row.Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, err
    }
    // ... more low-level operations mixed with business decisions ...
}
\`\`\`

**Suggested fix**:
\`\`\`go
func GetUserData(id int) (*User, error) {
    user, err := findUserByID(id)
    if err != nil {
        return nil, fmt.Errorf("get user data: %w", err)
    }
    return user, nil
}

func findUserByID(id int) (*User, error) {
    query := "SELECT * FROM users WHERE id = ?"
    row := db.QueryRow(query, id)
    var user User
    if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
        return nil, err
    }
    return &user, nil
}
\`\`\`

**Reference**: engineering-policy.md § Errors (wrap at each layer boundary)
```

---

## When NOT to comment

Avoid feedback on:
- Trivial formatting issues — `gofmt`/`goimports` handle these; flag the missing
  tool, not the lines.
- Personal style preferences not codified by this skill.
- Nitpicks that do not affect functionality or maintainability.
- Issues already addressed in other comments.

---

## Multi-file review strategy

For PRs with many files:
1. Start with the architecture overview — new packages, major structural changes.
2. Review core business logic files first.
3. Review tests for core logic.
4. Review supporting files (adapters, DTOs, helpers).
5. Summarize the overall assessment at the end.

---

## After the review

If significant issues were found:
- Summarize the most important themes (one line each).
- Recommend whether changes are required before merge.
- Offer to explain any pattern or practice in detail.

Do **not** block a review on `Suggestions` — only on `Critical` and (with
judgment) `Important`.

---

## Sources

- Review structure (Critical/Important/Suggestion/Positive + What-Why-How) from
  [teetsh-org `golang-code-review`](https://github.com/teetsh-org/claude-skills).
- Senior review checklist items from [danicat `go-best-practices`](https://github.com/danicat/skills).
- Review areas aligned with the rules in [engineering-policy.md](engineering-policy.md)
  and [architecture.md](architecture.md).
- All Teetsh-specific patterns (`school_id`, `pkg/externals/tracker/client.go`,
  `domain/repo/service/handler`, multi-tenancy conventions) deliberately removed.
