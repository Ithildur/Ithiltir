#!/usr/bin/env bash
set -euo pipefail

APP="ithiltir-node"

INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/ithiltir-node"
RELEASES_DIR="${DATA_DIR}/releases"
CURRENT_DIR="${DATA_DIR}/current"
BIN_PATH="${CURRENT_DIR}/${APP}"
PLIST_PATH="/Library/LaunchDaemons/com.ithiltir.node.plist"

DOWNLOAD_SCHEME="${DOWNLOAD_SCHEME:-__DOWNLOAD_SCHEME__}"
DOWNLOAD_HOST="${DOWNLOAD_HOST:-__DOWNLOAD_HOST__}"
DOWNLOAD_PATH="${DOWNLOAD_PATH:-__DOWNLOAD_PATH__}"
DOWNLOAD_PREFIX="${DOWNLOAD_PREFIX:-node_macos_}"

RUN_USER="${RUN_USER:-${SUDO_USER:-}}"

need_cmd() { command -v "$1" >/dev/null 2>&1; }

as_root() {
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    "$@"
  else
    if need_cmd sudo; then sudo "$@"; else
      echo "This installer requires root privileges, and sudo is not installed. Please run as root." >&2
      exit 1
    fi
  fi
}

usage() {
  cat >&2 <<EOF
Usage:  sudo bash $0 <dash_ip> [dash_port] <secret> [interval_seconds] [--net iface1,iface2]

Examples:
  sudo bash $0 10.0.0.2 8080 mysecret
  sudo bash $0 dash.example.com mysecret
  sudo bash $0 10.0.0.2 8080 'my secret with space' 3 --net en0,en1
EOF
  exit 1
}

detect_arch() {
  local m
  m="$(uname -m)"
  case "$m" in
    arm64|aarch64) echo "arm64" ;;
    *) echo "Only arm64 is supported; current uname -m=$m" >&2; exit 1 ;;
  esac
}

download_file() {
  local url="$1" out="$2"
  if need_cmd curl; then
    curl -fL --retry 3 --connect-timeout 10 --max-time 300 -o "$out" "$url"
  else
    echo "Missing download tool: please install curl" >&2
    exit 1
  fi
}

enable_time_sync() {
  echo "[+] enabling network time sync (non-fatal)"

  if need_cmd systemsetup && as_root systemsetup -setusingnetworktime on >/dev/null 2>&1; then
    echo "[+] network time sync is enabled"
    return 0
  fi

  echo "[!] could not enable network time sync automatically; please check Date & Time settings manually" >&2
  return 0
}

url_host() {
  local host="$1"
  if [[ "$host" == \[*\] ]]; then
    echo "$host"
    return
  fi
  if [[ "$host" == *:* ]]; then
    echo "[${host}]"
    return
  fi
  echo "$host"
}

report_url() {
  local dash_ip="$1"
  local dash_port="$2"
  printf "%s://%s:%s/api/node/metrics" "${DOWNLOAD_SCHEME}" "$(url_host "${dash_ip}")" "${dash_port}"
}

configure_report() {
  local url="$1"
  local secret="$2"
  shift 2 || true
  cd "${DATA_DIR}"
  as_root "${BIN_PATH}" report install "$url" "$secret" "$@"
}

write_plist() {
  local interval="${1:-}"
  shift 1 || true

  local -a program_args
  program_args=( "${BIN_PATH}" "push" )
  if [[ -n "${interval}" ]]; then
    program_args+=( "${interval}" )
  fi
  if [[ $# -gt 0 ]]; then
    program_args+=( "$@" )
  fi

  local args_xml=""
  local a
  for a in "${program_args[@]}"; do
    a="${a//&/&amp;}"
    a="${a//</&lt;}"
    a="${a//>/&gt;}"
    a="${a//\"/&quot;}"
    a="${a//\'/&apos;}"
    args_xml+="    <string>${a}</string>
"
  done

  local user_xml=""
  if [[ -n "${RUN_USER}" && "${RUN_USER}" != "root" ]]; then
    local escaped_user="${RUN_USER//&/&amp;}"
    escaped_user="${escaped_user//</&lt;}"
    escaped_user="${escaped_user//>/&gt;}"
    escaped_user="${escaped_user//\"/&quot;}"
    escaped_user="${escaped_user//\'/&apos;}"
    user_xml="  <key>UserName</key>
  <string>${escaped_user}</string>
"
  fi

  local tmp
  tmp="$(mktemp)"
  cat > "${tmp}" <<EOF
<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">
<plist version=\"1.0\">
<dict>
  <key>Label</key>
  <string>com.ithiltir.node</string>

  <key>ProgramArguments</key>
  <array>
${args_xml}  </array>

${user_xml}  <key>RunAtLoad</key>
  <true/>

  <key>KeepAlive</key>
  <true/>

  <key>WorkingDirectory</key>
  <string>${DATA_DIR}</string>

  <key>StandardOutPath</key>
  <string>/var/log/ithiltir-node.log</string>
  <key>StandardErrorPath</key>
  <string>/var/log/ithiltir-node.err</string>
</dict>
</plist>
EOF
  as_root install -m 0644 "${tmp}" "${PLIST_PATH}"
  rm -f "${tmp}"
}

restart_service() {
  as_root launchctl bootout system "${PLIST_PATH}" >/dev/null 2>&1 || true
  as_root launchctl bootstrap system "${PLIST_PATH}"
  as_root launchctl enable system/com.ithiltir.node >/dev/null 2>&1 || true
  as_root launchctl kickstart -k system/com.ithiltir.node >/dev/null 2>&1 || true
}

main() {
  if [[ $# -lt 2 ]]; then
    usage
  fi

  local dash_ip="$1"
  local dash_port=""
  local secret=""
  if [[ $# -eq 2 ]]; then
    case "${DOWNLOAD_SCHEME,,}" in
      https) dash_port="443" ;;
      http) dash_port="80" ;;
      *) dash_port="80" ;;
    esac
    secret="$2"
    shift 2
  else
    dash_port="$2"
    secret="$3"
    shift 3
  fi

  local interval=""
  if [[ $# -gt 0 && "$1" =~ ^[0-9]+$ ]]; then
    interval="$1"
    shift 1
  fi

  local require_https=0
  local arg
  for arg in "$@"; do
    if [[ "$arg" == "--require-https" ]]; then
      require_https=1
    fi
  done

  local arch url tmp node_version release_dir
  arch="$(detect_arch)"
  url="${DOWNLOAD_SCHEME}://${DOWNLOAD_HOST}${DOWNLOAD_PATH}/${DOWNLOAD_PREFIX}${arch}"

  echo "[+] arch=${arch}"
  echo "[+] url=${url}"
  echo "[+] install=${INSTALL_DIR}"
  echo "[+] mode=push dash_ip=${dash_ip} dash_port=${dash_port} interval=${interval:-default}"

  enable_time_sync

  as_root mkdir -p "${INSTALL_DIR}"
  as_root mkdir -p "${RELEASES_DIR}"
  as_root mkdir -p "${DATA_DIR}"

  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT
  download_file "${url}" "${tmp}"
  chmod +x "${tmp}"
  node_version="$("${tmp}" --version | head -n1 | tr -d '\r')"
  release_dir="${RELEASES_DIR}/${node_version}"
  as_root mkdir -p "${release_dir}"
  as_root install -m 0755 "${tmp}" "${release_dir}/${APP}"
  as_root ln -sfn "${release_dir}" "${CURRENT_DIR}"
  if [[ "${require_https}" -eq 1 ]]; then
    configure_report "$(report_url "${dash_ip}" "${dash_port}")" "${secret}" --require-https
  else
    configure_report "$(report_url "${dash_ip}" "${dash_port}")" "${secret}"
  fi
  if [[ -n "${RUN_USER}" && "${RUN_USER}" != "root" ]]; then
    as_root chown -R "${RUN_USER}" "${DATA_DIR}"
    as_root chown -h "${RUN_USER}" "${CURRENT_DIR}" >/dev/null 2>&1 || true
  fi

  if [[ $# -gt 0 ]]; then
    write_plist "${interval}" "$@"
  else
    write_plist "${interval}"
  fi
  restart_service

  echo "[OK] Done: LaunchDaemon com.ithiltir.node is enabled"
  echo "     Status: sudo launchctl print system/com.ithiltir.node"
  echo "     Logs:   tail -f /var/log/ithiltir-node.log /var/log/ithiltir-node.err"
}

main "$@"
