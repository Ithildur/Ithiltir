#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="build/frontend/dist"

usage() {
  cat <<'EOF'
Usage:
  scripts/build_frontend.sh [-o OUT_DIR]

Options:
  -o OUT_DIR   Frontend build output directory (default: build/frontend/dist)

Examples:
  scripts/build_frontend.sh
  scripts/build_frontend.sh -o build/frontend/dist
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      OUT_DIR="${2:-}"
      [[ -n "$OUT_DIR" ]] || { echo "missing value for -o" >&2; usage; exit 2; }
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage
      exit 2
      ;;
  esac
done

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
web_root="$repo_root/web"

if [[ ! -f "$web_root/package.json" ]]; then
  echo "frontend package.json not found: $web_root/package.json" >&2
  exit 1
fi

if ! command -v bun >/dev/null 2>&1; then
  echo "bun is required to build the frontend" >&2
  exit 1
fi

if [[ ! -f "$web_root/bun.lock" ]]; then
  echo "frontend lockfile not found: $web_root/bun.lock" >&2
  exit 1
fi

if [[ "$OUT_DIR" = /* ]]; then
  out_dir="$OUT_DIR"
else
  out_dir="$repo_root/$OUT_DIR"
fi

mkdir -p "$(dirname "$out_dir")"

(
  cd "$web_root"
  bun install --frozen-lockfile
  bun run build -- --outDir "$out_dir" --emptyOutDir
)

echo "frontend built: $out_dir"
