# go-ultimate

A single, opinionated AI agent skill for writing the best Go programs — synthesized from four general-Go skills, four agent/MCP donors (one donor contributes two documents), the Google and Uber style guides, and a repository/production-practice donor. Routes any Go task (CLI, library, backend service, MCP server, AI agent) to the right architecture, conventions, and review checklist. Highly opinionated; overrides generic Go style and lint guidance on conflicts.

> **Status:** v0.12.0. Designed for Claude Code, Codex, Cursor, Grok Build, GitHub Copilot CLI, OpenCode, and any harness that reads the Agent Skills `SKILL.md` format. MIT licensed.

## What's inside

The skill lives under [`skills/go-ultimate/`](skills/go-ultimate/) and is structured for progressive disclosure:

| Path | Purpose |
|---|---|
| `skills/go-ultimate/SKILL.md` | The skill itself — when it applies, the non-negotiable principles, the project-type decision tree, conflict precedence. |
| `skills/go-ultimate/references/` | Deep dives loaded on demand: `architecture.md`, `engineering-policy.md`, `modern-go.md`, `testing.md`, `code-review.md`, `libraries.md`, `project-layouts.md`, `wiring-and-runtime.md`, `modular-monolith.md`, `production-readiness.md`, `repo-and-ci.md`, `mcp-server.md`, `agents.md`. |
| `skills/go-ultimate/assets/` | Scaffolded project templates per type: `cli-simple`, `cli-cobra`, `library`, `game`, `mcp-server`, `webservice` (all `build`, `vet` and `test -race` green), plus `ci/` workflows. |
| `skills/go-ultimate/scripts/` | `goversion/main.go` (detects the project's `go.mod` version) and `check-source-versions.sh` + `source-versions.json` (drift checker for upstream donor sources). |
| `skills/go-ultimate/evals/` | Example prompts and expected behaviors for catching skill drift. |

## Install

go-ultimate ships a plugin manifest per harness (see the repo root), so the same skill installs natively on each. Install via the [Djarvur marketplace](https://github.com/Djarvur/cc-mplace) (`Djarvur/cc-mplace`), which indexes go-ultimate for every supported host. Once installed, it auto-triggers on any Go task (mentions of Go, Golang, `go.mod`, a Go package, MCP server, or AI agent).

### Claude Code

```bash
claude plugin marketplace add Djarvur/cc-mplace
claude plugin install go-ultimate
```

### Codex

```bash
codex plugin marketplace add Djarvur/cc-mplace
codex plugin add go-ultimate@djarvur-plugin-marketplace
```

Invoke with `$go-ultimate:go-ultimate`.

### Cursor

```bash
agent plugin marketplace add https://github.com/Djarvur/go-ultimate.git --git-ref v0.12.0
```

Then install go-ultimate from the Cursor marketplace UI.

### Grok Build

```bash
grok plugin install 'Djarvur/cc-mplace#go-ultimate' --trust
```

### GitHub Copilot CLI

```bash
copilot plugin marketplace add Djarvur/cc-mplace
copilot plugin install go-ultimate@djarvur-plugin-marketplace
```

### OpenCode

OpenCode consumes skills as files rather than a marketplace bundle. Copy the skill into your project (or globally below `~/.config/opencode/`):

```bash
src="$(mktemp -d)"
git clone --depth 1 https://github.com/Djarvur/go-ultimate.git "$src"
mkdir -p .opencode
cp -R "$src/skills/go-ultimate" .opencode/skills/
rm -rf "$src"
```

Invoke via OpenCode's skill tool; the `SKILL.md` documents `<skill-dir>` resolution.

### ZCode

Copy or symlink `skills/go-ultimate/` into `~/.zcode/skills/`. ZCode also recognizes the `.claude-plugin/` manifest.

### Manual / unsupported harness

Clone and point your harness at the skill directory:

```bash
git clone https://github.com/Djarvur/go-ultimate.git
```

The `SKILL.md` documents `<skill-dir>` resolution per harness.

## How it works

When triggered, the skill:

1. Detects the project's Go version (`go run <skill-dir>/scripts/goversion/main.go <project>`) and gates feature usage on it.
2. Routes the project through a decision tree — script / CLI / library / backend service / MCP server / AI agent — and loads the matching reference.
3. Applies nine non-negotiable principles (modern version-gated Go, no `pkg/`, context-first, thin adapters, concurrency hygiene, etc.).
4. Resolves conflicts via an explicit precedence ladder (architecture → modern-go → engineering-policy → libraries → code-review → mcp-server → agents → production-readiness → repo-and-ci).

See the [`SKILL.md`](skills/go-ultimate/SKILL.md) for the full opinionated rule set.

## Donor sources

The skill synthesizes thirteen upstream sources. Full attribution and the live pins used to detect upstream drift are in [`skills/go-ultimate/scripts/source-versions.json`](skills/go-ultimate/scripts/source-versions.json). Summary:

**General Go:** `teetsh-org/claude-skills#golang-code-review`, `JetBrains/go-modern-guidelines#use-modern-go`, `powerman/skills#go-bounded-context-hexagonal` (v0.6.0), `powerman/skills#go-engineering-policy` (v0.0.5).

**Style guides:** [`google/styleguide#go`](https://google.github.io/styleguide/go/) (CC-BY-3.0), [`uber-go/guide`](https://github.com/uber-go/guide) (Apache-2.0).

**Repository / production practice:** `metalagman/agent-skills#go-senior-developer`, `#go-oss-maintainer`, `#golangci-lint-strict`.

**Agent / MCP:** `cloudwego/eino#orchestration-design-principles`, `cloudwego/eino#adk-0.1`, `modelcontextprotocol/go-sdk` (v1.7.0), `philschmid.de#mcp-best-practices`, `modelcontextprotocol.info#docs/best-practices`.

> Originally also sourced from `danicat/skills#go-best-practices` and `danicat/skills#go-project-setup`; both were deleted upstream on 2026-07-21. Their distilled content is now owned and maintained by this skill (project-setup boilerplates under `assets/`, best-practices material folded into `references/engineering-policy.md` and `references/code-review.md`).

> `metalagman/agent-skills` ships no LICENSE file, so its text is all-rights-reserved by default. Nothing is copied from it verbatim — the rules were extracted and rewritten, with the donor credited in each affected reference. Its bulk style-guide documents are derivatives of the Google and Uber guides, which this skill pins and attributes directly from upstream instead. Its tool-specific playbooks (`go-fx`, `go-goose`, `go-options-gen`, `go-adk`) are deliberately **not** folded in: they are version-fragile, they contradict this skill's defaults (hand-written wiring over Fx, Resolvable Config Struct over options-gen), and an always-on skill should not carry per-library tutorials.

### Checking for upstream drift

```bash
bash skills/go-ultimate/scripts/check-source-versions.sh
```

Exit 0 = no drift, 1 = a donor changed since the pin was recorded, 2 = fetch failure. Refresh the pin in `source-versions.json` whenever you fold a change back into the skill.

## License

MIT — see [LICENSE](LICENSE).
