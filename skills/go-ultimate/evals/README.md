# Evals — go-ultimate skill

Example prompts and their expected behaviors, for catching skill drift and
trigger problems. Each eval is a prompt the skill *should* activate on, with
the observable behavior that confirms it activated correctly.

## How to use

Run each prompt against a fresh agent session in a Go project (or an empty
directory for scaffolding prompts). Compare the agent's behavior to the
"Expected behavior" checklist. Any mismatch is a regression.

These are lightweight smoke tests, not a full eval harness — see
[`../../skill-audit-report.md`](../../skill-audit-report.md) §6 for the rationale.

---

## Eval 1 — version detection (modern-go trigger)

**Setup:** any Go project with a `go.mod` whose `go` directive is `1.24` or later.

**Prompt:**
> "Write a function that returns the first non-empty string from a list, defaulting to 'unknown'. Use modern Go."

**Expected behavior:**
- Skill activates (Go work, mentions "modern Go").
- Agent runs the bundled detector:
  `go run <skill-dir>/scripts/goversion/main.go .`
- Agent states the detected version once, e.g. *"This project is using Go 1.24, so I'll use features up to and including this version."*
- For Go 1.22+, the answer uses `cmp.Or(list...)` with `"unknown"` as the final fallback
  (`cmp.Or(append(list, "unknown")...)`, or `cmp.Or(cmp.Or(list...), "unknown")`) — not an
  `if`/`else` chain.
- For Go < 1.22, the answer falls back to an `if`/`else` (the skill does not use features newer than the target).

**Failure signals:**
- Agent does not detect the Go version.
- Agent uses `cmp.Or` when the project is on Go < 1.22.
- Agent uses an `if`/`else` chain when the project is on Go 1.22+.
- Agent lists every Go feature instead of stating the version once and proceeding.

---

## Eval 2 — scaffolding (project-layouts + assets trigger)

**Setup:** empty directory.

**Prompt:**
> "Scaffold a new HTTP service called 'greeter' that exposes GET /hello?name=X and returns a greeting."

**Expected behavior:**
- Skill activates (building a Go backend service).
- Agent identifies the project type via the decision tree → BACKEND SERVICE.
- Agent copies from `assets/webservice/` (or recreates that layout from scratch):
  - `internal/port/port.go` with an `App` interface
  - `internal/app/app.go` implementing `port.App`
  - `internal/in/http/handler.go` — thin adapter, no business logic
  - `main.go` — thin, calls `run()`
  - `wire.go` — wiring only, starts nothing
- **No `pkg/` directory anywhere.**
- `main()` contains no testable logic — `run()` does.
- Graceful shutdown via `signal.NotifyContext`.
- Module path set to the user's actual path (not `example.com/webservice`).
- After scaffolding, agent runs `go build ./...` and reports the result.

**Failure signals:**
- `pkg/` directory appears in the scaffold.
- Business logic in `internal/in/http/` (instead of `internal/app/`).
- No `wire.go`, or `wire.go` starts the HTTP server.
- `main()` contains request-handling logic.

---

## Eval 3 — code review (code-review trigger)

**Setup:** a Go file with deliberate issues:

```go
package user

func Get(u *User) (*User, error) {
    if u == nil {
        return nil, nil
    }
    err := db.Save(u)
    if err == nil { return u, nil } else { return nil, err }
}
```

**Prompt:**
> "Review this Go function."

**Expected behavior:**
- Skill activates (reviewing Go code).
- Agent produces structured feedback with the four buckets: **Critical / Important / Suggestions / Positive**.
- Issues use the **What / Why / How** format with before/after code.
- At minimum, the agent flags:
  - **Important:** returns `(*User, error)` = `nil, nil` on the bad-input path — in-band error.
  - **Important:** `else` after `return` — should be early-return style.
  - **Suggestion:** wrap the save error with `fmt.Errorf("save user: %w", err)`.
  - **Suggestion:** `db.Save` and `nil` comparison suggest business logic mixed with storage — adapter discipline.
- Agent does **not** flag gofmt-level formatting (the framework says skip those).

**Failure signals:**
- Feedback is unstructured prose, not the four buckets.
- Agent misses the `nil, nil` in-band error.
- Agent nitpicks formatting.

---

## Eval 4 — conventions (engineering-policy trigger)

**Setup:** a Go constructor using Functional Options:

```go
func NewServer(opts ...Option) *Server { ... }
```

**Prompt:**
> "I'm writing a Go HTTP client constructor. Should I use functional options?"

**Expected behavior:**
- Skill activates (discussing Go conventions).
- Agent recommends the **Resolvable Config Struct** pattern (not Functional Options) per engineering-policy.md.
- Agent explains the three rules: zero-value-means-default, `Clone()` if references, idempotent `Resolve()`.
- Agent shows a concrete `Config` struct + `Resolve()` example.
- Agent names the exception: Functional Options only for genuinely streaming/fluent builder APIs.
- Agent cites the source: "this overrides Functional Options per
  engineering-policy.md / powerman go-engineering-policy."

**Failure signals:**
- Agent recommends Functional Options as the default.
- Agent presents both as equal choices without naming the skill's default.
- Agent cannot recall the `Resolve()` contract.

---

## Eval 5 — MCP server (mcp-server trigger)

**Setup:** empty directory or a project that needs tool-exposure to LLMs.

**Prompt:**
> "Build an MCP server in Go that exposes a 'search_docs' tool."

**Expected behavior:**
- Skill activates (MCP server + Go).
- Agent uses the **official `modelcontextprotocol/go-sdk`** (not `mark3labs/mcp-go`) as the default, with a one-line rationale if asked.
- Agent uses **typed tool handlers via the generic `mcp.AddTool`** (`In`/`Out` inferred from the handler) — not `map[string]any` args.
- Agent implements **two-channel errors**: tool-execution failure as `*mcp.CallToolResult{IsError: true}`, protocol failure as a Go `error`.
- Agent designs the tool around the **outcome** ("search and return top result with snippet"), not a CRUD operation.
- Tool args are a **flat typed struct** with constrained types.
- Agent may start from `assets/mcp-server/` template.

**Failure signals:**
- Agent defaults to `mark3labs/mcp-go` without rationale.
- Tool args use `map[string]any`.
- Agent conflates tool errors with protocol errors.
- Tool is a thin CRUD wrapper that makes the LLM orchestrate multiple round-trips.

---

## Negative evals — where the skill must NOT activate

The five evals above check **under-triggering** (skill should fire but doesn't).
These check **over-triggering**: prompts that look superficially Go-adjacent
but where this skill has nothing to contribute. The skill must stay dormant —
no `go run <skill-dir>/scripts/goversion/`, no "as a Go skill I recommend…",
no loading of references.

The risk this catches: the description lists "Go, Golang, go.mod, Go package"
as triggers, and an over-eager matcher fires on anything token-shaped like
those (MCP in another language, "go" as an English verb, a Rust workspace).

### Negative eval N1 — non-Go language

**Setup:** a Python or Rust project (has `pyproject.toml` or `Cargo.toml`, no
`go.mod`).

**Prompt:**
> "Add an endpoint to this service that returns a user by id."

**Expected behavior:**
- Skill does **not** activate (no Go context).
- Agent works in the project's actual language; no Go scaffolding, no
  hexagonal-layout prescription, no `go run` invocation.

**Failure signals (over-triggering):**
- Agent runs `goversion` or mentions the go-ultimate skill.
- Agent proposes Go-style layout (`internal/port`, `wire.go`) in a non-Go repo.
- Agent suggests switching the project to Go.

### Negative eval N2 — "go" as an English verb

**Setup:** a non-Go repo or a general conversation. No `go.mod` anywhere in
the workspace.

**Prompt:**
> "I want to go through the codebase and find dead code. How should I approach
> this?"

**Expected behavior:**
- Skill does **not** activate. "go through" is the English verb, not the
  language.
- Agent answers the dead-code question generically (or with a
  language-specific skill if one applies).

**Failure signals:**
- Agent activates go-ultimate because the prompt contains the token "go".
- Agent runs `goversion` or prescribes Go conventions to a non-Go codebase.

### Negative eval N3 — MCP / agent work in a non-Go language

**Setup:** a TypeScript/Python project. No `go.mod`.

**Prompt:**
> "Build an MCP server for searching our docs."

**Expected behavior:**
- Skill does **not** activate. The MCP/agent references are scoped to *Go*
  MCP servers and *Go* agents ("MCP servers **in Go**", "AI agents **in
  Go**"); a TS/Python MCP server is out of scope.

**Failure signals:**
- Agent loads `references/mcp-server.md` and prescribes `go-sdk`, typed Go
  handlers, or the adapter carve-out for a TypeScript server.
- Agent suggests porting the project to Go to use the official SDK.

### Negative eval N4 — Go topic that the skill deliberately defers

**Setup:** a Go project with a `go.mod`.

**Prompt:**
> "Recommend a Go ORM."

**Expected behavior:**
- Skill may activate (Go work), but should **defer** rather than improvise.
  The skill has no ORM recommendation; engineering-policy.md covers data
  access at the architecture level (DAL adapter, transaction boundaries), not
  library choice.
- Agent should say so plainly: "the skill doesn't recommend an ORM; here's
  what it does say about data access" and point at architecture.md §Repo.

**Failure signals:**
- Agent invents an ORM recommendation and attributes it to the skill.
- Agent silently applies a project-file override without flagging the gap.

