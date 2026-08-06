#!/usr/bin/env bash
# check-version-consistency.sh — fail if the skill version is not identical in
# the three places it is recorded: plugin.json, SKILL.md frontmatter, README.md.
#
# Exit codes:
#   0  all three versions agree
#   1  mismatch — fix the version in the file(s) shown
#   2  could not read one of the files (missing / bad frontmatter)
#
# Requires: jq. Compatible with bash 3.2 (macOS).
# Run from anywhere; resolves paths relative to this script.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Repo root is three levels up: scripts/ -> go-ultimate/ -> skills/ -> root.
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

PLUGIN_JSON="${REPO_ROOT}/.claude-plugin/plugin.json"
SKILL_MD="${REPO_ROOT}/skills/go-ultimate/SKILL.md"
README_MD="${REPO_ROOT}/README.md"

if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required (brew install jq)." >&2
    exit 2
fi

# plugin.json version.
if [[ ! -f "$PLUGIN_JSON" ]]; then
    echo "ERROR: $PLUGIN_JSON not found" >&2
    exit 2
fi
v_plugin=$(jq -r '.version' "$PLUGIN_JSON")

# SKILL.md frontmatter version (YAML `version: 'x.y.z'`).
if [[ ! -f "$SKILL_MD" ]]; then
    echo "ERROR: $SKILL_MD not found" >&2
    exit 2
fi
v_skill=$(awk '/^---/{c++; next} c==1 && /^[[:space:]]*version:/{sub(/^[[:space:]]*version:[[:space:]]*/,""); print; exit}' "$SKILL_MD" | tr -d "'\"")
if [[ -z "$v_skill" ]]; then
    echo "ERROR: could not read version from SKILL.md frontmatter" >&2
    exit 2
fi

# README.md Status line: "> **Status:** vX.Y.Z. ..."
if [[ ! -f "$README_MD" ]]; then
    echo "ERROR: $README_MD not found" >&2
    exit 2
fi
v_readme=$(awk '/Status:/ && match($0, /v[0-9]+\.[0-9]+\.[0-9]+/) {print substr($0, RSTART+1, RLENGTH-1); exit}' "$README_MD")
if [[ -z "$v_readme" ]]; then
    echo "ERROR: could not read version from README.md Status line" >&2
    exit 2
fi

printf "%-18s %s\n" "plugin.json:" "$v_plugin"
printf "%-18s %s\n" "SKILL.md:" "$v_skill"
printf "%-18s %s\n" "README.md:" "$v_readme"

if [[ "$v_plugin" == "$v_skill" && "$v_plugin" == "$v_readme" ]]; then
    echo "RESULT: versions agree ($v_plugin)."
    exit 0
fi

echo "RESULT: version mismatch — all three must match on release." >&2
exit 1
