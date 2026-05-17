#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<EOF
Usage:
  scripts/release_notes.sh [--default-base] TAG OUT_FILE
EOF
  exit 2
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DEFAULT_BASE="false"
if [[ "${1:-}" == "--default-base" ]]; then
  DEFAULT_BASE="true"
  shift
fi

tag="${1:-}"
out="${2:-}"
[[ -n "$tag" && -n "$out" ]] || usage

version_tool() {
  go -C "$repo_root" run ./cmd/releasever "$@"
}

previous_release_tag() {
  local candidate channel cmp
  local -a releases=()

  while IFS= read -r candidate; do
    [[ -n "$candidate" && "$candidate" != "$tag" ]] || continue
    channel="$(version_tool channel "$candidate" 2>/dev/null || true)"
    [[ "$channel" == "release" ]] || continue
    cmp="$(version_tool compare "$candidate" "$tag" 2>/dev/null || true)"
    [[ "$cmp" == "-1" ]] || continue
    releases+=("$candidate")
  done < <(git -C "$repo_root" tag --merged "$tag" | sed '/^$/d')

  if ((${#releases[@]} == 0)); then
    return 0
  fi

  printf '%s\n' "${releases[@]}" | version_tool latest release
}

previous_tag() {
  local candidate cmp
  local -a tags=()

  while IFS= read -r candidate; do
    [[ -n "$candidate" && "$candidate" != "$tag" ]] || continue
    version_tool validate "$candidate" >/dev/null 2>&1 || continue
    cmp="$(version_tool compare "$candidate" "$tag" 2>/dev/null || true)"
    [[ "$cmp" == "-1" ]] || continue
    tags+=("$candidate")
  done < <(git -C "$repo_root" tag --merged "$tag" | sed '/^$/d')

  if ((${#tags[@]} == 0)); then
    return 0
  fi

  printf '%s\n' "${tags[@]}" | version_tool latest prerelease
}

write_commits() {
  local previous="$1"
  local range="$tag"
  if [[ -n "$previous" ]]; then
    range="${previous}..${tag}"
  fi

  git -C "$repo_root" log --reverse --abbrev=7 --format='· %h %s' "$range"
}

version_tool validate "$tag" >/dev/null
previous=""
if [[ "$DEFAULT_BASE" == "true" ]]; then
  previous="$(previous_tag || true)"
else
  previous="$(previous_release_tag || true)"
fi

{
  printf '## Changelog\n\n'
  write_commits "$previous"
} > "$out"

if [[ "$DEFAULT_BASE" == "true" ]]; then
  if [[ -n "$previous" ]]; then
    echo "release notes base: $previous" >&2
  else
    echo "release notes base: first release" >&2
  fi
elif [[ -n "$previous" ]]; then
  echo "release notes base: $previous" >&2
else
  echo "release notes base: first release" >&2
fi
