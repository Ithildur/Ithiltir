#!/usr/bin/env bash
set -euo pipefail

# Compatibility entrypoint. All update logic lives in the Dash subcommand.
INSTALL_DIR="${DASH_HOME:-/opt/Ithiltir-dash}"
BIN="${INSTALL_DIR}/bin/dash"

if [[ ! -x "$BIN" ]]; then
  echo "error: Dash binary is missing or not executable: $BIN" >&2
  exit 1
fi

needs_root="true"
for arg in "$@"; do
  case "$arg" in
    --check|check|-h|--help|--version|-v|version)
      needs_root="false"
      ;;
  esac
done

if [[ "$needs_root" == "true" && "${EUID:-$(id -u)}" -ne 0 ]]; then
  if ! command -v sudo >/dev/null 2>&1; then
    echo "error: root privileges are required and sudo is not installed" >&2
    exit 1
  fi
  sudo_env=(env "DASH_HOME=$INSTALL_DIR" "SCRIPT_LANG=${SCRIPT_LANG:-}")
  for proxy_name in HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy; do
    if [[ -v "$proxy_name" ]]; then
      sudo_env+=("$proxy_name=${!proxy_name}")
    fi
  done
  exec sudo "${sudo_env[@]}" "$BIN" update "$@"
fi

export DASH_HOME="$INSTALL_DIR"
exec "$BIN" update "$@"
