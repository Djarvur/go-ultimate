# Upstream provenance

Six community skills are vendored in this repository. Each directory carries
its upstream license and is a snapshot of an external project, re-licensed to
you under that upstream license, not under go-ultimate's MIT. This file is the
passport: where each file came from, what was changed here, and why.

Check both upstreams for updates when cutting a release; their histories are
not tracked here.

## skills/ponytail/ and skills/ponytail-review/

- **Upstream:** [DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail)
  (`skills/ponytail/SKILL.md`, `skills/ponytail-review/SKILL.md`)
- **Pinned upstream version:** v5.3.3 (2026-10-29, commit 96608e0)
- **License:** MIT — see `LICENSE` in each directory (© DietrichGebert)
- **Vendored:** 2026-10-31
- **Changes vs upstream:** none — byte-for-byte copies of the two SKILL.md
  files. The upstream repo also ships commands, hooks (usage telemetry to a
  local CSV/webhook), plugin manifests for a dozen hosts, and four more skills
  (`ponytail-audit`, `ponytail-debt`, `ponytail-gain`, `ponytail-help`); none
  of that is vendored. The core skill is self-contained and host-agnostic;
  the `/ponytail …` slash-command references in its prose are inert outside
  the upstream plugin.

## skills/test-audit/

- **Upstream:** [openclaw/openclaw](https://github.com/openclaw/openclaw)
  (`.agents/skills/test-audit/`), MIT (© OpenClaw Foundation, see `LICENSE`)
- **Pinned upstream state:** `main` as of 2026-10-31 (sparse clone of
  `.agents/skills/test-audit`); the upstream repo does not tag releases
- **License:** MIT
- **Vendored:** 2026-10-31
- **Changes vs upstream** — removal of openclaw-project-specific coupling;
  rules themselves are untouched:
  - `SKILL.md`: dropped the link to openclaw's internal
    `docs/help/testing/writing-tests.md` (boundary-and-cost guide); replaced
    the `proof policy` link to the sibling `openclaw-testing` skill and its
    Vitest-checkout rule with a harness-agnostic "run owner + sibling suites,
    re-run checks the deleted tests fed" bullet; dropped the `autoreview`
    step (an openclaw-internal tool).
  - `references/audit.md`: dropped the `proof policy` link and Vitest wording;
    replaced openclaw's `scripts/check-changed.mjs` gate-classification step
    and the `pnpm deadcode:*` script names with harness-agnostic equivalents
    ("run the project's dead-code/unused-export checks"); "native maintainer
    workflow" → "the project's maintainer workflow"; `main` → "the default
    branch".
  - `CAMPAIGN.md`: de-identified the Telegram-plugin campaign examples (kept
    the numbers and the lessons, removed the product names); `it.each`
    (Vitest/Jest) → "parameterized case".
  - Not vendored: nothing — all three files carried over.

## skills/ponytail-audit/

- **Upstream:** [DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail)
  (`skills/ponytail-audit/SKILL.md`)
- **Pinned upstream version:** v5.3.3 (2026-10-29, commit 96608e0)
- **License:** MIT — see `LICENSE` (© DietrichGebert)
- **Vendored:** 2026-10-31
- **Changes vs upstream:** none — byte-for-byte copy.

## skills/pr-simplify/

- **Upstream:** [openclaw/openclaw](https://github.com/openclaw/openclaw)
  (`.agents/skills/pr-simplify/SKILL.md`), `main` as of 2026-10-31
- **License:** MIT — see `LICENSE` (© OpenClaw Foundation)
- **Vendored:** 2026-10-31
- **Changes vs upstream:** one line. The upstream "**Rules:**" bullet pointed
  at named sections of openclaw's root `AGENTS.md` ("Design priorities", "One
  owner, complete cutover", …); generalized to "the repo's own instruction
  files — root `AGENTS.md` (or equivalent: `CLAUDE.md`, `.cursorrules`), every
  scoped `AGENTS.md` on the changed paths, and the project's stated design
  priorities". Everything else is byte-for-byte.

## skills/technical-documentation/

- **Upstream:** [openclaw/openclaw](https://github.com/openclaw/openclaw)
  (`.agents/skills/technical-documentation/`), which itself carries it from
  [vincentkoc/dotskills](https://github.com/vincentkoc/dotskills);
  `main` as of 2026-10-31
- **License:** MIT — see `LICENSE` (© OpenClaw Foundation)
- **Vendored:** 2026-10-31
- **What is vendored:** `SKILL.md` plus `references/` (`principles.md`,
  `agent-and-contributing.md`, `build.md`, `review.md`, `tooling.md`).
- **What is not vendored:** `references/openclaw.md` (a clean, purely
  openclaw-specific docs overlay — the general skill already says to layer a
  repo's own overlay when it defines one), the `agents/` sub-agent prompt
  files, and `assets/icon.jpg`.
- **Changes vs upstream** — de-coupling only:
  - `SKILL.md`: "OpenClaw page-type/IA overlay" trigger generalized to
    "repository-specific overlays … when the target repo defines them";
    workflow step 6 no longer names `references/openclaw.md`; the four named
    sub-agent prompts (`agents/*.md`, not vendored) replaced by one prose
    paragraph describing the same four bounded concerns in ascending depth.
  - `references/principles.md`, `build.md`, `review.md`: every
    "For OpenClaw docs work …" bullet generalized the same way ("if the
    target repo defines a docs overlay, apply it"); the openclaw
    `docs/docs.json` example generalized to "the framework's nav config".
  - Attribution "Source:" links to openclaw files inside
    `agent-and-contributing.md` kept — they cite donors, not coupling.
