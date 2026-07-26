# Publish go-ultimate as a Claude Code Plugin

**Date:** 2026-07-26
**Status:** Approved
**Skill source:** `~/.zcode/skills/go-ultimate/` (v0.4.0, MIT)
**Target:** new repo `Djarvur/go-ultimate`, listed in the existing marketplace `Djarvur/cc-mplace`

## Goal

Publish the existing `go-ultimate` skill as an installable Claude Code plugin, and register
it in the curated `Djarvur/cc-mplace` marketplace so users can install it with
`claude plugin install go-ultimate`.

## Context discovered

- **Skill source** lives at `/Users/nil/.zcode/skills/go-ultimate/` and contains
  `SKILL.md` (12 KB, v0.4.0, `license: MIT`, `user-invocable: true`) plus `assets/`,
  `references/`, `scripts/`, `evals/`. Content is harness-agnostic (the SKILL.md
  `<skill-dir>` table already covers Claude Code, ZCode, Cursor, Codex).
- **GitHub org** is `Djarvur` (an existing organization; admin access via account
  `onokonem`). Note: `Djarvir` / `Djarvur` spellings — the correct one is `Djarvur`.
- **Marketplace repo** `Djarvur/cc-mplace` already exists with one plugin
  (`cc-websearch`). Its `.claude-plugin/marketplace.json` uses the official schema
  (`https://json.schemastore.org/claude-code-marketplace.json`) with fields:
  `name`, `displayName`, `version`, `description`, `author.name`, `category`, and
  `source: { source: "url", url: "<repo>.git" }`. The README states plugins are added
  via PR, and CI validates entries against the schema (AJV + Vitest).
- **Reference plugin layout** is `superpowers`: a repo root with `.claude-plugin/plugin.json`
  (manifest only) and `skills/<name>/SKILL.md`. A repo can additionally carry its own
  `.claude-plugin/marketplace.json` to double as a self-contained marketplace — we do
  **not** need that here because listing lives in `cc-mplace`.

## Design

### Repo 1 — `Djarvur/go-ultimate` (new)

Plugin-native layout. No `marketplace.json` in this repo (listing is centralised in
`cc-mplace`).

```
go-ultimate/
├── .claude-plugin/
│   └── plugin.json              # plugin manifest only
├── skills/
│   └── go-ultimate/
│       ├── SKILL.md             # copied verbatim from ~/.zcode/skills/go-ultimate/
│       ├── assets/
│       ├── references/
│       ├── scripts/
│       └── evals/
├── README.md                    # what it is, install instructions, license
├── LICENSE                      # MIT
└── .gitignore                   # OS / editor cruft
```

#### `.claude-plugin/plugin.json`

```json
{
  "$schema": "https://json.schemastore.org/claude-code-plugin.json",
  "name": "go-ultimate",
  "description": "Opinionated Go development skill — architecture, conventions, review, and MCP/agent patterns, synthesized from six general-Go skills and five agent/MCP donors.",
  "version": "0.4.0",
  "author": { "name": "Djarvur" },
  "homepage": "https://github.com/Djarvur/go-ultimate",
  "repository": "https://github.com/Djarvur/go-ultimate",
  "license": "MIT",
  "keywords": ["go", "golang", "skill", "code-review", "mcp", "agent"]
}
```

- `version` matches `SKILL.md` metadata (`0.4.0`) so consumers can map plugin releases
  to skill releases.
- `description` is the short marketplace-facing summary; the long-form pitch lives in
  SKILL.md and README.

#### Skill contents

Copied **verbatim** from `/Users/nil/.zcode/skills/go-ultimate/` into
`skills/go-ultimate/`. No edits to `SKILL.md` or any bundled file. The skill already
declares `license: MIT` and `user-invocable: true` and is harness-agnostic.

#### `README.md`

Sections:
1. **What it is** — one paragraph: a single opinionated skill for Go, synthesized from
   six general-Go skills and five agent/MCP donors; routes by project type.
2. **Install** — two paths:
   - Marketplace: `claude plugin marketplace add Djarvur/cc-mplace` then
     `claude plugin install go-ultimate`.
   - Direct clone for non-Claude-Code harnesses: clone and point the harness at the
     `skills/go-ultimate/` directory (link to the SKILL.md `<skill-dir>` table).
3. **What's inside** — bullet list of the skill's bundled resources
   (`assets/`, `references/`, `scripts/`, `evals/`).
4. **License** — MIT, with attribution note pointing at SKILL.md `metadata.sources`
   for the upstream skill donors.

#### `LICENSE`

Standard MIT text, copyright `Djarvur`, year 2026.

#### `.gitignore`

OS/editor cruft only (`.DS_Store`, `*.swp`, `.idea/`, `.vscode/`); no build artifacts
expected (the Go scripts under `scripts/` are read by the skill, not compiled by CI).

### Repo 2 — `Djarvur/cc-mplace` (existing, PR)

Two file edits, matching the established `cc-websearch` entry shape exactly.

#### `.claude-plugin/marketplace.json` — append entry

```json
{
  "name": "go-ultimate",
  "displayName": "Go Ultimate",
  "version": "0.4.0",
  "description": "Opinionated Go development skill — architecture, conventions, review, MCP/agent patterns",
  "author": { "name": "Djarvur" },
  "category": "languages",
  "source": {
    "source": "url",
    "url": "https://github.com/Djarvur/go-ultimate.git"
  }
}
```

Fields mirror `cc-websearch` (same key order, same `source` shape, same `author.name`
form). Per the marketplace JSON Schema (`json.schemastore.org/claude-code-marketplace.json`),
only `name` and `source` are required at the plugin-entry level; `category` is a
free-form string (examples in the schema: `"productivity"`, `"development"`). We use
`"languages"`, consistent with the free-form convention and parallel to the existing
`"search"` entry.

#### `README.md` — add a table row

Append to the "Available Plugins" table:

```
| go-ultimate | Opinionated Go development skill — architecture, conventions, review, MCP/agent patterns |
```

### Publishing flow

1. **Build** the `go-ultimate` plugin repo locally in a temp dir.
2. **Create + push** the plugin repo:
   `gh repo create Djarvur/go-ultimate --public --source <dir> --push`.
3. **Marketplace PR**: in a `cc-mplace` working clone, edit `marketplace.json` and
   `README.md`, push a branch, open a PR. Let CI (AJV schema validation + Vitest) run
   green before merging. Branch back to `main` after merge.
4. **Verify** the plugin URL resolves and the marketplace entry parses (fetch
   `raw.githubusercontent.com/Djarvur/cc-mplace/main/.claude-plugin/marketplace.json`
   and confirm the new entry is present and schema-valid).

## Out of scope (YAGNI)

- **Cross-harness shims** (`.cursor-plugin/`, `.codex-plugin/`, `gemini-extension.json`):
  superpowers has these because it is cross-harness; we target Claude Code (ZCode
  consumes the same plugin format). Can be added later if a non-CC harness needs them.
- **Hooks and slash commands**: this is a single skill; nothing to hook.
- **CHANGELOG / RELEASE-NOTES**: only v0.4.0 exists today; start one at v0.5.0.
- **CI in the plugin repo**: the skill is static markdown plus Go scripts that the skill
  itself drives; add CI if/when the scripts need test coverage.
- **GSD workflow** in `cc-mplace`: that repo's `CLAUDE.md` mentions a `/gsd-quick` etc.
  convention for its own development. For a two-line registry PR we proceed directly;
  revisit if the maintainer wants GSD used for registry edits.
