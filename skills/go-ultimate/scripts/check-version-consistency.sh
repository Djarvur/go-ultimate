#!/usr/bin/env bash
# check-version-consistency.sh — fail if the skill version is not identical
# everywhere it is recorded: SKILL.md frontmatter, README.md Status line, and
# every plugin/marketplace manifest across the supported harnesses.
#
# Manifests scanned (relative to repo root):
#   .claude-plugin/plugin.json          .codex-plugin/plugin.json
#   .cursor-plugin/plugin.json          .cursor-plugin/marketplace.json
#   .grok-plugin/plugin.json            .grok-plugin/marketplace.json
#   .plugin/plugin.json
#   .agents/plugins/marketplace.json    .github/plugin/marketplace.json
#
# Exit codes:
#   0  all versions agree
#   1  mismatch — fix the version in the file(s) shown
#   2  could not read a required file (missing / bad frontmatter)
#
# Requires: jq. Compatible with bash 3.2 (macOS).
# Run from anywhere; resolves paths relative to this script.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Repo root is three levels up: scripts/ -> go-ultimate/ -> skills/ -> root.
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

SKILL_MD="${REPO_ROOT}/skills/go-ultimate/SKILL.md"
README_MD="${REPO_ROOT}/README.md"

if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required (brew install jq)." >&2
    exit 2
fi

# Collect every version string found, with its source, then assert they all match.
declare -a seen=()      # "version|source" rows
mismatch=0
canonical=""

record() {
    local val="$1" src="$2"
    if [[ -z "$val" ]]; then
        echo "ERROR: could not read version from $src" >&2
        exit 2
    fi
    seen+=("$val|$src")
    if [[ -z "$canonical" ]]; then
        canonical="$val"
    elif [[ "$val" != "$canonical" ]]; then
        mismatch=1
    fi
}

# SKILL.md frontmatter version (YAML `version: 'x.y.z'`).
if [[ ! -f "$SKILL_MD" ]]; then
    echo "ERROR: $SKILL_MD not found" >&2; exit 2
fi
v_skill=$(awk '/^---/{c++; next} c==1 && /^[[:space:]]*version:/{sub(/^[[:space:]]*version:[[:space:]]*/,""); print; exit}' "$SKILL_MD" | tr -d "'\"")
record "$v_skill" "SKILL.md"

# README.md Status line: "> **Status:** vX.Y.Z. ..."
if [[ ! -f "$README_MD" ]]; then
    echo "ERROR: $README_MD not found" >&2; exit 2
fi
v_readme=$(awk '/Status:/ && match($0, /v[0-9]+\.[0-9]+\.[0-9]+/) {print substr($0, RSTART+1, RLENGTH-1); exit}' "$README_MD")
record "$v_readme" "README.md"

# Every JSON manifest with a top-level .version (plugin manifests) or a nested
# marketplace .plugins[].version. Scan the known harness manifest dirs/files.
while IFS= read -r mf; do
    [[ -f "$mf" ]] || continue
    rel="${mf#$REPO_ROOT/}"
    # Top-level version (plugin.json shape).
    v=$(jq -r '.version // empty' "$mf" 2>/dev/null)
    [[ -n "$v" ]] && record "$v" "$rel"
    # Nested plugin entries (marketplace.json shape).
    while IFS= read -r pv; do
        [[ -n "$pv" ]] && record "$pv" "$rel (plugins[])"
    done < <(jq -r '.plugins[].version // empty' "$mf" 2>/dev/null)
    # Nested metadata.version (some marketplace shapes).
    mv=$(jq -r '.metadata.version // empty' "$mf" 2>/dev/null)
    [[ -n "$mv" ]] && record "$mv" "$rel (metadata)"
done < <(find "$REPO_ROOT" -maxdepth 3 \( -name plugin.json -o -name marketplace.json \) \
    -not -path "*/node_modules/*" -not -path "*/.git/*" 2>/dev/null)

for row in "${seen[@]}"; do
    val="${row%%|*}"
    src="${row#*|}"
    printf "%-12s  %s\n" "$val" "$src"
done

if [[ $mismatch -eq 0 ]]; then
    echo "RESULT: versions agree ($canonical) across ${#seen[@]} locations."
    exit 0
fi

echo "RESULT: version mismatch — every location must match on release." >&2
exit 1

