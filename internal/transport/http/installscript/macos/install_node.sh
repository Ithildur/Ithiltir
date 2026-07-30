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
APP_LANGUAGE="${APP_LANGUAGE:-__APP_LANGUAGE__}"

RUN_USER="${RUN_USER:-${SUDO_USER:-}}"

need_cmd() { command -v "$1" >/dev/null 2>&1; }

is_zh() {
  case "$APP_LANGUAGE" in
    [eE][nN]|[eE][nN][gG][lL][iI][sS][hH]) return 1 ;;
    *) return 0 ;;
  esac
}

msg() {
  local key="$1"
  shift || true
  if is_zh; then
    case "$key" in
      root_required) echo "此安装脚本需要 root 权限，且当前系统未安装 sudo。请使用 root 用户运行。" ;;
      unsupported_arch) echo "仅支持 arm64，当前 uname -m=$1" ;;
      missing_download_tool) echo "缺少下载工具：请安装 curl" ;;
      invalid_node_version) echo "下载的节点返回了非法版本号：$1" ;;
      unsafe_redirect) echo "拒绝不安全的节点下载重定向：$1" ;;
      redirect_limit) echo "节点下载重定向超过 5 次。" ;;
      enable_time_sync) echo "[+] 正在启用网络时间同步（非致命）" ;;
      time_sync_enabled) echo "[+] 网络时间同步已启用" ;;
      time_sync_failed) echo "[Warn] 无法自动启用网络时间同步；请手动检查日期与时间设置" ;;
      secret_required) echo "Secret 不能为空。" ;;
      done) echo "[OK] 完成：LaunchDaemon com.ithiltir.node 已启用" ;;
      status) echo "     状态：sudo launchctl print system/com.ithiltir.node" ;;
      logs) echo "     日志：tail -f /var/log/ithiltir-node.log /var/log/ithiltir-node.err" ;;
      *) echo "$key" ;;
    esac
    return
  fi

  case "$key" in
    root_required) echo "This installer requires root privileges, and sudo is not installed. Please run as root." ;;
    unsupported_arch) echo "Only arm64 is supported; current uname -m=$1" ;;
    missing_download_tool) echo "Missing download tool: please install curl" ;;
    invalid_node_version) echo "Downloaded node returned an invalid version: $1" ;;
    unsafe_redirect) echo "Refusing unsafe node download redirect: $1" ;;
    redirect_limit) echo "Node download exceeded 5 redirects." ;;
    enable_time_sync) echo "[+] enabling network time sync (non-fatal)" ;;
    time_sync_enabled) echo "[+] network time sync is enabled" ;;
    time_sync_failed) echo "[Warn] could not enable network time sync automatically; please check Date & Time settings manually" ;;
    secret_required) echo "Secret is required." ;;
    done) echo "[OK] Done: LaunchDaemon com.ithiltir.node is enabled" ;;
    status) echo "     Status: sudo launchctl print system/com.ithiltir.node" ;;
    logs) echo "     Logs:   tail -f /var/log/ithiltir-node.log /var/log/ithiltir-node.err" ;;
    *) echo "$key" ;;
  esac
}

as_root() {
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    "$@"
  else
    if need_cmd sudo; then sudo "$@"; else
      msg root_required >&2
      exit 1
    fi
  fi
}

usage() {
  if is_zh; then
    cat >&2 <<EOF
用法：sudo bash $0 <dash_ip> [dash_port] <secret> [interval_seconds] [--net iface1,iface2]

示例：
  sudo bash $0 10.0.0.2 8080 mysecret
  sudo bash $0 dash.example.com mysecret
  sudo bash $0 10.0.0.2 8080 'my secret with space' 3 --net en0,en1
EOF
  else
    cat >&2 <<EOF
Usage:  sudo bash $0 <dash_ip> [dash_port] <secret> [interval_seconds] [--net iface1,iface2]

Examples:
  sudo bash $0 10.0.0.2 8080 mysecret
  sudo bash $0 dash.example.com mysecret
  sudo bash $0 10.0.0.2 8080 'my secret with space' 3 --net en0,en1
EOF
  fi
  exit 1
}

detect_arch() {
  local m
  m="$(uname -m)"
  case "$m" in
    arm64|aarch64) echo "arm64" ;;
    *) msg unsupported_arch "$m" >&2; exit 1 ;;
  esac
}

valid_node_version() {
  local version="$1"
  ((${#version} <= 128)) &&
    [[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]]
}

http_url_parts() {
  local url="$1" scheme rest authority host port
  [[ "$url" != *$'\r'* && "$url" != *$'\n'* ]] || return 1
  if [[ "$url" =~ ^([Hh][Tt][Tt][Pp][Ss]?)://(.*)$ ]]; then
    scheme="$(printf '%s' "${BASH_REMATCH[1]}" | tr '[:upper:]' '[:lower:]')"
    rest="${BASH_REMATCH[2]}"
  else
    return 1
  fi
  authority="${rest%%[/?#]*}"
  [[ -n "$authority" && "$authority" != *"@"* ]] || return 1
  if [[ "$authority" =~ ^(\[[^]]+\])(:([0-9]+))?$ ]]; then
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[3]}"
  elif [[ "$authority" =~ ^([^:]+)(:([0-9]+))?$ ]]; then
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[3]}"
  else
    return 1
  fi
  host="$(printf '%s' "$host" | tr '[:upper:]' '[:lower:]')"
  [[ -n "$host" ]] || return 1
  if [[ -n "$port" ]]; then
    [[ ${#port} -le 5 ]] && ((10#$port >= 1 && 10#$port <= 65535)) || return 1
  elif [[ "$scheme" == "https" ]]; then
    port="443"
  else
    port="80"
  fi
  printf '%s|%s|%s|%s\n' "$scheme" "$host" "$port" "$authority"
}

download_redirect_allowed() {
  local original="$1" current="$2" next="$3" original_parts current_parts next_parts
  local original_host current_scheme current_port next_scheme next_host next_port
  original_parts="$(http_url_parts "$original")" || return 1
  current_parts="$(http_url_parts "$current")" || return 1
  next_parts="$(http_url_parts "$next")" || return 1
  IFS='|' read -r _ original_host _ _ <<<"$original_parts"
  IFS='|' read -r current_scheme _ current_port _ <<<"$current_parts"
  IFS='|' read -r next_scheme next_host next_port _ <<<"$next_parts"
  [[ "$next_host" == "$original_host" ]] || return 1
  if [[ "$next_scheme" == "$current_scheme" ]]; then
    [[ "$next_port" == "$current_port" ]]
    return
  fi
  [[ "$current_scheme" == "http" && "$next_scheme" == "https" ]]
}

download_with_curl() {
  local original="$1" current="$1" out="$2" secret="$3" meta status next redirects=0
  while true; do
    if ! meta="$(curl --proto "=http,https" --tlsv1.2 -f --retry 3 --connect-timeout 10 --max-time 300 \
      -H "X-Node-Secret: ${secret}" -o "$out" -w $'%{http_code}\n%{redirect_url}' "$current")"; then
      return 1
    fi
    status="${meta%%$'\n'*}"
    next="${meta#*$'\n'}"
    [[ "$status" =~ ^2[0-9][0-9]$ ]] && return 0
    case "$status" in
      301|302|303|307|308) ;;
      *) return 1 ;;
    esac
    if ((redirects >= 5)); then
      msg redirect_limit >&2
      return 1
    fi
    if ! download_redirect_allowed "$original" "$current" "$next"; then
      msg unsafe_redirect "$next" >&2
      return 1
    fi
    current="$next"
    redirects=$((redirects + 1))
  done
}

download_file() {
  local url="$1" out="$2" secret="$3"
  if need_cmd curl; then
    download_with_curl "$url" "$out" "$secret"
  else
    msg missing_download_tool >&2
    exit 1
  fi
}

enable_time_sync() {
  msg enable_time_sync

  if need_cmd systemsetup && as_root systemsetup -setusingnetworktime on >/dev/null 2>&1; then
    msg time_sync_enabled
    return 0
  fi

  msg time_sync_failed >&2
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
    case "$DOWNLOAD_SCHEME" in
      [hH][tT][tT][pP][sS]) dash_port="443" ;;
      [hH][tT][tT][pP]) dash_port="80" ;;
      *) dash_port="80" ;;
    esac
    secret="$2"
    shift 2
  else
    dash_port="$2"
    secret="$3"
    shift 3
  fi
  if [[ -z "$secret" ]]; then
    msg secret_required >&2
    exit 1
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
  download_file "${url}" "${tmp}" "${secret}"
  chmod +x "${tmp}"
  node_version="$("${tmp}" --version)"
  node_version="${node_version//$'\r'/}"
  valid_node_version "$node_version" || { msg invalid_node_version "$node_version" >&2; exit 1; }
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

  msg done
  msg status
  msg logs
}

main "$@"
