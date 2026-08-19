# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| 0.11.x | yes |
| < 0.11 | no |

Only the latest minor release receives fixes. Upgrade before reporting against
an older version.

## Reporting a vulnerability

Report privately through GitHub Security Advisories: open the repository's
**Security** tab and choose **Report a vulnerability**. Do not open a public
issue for a suspected vulnerability.

Expect an acknowledgement within 7 days and a status update within 30 days. If
a report is accepted, the fix ships in a patch release and the advisory is
published with credit unless you ask otherwise.

## Scope

go-ultimate ships markdown skill content under `skills/go-ultimate/`, Go
template projects under `skills/go-ultimate/assets/`, and helper scripts under
`skills/go-ultimate/scripts/`. The skill registers no hooks, requires no
secrets, and makes no network calls when invoked.

In scope:

- content in `SKILL.md` or `references/` that could steer a host agent into
  unsafe actions;
- vulnerable or malicious code in the `assets/` templates a user would copy;
- unsafe behavior in `scripts/` (`goversion`, `check-source-versions.sh`,
  `check-version-consistency.sh`).

Out of scope: vulnerabilities in the host agent (Claude Code, Codex, Cursor,
Grok Build, GitHub Copilot CLI, OpenCode), in Go itself, or in third-party
modules the templates depend on — report those upstream.
