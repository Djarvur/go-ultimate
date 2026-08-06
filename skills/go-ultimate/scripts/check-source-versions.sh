#!/usr/bin/env bash
# check-source-versions.sh — detect upstream drift in go-ultimate's donor sources.
#
# Reads baseline pins from source-versions.json and compares them against the
# live upstream state for every source:
#   github-path    — last commit SHA touching the path (robust whether or not the
#                    upstream tags versions; this is the only signal that works
#                    for donors with no frontmatter version)
#   github-tag     — latest stable release tag (falls back to latest tag if no
#                    release is published)
#   manual-review  — not auto-checked; the pin records only the last manual
#                    review date (full-body HTTP hashes were too noisy to be useful)
#
# Exit codes:
#   0  no drift detected
#   1  drift detected (an upstream moved since the pin was recorded)
#   2  could not fetch one or more upstreams (network / 404 / tooling)
#
# Requires: jq, gh (authenticated). Compatible with bash 3.2 (macOS).
# Run from anywhere; resolves paths relative to this script.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PIN_FILE="${SCRIPT_DIR}/source-versions.json"

if [[ ! -f "$PIN_FILE" ]]; then
    echo "ERROR: pin file not found at $PIN_FILE" >&2
    exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required (brew install jq)." >&2
    exit 2
fi
if ! command -v gh >/dev/null 2>&1; then
    echo "ERROR: gh is required (brew install gh)." >&2
    exit 2
fi

# Fetch the latest commit SHA on a repo's default branch that touches a path.
# Prints the 12-char abbreviated SHA, or "FETCH_ERROR".
github_path_sha() {
    local repo="$1" path="$2"
    local sha
    # `// empty` matters: on an empty array (path deleted, repo renamed, file
    # moved) jq would otherwise print the string "null", which passes the -z
    # test below and gets reported as DRIFT instead of FETCH ERROR.
    sha=$(gh api "repos/${repo}/commits?path=${path}&per_page=1" \
            --jq '.[0].sha // empty' 2>/dev/null || true)
    if [[ -z "$sha" ]]; then
        echo "FETCH_ERROR"
    else
        echo "${sha:0:12}"
    fi
}

# Fetch the latest stable release tag for a repo. Falls back to latest tag.
# Prints the tag name, or "FETCH_ERROR".
github_latest_tag() {
    local repo="$1" tag
    # Prefer latest published release.
    tag=$(gh api "repos/${repo}/releases/latest" --jq '.tag_name // empty' 2>/dev/null || true)
    if [[ -z "$tag" ]]; then
        # Fall back to most recently pushed tag.
        tag=$(gh api "repos/${repo}/tags?per_page=1" --jq '.[0].name // empty' 2>/dev/null || true)
    fi
    if [[ -z "$tag" ]]; then
        echo "FETCH_ERROR"
    else
        echo "$tag"
    fi
}

drift_found=0
fetch_error=0

printf "%-40s  %-26s  %-26s  %s\n" "SOURCE" "PINNED" "UPSTREAM" "STATUS"
printf "%-40s  %-26s  %-26s  %s\n" "------" "------" "--------" "------"

# Iterate sources in stable order. (No mapfile — bash 3.2 / macOS default.)
KEYS=()
while IFS= read -r line; do
    KEYS+=("$line")
done < <(jq -r '.sources | keys | .[]' "$PIN_FILE" | sort)

for key in "${KEYS[@]}"; do
    kind=$(jq -r --arg k "$key" '.sources[$k].kind' "$PIN_FILE")
    docref=$(jq -r --arg k "$key" '.sources[$k].doc_ref // "—"' "$PIN_FILE")

    pinned=""
    upstream=""
    status=""
    note=""

    case "$kind" in
        github-path)
            repo=$(jq -r --arg k "$key" '.sources[$k].repo' "$PIN_FILE")
            path=$(jq -r --arg k "$key" '.sources[$k].path' "$PIN_FILE")
            pinned=$(jq -r --arg k "$key" '.sources[$k].commit_sha' "$PIN_FILE")
            upstream=$(github_path_sha "$repo" "$path")
            if [[ "$upstream" == "FETCH_ERROR" ]]; then
                status="FETCH ERROR ($repo)"; fetch_error=1
            elif [[ "$pinned" == "$upstream" ]]; then
                status="ok"
            else
                status="DRIFT — refresh $docref"; drift_found=1
            fi
            pinned="sha:${pinned:0:12}"
            upstream="sha:${upstream:0:12}"
            ;;
        github-tag)
            repo=$(jq -r --arg k "$key" '.sources[$k].repo' "$PIN_FILE")
            pinned=$(jq -r --arg k "$key" '.sources[$k].tag' "$PIN_FILE")
            upstream=$(github_latest_tag "$repo")
            if [[ "$upstream" == "FETCH_ERROR" ]]; then
                status="FETCH ERROR ($repo)"; fetch_error=1
            elif [[ "$pinned" == "$upstream" ]]; then
                status="ok"
            else
                status="DRIFT — eyeball $repo releases, refresh $docref"; drift_found=1
            fi
            pinned="tag:${pinned}"
            upstream="tag:${upstream}"
            ;;
        manual-review)
            # Not auto-checked. Full-body HTTP hashes are permanently noisy
            # (ads, dynamic chrome) and trained people to ignore the report.
            # These sources are reviewed by hand on a schedule; the pin records
            # only the date of the last review, surfaced here as a reminder.
            mr_last=$(jq -r --arg k "$key" '.sources[$k].last_reviewed // "?"' "$PIN_FILE")
            mr_docref=$(jq -r --arg k "$key" '.sources[$k].doc_ref // "—"' "$PIN_FILE")
            pinned="reviewed:${mr_last}"
            upstream="—"
            status="MANUAL REVIEW (last ${mr_last}) — eyeball ${mr_docref}"
            ;;
        *)
            status="UNKNOWN KIND: $kind"; fetch_error=1
            ;;
    esac

    printf "%-40s  %-26s  %-26s  %s\n" "$key" "$pinned" "$upstream" "$status"
done

echo ""
if [[ $drift_found -eq 1 ]]; then
    echo "RESULT: drift detected. Investigate the flagged sources; if a change is"
    echo "        worth folding in, update the skill and refresh the pin in"
    echo "        source-versions.json (and bump SKILL.md version)."
    exit 1
elif [[ $fetch_error -eq 1 ]]; then
    echo "RESULT: could not fetch one or more upstreams. Check network/auth/URLs."
    exit 2
else
    echo "RESULT: no drift detected."
    exit 0
fi
