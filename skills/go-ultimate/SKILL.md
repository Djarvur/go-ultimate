---
name: go-ultimate
description: >
  The complete Go development skill. REQUIRED for ALL Go work — writing, refactoring,
  reviewing, generating, scaffolding, or discussing Go code, including any change to .go
  files. Use whenever the user mentions Go, Golang, go.mod, a Go package, or asks to
  build a Go backend service, CLI, library, script, MCP server (Model Context Protocol),
  or AI/LLM agent with tool calling or ReAct-style reasoning. Routes by project type to
  the right architecture, conventions, and review checklist. Highly opinionated;
  overrides generic Go style and lint guidance on conflicts.
user-invocable: true
license: MIT
compatibility: Designed for Claude Code, and for any harness that reads the Agent Skills SKILL.md format. Targets any Go project.
metadata:
  author: go-ultimate-skill
  version: '0.4.1'
  sources:
    - teetsh-org/claude-skills#golang-code-review
    - JetBrains/go-modern-guidelines#use-modern-go
    - powerman/skills#go-bounded-context-hexagonal
    - powerman/skills#go-engineering-policy
    - cloudwego/eino#orchestration-design-principles
    - cloudwego/eino#adk-0.1
    - modelcontextprotocol/go-sdk#design
    - philschmid.de#mcp-best-practices
    - modelcontextprotocol.info#docs/best-practices
  source-notes:
    danicat-skills-deleted: |
      Originally also sourced from danicat/skills#go-best-practices and
      danicat/skills#go-project-setup; both skills were deleted upstream on
      2026-07-21 (commit 201fa420, "skill optimisation and repo cleanup"). Their
      distilled content is now owned and maintained by go-ultimate — the
      project-setup boilerplates live under assets/ (verified to build with
      Go 1.26.x) and the best-practices material is folded into
      references/engineering-policy.md and references/code-review.md.
    cloudwego-eino-adk-0.1: |
      `adk-0.1` is an in-repo ADK package, not a versioned tag; treated as a
      point-in-time reference, not a recoverable pin.
---

# Go Ultimate Skill

A single, opinionated skill for writing the best Go programs — synthesized from
four general-Go skills plus four agent/MCP donors (one donor, eino, contributes
two documents). Opinions represent the synthesized consensus; where the donors
disagreed, this skill resolved the conflict per the precedence ladder below.

## When this skill applies

**Always**, for any Go work: code generation, refactoring, code review,
architecture and style discussions, scaffolding, or any change to `.go` files.

It overrides generic Go style/lint guidance when they conflict.

## Setup — resolving `<skill-dir>`

Several instructions in this skill reference `<skill-dir>` — the absolute path to
this installed skill directory. Resolve it once per session.

**Primary rule:** `<skill-dir>` is the directory containing this `SKILL.md`.
That always works, regardless of how the skill was installed. The bundled
detector is at `<skill-dir>/scripts/goversion/main.go`; the templates are at
`<skill-dir>/assets/`.

For specific harnesses, the install path is:

| Harness | Skill location |
|---|---|
| Claude Code (plugin) | `${CLAUDE_PLUGIN_ROOT}/skills/go-ultimate` — the harness resolves this at runtime; the path includes a version directory, so do not hardcode it. |
| Claude Code (manual clone) | the cloned `skills/go-ultimate/` directory. |
| ZCode | `~/.zcode/skills/go-ultimate` |

> **Other harnesses** (Cursor, Codex CLI, OpenCode): the table entries that
> previously appeared here were unverified — nothing in this repo establishes
> that those agents read the `SKILL.md` format from a fixed directory. If your
> harness supports the Agent Skills `SKILL.md` format, point it at this
> directory; otherwise this skill is not confirmed to work there.

## How to use this skill

1. **Detect the project's Go version** by running the bundled detector
   `go run <skill-dir>/scripts/goversion/main.go <project-path>` (it prints the
   bare version, e.g. `1.24.3`, and always exits 0 — falling back to the Go
   runtime version if no `go.mod` or no `go` directive is found). Use features
   up to that version. See [references/modern-go.md](references/modern-go.md).
2. **Identify the project type** using the decision tree below, then load the
   matching reference.
3. **Follow the non-negotiable principles** below — these always apply.
4. **On conflicts** between references, apply the precedence rules at the end.

---

## Project-type decision tree

Ask "what is the user building?" and route accordingly:

```
Is the code a single binary that the user runs?
├── Yes → Is it < ~300 lines with no external deps beyond stdlib?
│   ├── Yes → SCRIPT / TINY TOOL
│   │          → references/project-layouts.md § Scripts
│   │          → references/engineering-policy.md
│   └── No  → CLI APPLICATION
│              → references/project-layouts.md § CLI
│              → references/engineering-policy.md
│
Is the code meant to be imported by other modules?
├── Yes → LIBRARY / MODULE
│          → references/libraries.md
│          → references/project-layouts.md § Library
│          → references/engineering-policy.md
│
Is the code a long-running server (HTTP/gRPC/messaging)?
├── Yes → BACKEND SERVICE
│          → references/architecture.md        (mandatory)
│          → references/project-layouts.md § Service
│          → references/engineering-policy.md
│
Is the code a server exposing tools/resources/prompts to LLMs over MCP?
├── Yes → MCP SERVER
│          → references/mcp-server.md          (mandatory)
│          → references/project-layouts.md § Service or § CLI (stdio server = CLI-shaped)
│          → references/engineering-policy.md
│
Is the code an LLM-driven agent that calls tools / orchestrates multi-step reasoning?
├── Yes → AI AGENT
│          → references/agents.md              (mandatory)
│          → references/project-layouts.md § CLI or § Service (depends on hosting)
│          → references/engineering-policy.md
│
Is the code a code generator, linter, or analysis tool?
├── Yes → Treat as CLI APPLICATION or LIBRARY (whichever fits the distribution),
│          then see references/engineering-policy.md § Code generation
│
Otherwise → ask the user to clarify before proposing structure.
```

---

## Non-negotiable principles (always apply)

These are the consensus backbone. Every Go file this skill touches obeys them.

1. **Modern Go, version-gated.** Use every feature up to the project's `go.mod`
   version. `any` over `interface{}`. `errors.Is/As` over `==`. `slices`/`maps`/
   `cmp` over hand-rolled loops. See [references/modern-go.md](references/modern-go.md).

2. **No `pkg/` directory.** Ever. Library code lives at the module root;
   importable apps expose a public `port/` package. The `pkg/` convention is a
   kubernetes-era anti-pattern. See [references/project-layouts.md](references/project-layouts.md).

3. **`internal/` for private code; `cmd/<name>/` for multiple binaries; flat for
   tiny tools.** Start simple; grow deliberately.

4. **Context first, errors explicit.** `func F(ctx context.Context, ...) (..., error)`.
   Wrap with `fmt.Errorf("...: %w", err)`. Never compare errors with `==`.

5. **Thin adapters, fat-free imports.** Business-logic packages never import
   `net/http`, `os`, or transport SDKs. Adapters only translate formats and
   protocol shapes.

6. **Concurrency hygiene.** Every goroutine has an exit condition (Context or
   WaitGroup). Use `errgroup` for parallel fan-out. Always test with `-race`.

7. **Accept interfaces, return structs.** Interfaces are defined at the point of
   use, not the point of implementation. Small, single-method interfaces win.

8. **Comments explain why, not what.** Named, idiomatic, exported identifiers do
   the rest.

9. **Typed contracts at boundaries.** MCP tool args and agent inter-node data
   flow are typed structs (generics where the framework supports it), never
   `map[string]any` as the universal contract. See
   [references/mcp-server.md](references/mcp-server.md) and
   [references/agents.md](references/agents.md).

---

## Quick reference (top rules, with link to detail)

| Rule | Detail |
|---|---|
| Detect Go version, use features up to it | [modern-go.md](references/modern-go.md) |
| Pick layout by project type | [project-layouts.md](references/project-layouts.md) |
| Service architecture: bounded-context hexagonal | [architecture.md](references/architecture.md) |
| `package main`: thin, `run(ctx, cfg) error` pattern, no testable logic in `main()` | [engineering-policy.md](references/engineering-policy.md) |
| Config: Resolvable Config Struct (not Functional Options) | [engineering-policy.md](references/engineering-policy.md) |
| `if x := f(); cond` only when init is the condition; else separate statement | [engineering-policy.md](references/engineering-policy.md) |
| Errors: sentinels via `errors.Is`, typed via `errors.As`, wrap with `%w`, aggregate with `errors.Join` | [engineering-policy.md](references/engineering-policy.md) |
| Tests: vanilla `testing`, `package xxx_test`, `TestF_suffixCamelCase` names | [testing.md](references/testing.md) |
| Mocks: `go.uber.org/mock` (gomock) default; testify-mocks only if already on testify | [testing.md](references/testing.md) |
| Review: Critical / Important / Suggestion / Positive buckets, What-Why-How per issue | [code-review.md](references/code-review.md) |
| Libraries: README (rationale + honest comparison + payoff-first quick start) vs `doc.go` (contracts) | [libraries.md](references/libraries.md) |
| `go.mod`: apps latest, libraries latest-1 | [engineering-policy.md](references/engineering-policy.md) |
| Lint: `golangci-lint` + `govet` + `go test -race` mandatory in CI | [engineering-policy.md](references/engineering-policy.md) |
| MCP server: official `go-sdk`, typed tool handlers, two-channel errors, tool design as outcomes-not-operations | [mcp-server.md](references/mcp-server.md) |
| AI agent: ReAct loop w/ bounded iterations, 6 topology archetypes, type-aligned composition (reject `map[string]any` between nodes) | [agents.md](references/agents.md) |

---

## Conflict precedence (which reference wins)

When two references seem to disagree:

1. **Layout / architecture / ports / wiring / modular monolith** for services →
   [architecture.md](references/architecture.md) wins.
2. **Version-gated syntax** (whether a feature exists in this Go version) →
   [modern-go.md](references/modern-go.md) wins.
3. **Everything else** (config, errors, testing, naming, linting, dependencies,
   CI) → [engineering-policy.md](references/engineering-policy.md) wins.
4. **Library public-API concerns** (semver, deprecation, doc.go split) →
   [libraries.md](references/libraries.md) wins for library projects.
5. **Code review output format** → [code-review.md](references/code-review.md) wins.
6. **MCP servers in Go** (SDK choice, tool-handler design, two-channel errors,
   pagination, middleware) → [mcp-server.md](references/mcp-server.md) wins.
7. **AI agents in Go** (ReAct loop, multi-agent topology, typed data flow
   between nodes, state externalism) → [agents.md](references/agents.md) wins.

The adapter carve-out ([mcp-server.md](references/mcp-server.md) § "Adapter
carve-out") refines rule 5 of the non-negotiable principles for MCP handlers
only: the handler package is an inbound adapter and may import `mcp.*`.

## Project-file precedence (escalation rule)

When a project-level instruction file contradicts this skill, **follow this
order**, stopping at the first match:

1. **Explicit user instruction in the current turn** — always wins. If the user
   says "use Functional Options here," use them, even though the skill mandates
   Resolvable Config Struct.
2. **Project `AGENTS.md` / `CLAUDE.md` at the repo root** — wins over this skill
   for project-specific conventions. A project that legitimately needs `pkg/`
   (e.g. a kubernetes-style repo) says so there.
3. **This skill** — the default for any Go decision not addressed above.
4. **Generic Go style/lint guidance** — lowest priority; this skill overrides it
   on conflicts by design.

If (1) and (2) are silent and the skill's rule feels wrong for the project,
**say so out loud before applying it** — propose the deviation, name the rule it
contradicts, and let the user decide. Do not silently override the skill, and do
not silently apply it when a project file arguably contradicts it.

## Verification

After applying this skill to a Go project, the project should pass:

```
go vet ./...
go build ./...
go test -race ./...
```

If any of these fail as a result of changes made under this skill, that is a
skill regression — surface it, do not paper over it. For new projects scaffolded
from `assets/`, the result should compile and test green before handing control
back to the user.

## Evaluating the skill itself

Example prompts and expected behaviors for catching drift live in
[`evals/`](evals/README.md). Run them when the skill is updated. The audit
methodology that produced this skill is not shipped with it; the rules above are
the canonical source.

This skill is **highly opinionated**. Do not relitigate these rules at runtime —
apply them, and note any project-specific deviation in the project's own
`AGENTS.md` / `CLAUDE.md` if one exists.
