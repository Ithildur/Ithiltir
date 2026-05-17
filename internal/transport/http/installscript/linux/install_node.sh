#!/usr/bin/env bash
set -euo pipefail

APP="ithiltir-node"

INSTALL_DIR="/opt/node"
DATA_DIR="/var/lib/ithiltir-node"
RELEASES_DIR="${DATA_DIR}/releases"
CURRENT_DIR="${DATA_DIR}/current"
BIN_PATH="${CURRENT_DIR}/${APP}"

DOWNLOAD_SCHEME="${DOWNLOAD_SCHEME:-__DOWNLOAD_SCHEME__}"
DOWNLOAD_HOST="${DOWNLOAD_HOST:-__DOWNLOAD_HOST__}"
DOWNLOAD_PATH="${DOWNLOAD_PATH:-__DOWNLOAD_PATH__}"
DOWNLOAD_PREFIX="${DOWNLOAD_PREFIX:-node_linux_}"

RUN_USER="${RUN_USER:-ithiltir}"
RUN_GROUP="${RUN_GROUP:-ithiltir}"

SERVICE_FILE="/etc/systemd/system/${APP}.service"

TMPFILES_FILE="/etc/tmpfiles.d/ithiltir-node.conf"
CACHE_DIR="/run/ithiltir-node"
CACHE_FILE="${CACHE_DIR}/thinpool.json"
SMART_CACHE_FILE="${CACHE_DIR}/smart.json"
SMART_HELPER_FILE="/usr/local/libexec/ithiltir-node/smart-cache"
SMART_SERVICE_NAME="ithiltir-node-smart-cache.service"
SMART_TIMER_NAME="ithiltir-node-smart-cache.timer"
SMART_SERVICE_FILE="/etc/systemd/system/${SMART_SERVICE_NAME}"
SMART_TIMER_FILE="/etc/systemd/system/${SMART_TIMER_NAME}"

COLLECTOR="${INSTALL_DIR}/collect_thinpool.sh"
CRON_FILE="/etc/cron.d/ithiltir-node-thinpool"

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
  sudo bash $0 10.0.0.2 8080 'my secret with space' 3 --net eth0,eth1
EOF
  exit 1
}

detect_arch() {
  local m
  m="$(uname -m)"
  case "$m" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "Only amd64/arm64 are supported; current uname -m=$m" >&2; exit 1 ;;
  esac
}

download_file() {
  local url="$1" out="$2"
  if need_cmd curl; then
    curl -fL --retry 3 --connect-timeout 10 --max-time 300 -o "$out" "$url"
  elif need_cmd wget; then
    wget -O "$out" "$url"
  else
    echo "Missing download tool: please install curl or wget" >&2
    exit 1
  fi
}

ensure_user() {
  if id -u "${RUN_USER}" >/dev/null 2>&1; then
    return 0
  fi
  if need_cmd useradd; then
    as_root useradd --system --no-create-home --shell /usr/sbin/nologin --user-group "${RUN_USER}"
  elif need_cmd adduser; then
    as_root adduser --system --no-create-home --disabled-login --group "${RUN_USER}"
  else
    echo "Missing useradd/adduser; cannot create user ${RUN_USER}" >&2
    exit 1
  fi
}

enable_time_sync() {
  echo "[+] enabling system time sync (NTP; non-fatal)"

  if need_cmd timedatectl && as_root timedatectl set-ntp true >/dev/null 2>&1; then
    echo "[+] system time sync is enabled"
    return 0
  fi

  if need_cmd systemctl; then
    local unit
    for unit in systemd-timesyncd.service chronyd.service ntpd.service ntp.service; do
      if as_root systemctl enable --now "$unit" >/dev/null 2>&1; then
        echo "[+] system time sync service started: ${unit}"
        return 0
      fi
    done
  fi

  echo "[!] could not enable system time sync automatically; please check NTP/chrony/systemd-timesyncd manually" >&2
  return 0
}

has_lvm() {
  if need_cmd lsblk; then
    if lsblk -nr -o NAME 2>/dev/null | grep -Eq '(-tpool|_tmeta|_tdata)$'; then
      return 0
    fi
    if lsblk -nr -o TYPE 2>/dev/null | grep -qiE '^lvm$|lvm'; then
      return 0
    fi
  fi
  return 1
}

install_cron() {
  local svc=""
  if need_cmd apt-get; then
    as_root apt-get update -y
    as_root apt-get install -y cron
    svc="cron"
  elif need_cmd dnf; then
    as_root dnf install -y cronie
    svc="crond"
  elif need_cmd yum; then
    as_root yum install -y cronie
    svc="crond"
  elif need_cmd pacman; then
    as_root pacman -Sy --noconfirm cronie
    svc="cronie"
  elif need_cmd apk; then
    as_root apk add --no-cache dcron
    svc="dcron"
  else
    echo "No supported package manager found (apt/dnf/yum/pacman/apk); cannot auto-install cron." >&2
    return 1
  fi

  if [[ -n "$svc" ]]; then
    as_root systemctl enable --now "$svc" >/dev/null 2>&1 || true
    as_root systemctl enable --now crond >/dev/null 2>&1 || true
    as_root systemctl enable --now cron  >/dev/null 2>&1 || true
  fi
}

install_smartmontools() {
  if need_cmd smartctl; then
    echo "[+] smartctl already installed"
    return 0
  fi

  echo "[+] installing smartmontools for SMART cache (non-fatal)"
  if need_cmd apt-get; then
    as_root apt-get update || { echo "[!] apt-get update failed; smartctl not installed" >&2; return 0; }
    as_root apt-get install -y smartmontools || echo "[!] smartmontools install failed; node install continues" >&2
  elif need_cmd dnf; then
    as_root dnf install -y smartmontools || echo "[!] smartmontools install failed; node install continues" >&2
  elif need_cmd yum; then
    as_root yum install -y smartmontools || echo "[!] smartmontools install failed; node install continues" >&2
  elif need_cmd pacman; then
    as_root pacman -Sy --noconfirm smartmontools || echo "[!] smartmontools install failed; node install continues" >&2
  elif need_cmd zypper; then
    as_root zypper --non-interactive install smartmontools || echo "[!] smartmontools install failed; node install continues" >&2
  elif need_cmd apk; then
    as_root apk add --no-cache smartmontools || echo "[!] smartmontools install failed; node install continues" >&2
  else
    echo "[!] unsupported package manager; smartctl not installed" >&2
  fi
}

write_tmpfiles() {
  as_root bash -c "cat > '${TMPFILES_FILE}' <<'EOF'
d /run/ithiltir-node 0750 root ${RUN_GROUP} -
EOF"
  as_root systemd-tmpfiles --create >/dev/null 2>&1 || true
}

systemd_quote_arg() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  printf "\"%s\"" "$s"
}

build_execstart_line() {
  local -a args=("$@")
  local out=""
  local a
  for a in "${args[@]}"; do
    out+=$(systemd_quote_arg "$a")
    out+=" "
  done
  echo "${out% }"
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

write_service_push() {
  local interval="${1:-}"
  shift 1 || true

  local -a exec_args
  exec_args=( "${BIN_PATH}" "push" )
  if [[ -n "${interval}" ]]; then
    exec_args+=( "${interval}" )
  fi
  if [[ $# -gt 0 ]]; then
    exec_args+=( "$@" )
  fi

  local exec_line
  exec_line="$(build_execstart_line "${exec_args[@]}")"

  as_root bash -c "cat > '${SERVICE_FILE}' <<EOF
[Unit]
Description=Ithiltir Node (system metrics agent)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${RUN_USER}
Group=${RUN_GROUP}

ExecStart=${exec_line}

Restart=always
RestartSec=2

WorkingDirectory=${DATA_DIR}

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR}

[Install]
WantedBy=multi-user.target
EOF"
}

write_collector() {
  local tmp
  tmp="$(mktemp)"
  cat > "$tmp" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${OUT_DIR:-/run/ithiltir-node}"
OUT_FILE="${OUT_DIR}/thinpool.json"
RUN_GROUP="${RUN_GROUP:-__RUN_GROUP__}"
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

install -d -m 0750 -o root -g "$RUN_GROUP" "$OUT_DIR" 2>/dev/null || install -d -m 0750 "$OUT_DIR"

command -v dmsetup >/dev/null 2>&1 || exit 0

get_block_sectors() {
  local pool="$1"
  local line
  line=$(dmsetup table "$pool" 2>/dev/null | head -n1 || true)
  [[ -z "$line" ]] && return 1
  echo "$line" | awk '{for(i=1;i<=NF;i++){if($i=="thin-pool"){print $(i+3); exit}}}'
}

parse_status_line() {
  local line="$1"
  local txn meta data
  txn=$(echo "$line" | awk '{for(i=1;i<=NF;i++){if($i=="thin-pool"){print $(i+1); exit}}}')
  meta=$(echo "$line" | awk '{for(i=1;i<=NF;i++){if($i=="thin-pool"){print $(i+2); exit}}}')
  data=$(echo "$line" | awk '{for(i=1;i<=NF;i++){if($i=="thin-pool"){print $(i+3); exit}}}')
  echo "$txn" "$meta" "$data"
}

now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo '{' > "$TMP"
echo "\"updated_at\":\"$now\"," >> "$TMP"
echo '"pools":[' >> "$TMP"

first=1

while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  pool=${line%%:*}
  [[ -z "$pool" ]] && continue

  read -r txn metaFrac dataFrac < <(parse_status_line "$line")

  metaUsed=${metaFrac%/*}; metaTotal=${metaFrac#*/}
  dataUsed=${dataFrac%/*}; dataTotal=${dataFrac#*/}

  bs=$(get_block_sectors "$pool" || true)
  [[ -z "$bs" || "$bs" == "0" ]] && continue

  blockBytes=$((bs * 512))
  totalBytes=$((dataTotal * blockBytes))
  usedBytes=$((dataUsed * blockBytes))
  freeBytes=0
  if (( totalBytes > usedBytes )); then freeBytes=$((totalBytes - usedBytes)); fi

  usedRatio=$(awk -v u="$usedBytes" -v t="$totalBytes" 'BEGIN{if(t==0)printf "0"; else printf "%.6f", u/t}')
  dataRatio=$(awk -v u="$dataUsed" -v t="$dataTotal" 'BEGIN{if(t==0)printf "0"; else printf "%.6f", u/t}')
  metaRatio=$(awk -v u="$metaUsed" -v t="$metaTotal" 'BEGIN{if(t==0)printf "0"; else printf "%.6f", u/t}')

  if [[ "$first" -eq 0 ]]; then echo ',' >> "$TMP"; fi
  first=0

  cat >> "$TMP" <<JSON
{
  "name":"$pool",
  "transaction_id":$txn,
  "block_bytes":$blockBytes,
  "total":$totalBytes,
  "used":$usedBytes,
  "free":$freeBytes,
  "used_ratio":$usedRatio,
  "data_ratio":$dataRatio,
  "meta_ratio":$metaRatio
}
JSON

done < <(dmsetup status --target thin-pool 2>/dev/null || true)

echo ']}' >> "$TMP"
install -m 0640 -o root -g "$RUN_GROUP" "$TMP" "$OUT_FILE" 2>/dev/null || install -m 0640 "$TMP" "$OUT_FILE"
EOF
  sed -i "s/__RUN_GROUP__/${RUN_GROUP}/g" "$tmp"
  as_root install -m 0755 "$tmp" "${COLLECTOR}"
  rm -f "$tmp"
}

write_cron() {
  as_root bash -c "cat > '${CRON_FILE}' <<'EOF'
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
* * * * * root /opt/node/collect_thinpool.sh >/dev/null 2>&1
EOF"
  as_root chmod 0644 "${CRON_FILE}"
}

write_smart_cache_helper() {
  local tmp
  tmp="$(mktemp)"
  cat > "$tmp" <<'EOF'
#!/usr/bin/env bash
set -u

CACHE_DIR="${CACHE_DIR:-/run/ithiltir-node}"
CACHE_FILE="${SMART_CACHE_FILE:-${CACHE_DIR}/smart.json}"
RUN_GROUP="${RUN_GROUP:-__RUN_GROUP__}"
TTL_SECONDS="${SMART_TTL_SECONDS:-300}"
SMARTCTL="${SMARTCTL:-smartctl}"
SCHEMA=1

TMP="$(mktemp)"
SCAN_TMP="$(mktemp)"
DETAIL_TMP="$(mktemp)"
ERR_TMP="$(mktemp)"
trap 'rm -f "$TMP" "$SCAN_TMP" "$DETAIL_TMP" "$ERR_TMP"' EXIT

json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/ }"
  s="${s//$'\r'/ }"
  printf "%s" "$s"
}

json_string_field() {
  local key="$1" value="$2"
  [[ -z "$value" ]] && return 0
  printf ',"%s":"%s"' "$key" "$(json_escape "$value")"
}

json_number_field() {
  local key="$1" value="$2"
  [[ -z "$value" ]] && return 0
  [[ "$value" =~ ^-?[0-9]+([.][0-9]+)?$ ]] || return 0
  printf ',"%s":%s' "$key" "$value"
}

write_cache() {
  local status="$1" devices="${2:-}"
  local now
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  install -d -m 0750 -o root -g "$RUN_GROUP" "$CACHE_DIR" 2>/dev/null || install -d -m 0750 "$CACHE_DIR"
  printf '{"schema":%d,"updated_at":"%s","ttl_seconds":%d,"status":"%s","devices":[%s]}\n' \
    "$SCHEMA" "$now" "$TTL_SECONDS" "$(json_escape "$status")" "$devices" > "$TMP"
  install -m 0640 -o root -g "$RUN_GROUP" "$TMP" "$CACHE_FILE" 2>/dev/null || install -m 0640 "$TMP" "$CACHE_FILE"
}

extract_string() {
  local key="$1" file="$2"
  awk -v key="\"${key}\"" '
    index($0, key) {
      line=$0
      sub(/^.*:[[:space:]]*"/, "", line)
      sub(/".*$/, "", line)
      print line
      exit
    }
  ' "$file"
}

extract_number() {
  local key="$1" file="$2"
  awk -v key="\"${key}\"" '
    index($0, key) {
      line=$0
      sub(/^.*:[[:space:]]*/, "", line)
      sub(/[,}].*$/, "", line)
      gsub(/[[:space:]]/, "", line)
      if (line ~ /^-?[0-9]+([.][0-9]+)?$/) {
        print line
        exit
      }
    }
  ' "$file"
}

extract_health() {
  local file="$1"
  awk '
    /"passed"[[:space:]]*:[[:space:]]*true/ { print "passed"; exit }
    /"passed"[[:space:]]*:[[:space:]]*false/ { print "failed"; exit }
  ' "$file"
}

extract_temp_c() {
  local file="$1"
  awk '
    function emit(line) {
      sub(/^.*:[[:space:]]*/, "", line)
      sub(/[,}].*$/, "", line)
      gsub(/[[:space:]]/, "", line)
      if (line ~ /^-?[0-9]+([.][0-9]+)?$/) {
        print line
        exit
      }
    }
    /"temperature"[[:space:]]*:/ {
      line=$0
      if (line ~ /"temperature"[[:space:]]*:[[:space:]]*-?[0-9]/) {
        emit(line)
      }
      if (line ~ /"current"[[:space:]]*:/) {
        sub(/^.*"current"[[:space:]]*:[[:space:]]*/, "", line)
        emit(line)
      }
      in_temp=1
      next
    }
    in_temp && /"current"[[:space:]]*:/ {
      emit($0)
    }
    in_temp && /}/ { in_temp=0 }
  ' "$file"
}

extract_power_hours() {
  local file="$1"
  awk '
    /"power_on_time"[[:space:]]*:/ {
      line=$0
      if (line ~ /"hours"[[:space:]]*:/) {
        sub(/^.*"hours"[[:space:]]*:[[:space:]]*/, "", line)
        sub(/[,}].*$/, "", line)
        gsub(/[[:space:]]/, "", line)
        if (line ~ /^[0-9]+$/) {
          print line
          exit
        }
      }
      in_power=1
      next
    }
    in_power && /"hours"[[:space:]]*:/ {
      line=$0
      sub(/^.*:[[:space:]]*/, "", line)
      sub(/[,}].*$/, "", line)
      gsub(/[[:space:]]/, "", line)
      if (line ~ /^[0-9]+$/) {
        print line
        exit
      }
    }
    in_power && /}/ { in_power=0 }
  ' "$file"
}

extract_failing_attrs() {
  local file="$1"
  awk '
    function reset() {
      id=""
      name=""
      when_failed=""
    }
    function json_escape(s) {
      gsub(/\\/, "\\\\", s)
      gsub(/"/, "\\\"", s)
      gsub(/\r/, " ", s)
      gsub(/\n/, " ", s)
      return s
    }
    function emit() {
      if (toupper(when_failed) == "FAILING_NOW" && name != "") {
        if (out != "") {
          out=out ","
        }
        if (id ~ /^[0-9]+$/) {
          out=out "{\"id\":" id ",\"name\":\"" json_escape(name) "\",\"when_failed\":\"" json_escape(when_failed) "\"}"
        } else {
          out=out "{\"name\":\"" json_escape(name) "\",\"when_failed\":\"" json_escape(when_failed) "\"}"
        }
      }
      reset()
    }
    function string_value(line) {
      sub(/^.*:[[:space:]]*"/, "", line)
      sub(/".*$/, "", line)
      return line
    }
    function number_value(line) {
      sub(/^.*:[[:space:]]*/, "", line)
      sub(/[,}].*$/, "", line)
      gsub(/[[:space:]]/, "", line)
      return line
    }
    /"ata_smart_attributes"[[:space:]]*:/ {
      in_attrs=1
      next
    }
    in_attrs && /"table"[[:space:]]*:/ {
      in_table=1
      next
    }
    in_table && /^[[:space:]]*{[[:space:]]*$/ {
      reset()
      in_item=1
      depth=1
      next
    }
    in_item && /"id"[[:space:]]*:/ {
      id=number_value($0)
    }
    in_item && /"name"[[:space:]]*:/ {
      name=string_value($0)
    }
    in_item && /"when_failed"[[:space:]]*:/ {
      when_failed=string_value($0)
    }
    in_item {
      line=$0
      depth += gsub(/{/, "{", line)
      depth -= gsub(/}/, "}", line)
      if (depth <= 0) {
        emit()
        in_item=0
      }
      next
    }
    in_table && /^[[:space:]]*]/ {
      in_table=0
      in_attrs=0
    }
    END {
      if (out != "") {
        print out
      }
    }
  ' "$file"
}

run_detail() {
  local path="$1" dtype="$2"
  local -a args
  args=(-j -a -n standby)
  if [[ -n "$dtype" ]]; then
    args+=(-d "$dtype")
  fi
  args+=("$path")

  : > "$DETAIL_TMP"
  : > "$ERR_TMP"
  if command -v timeout >/dev/null 2>&1; then
    timeout 60 "$SMARTCTL" "${args[@]}" >"$DETAIL_TMP" 2>"$ERR_TMP"
  else
    "$SMARTCTL" "${args[@]}" >"$DETAIL_TMP" 2>"$ERR_TMP"
  fi
}

if ! command -v "$SMARTCTL" >/dev/null 2>&1; then
  write_cache "no_tool" ""
  exit 0
fi

if ! "$SMARTCTL" --scan-open >"$SCAN_TMP" 2>/dev/null; then
  "$SMARTCTL" --scan >"$SCAN_TMP" 2>/dev/null || true
fi

devices_json=""
count=0
ok_count=0
standby_count=0
permission_count=0
timeout_count=0
error_count=0

while IFS= read -r raw; do
  line="${raw%%#*}"
  read -r path opt dtype _ <<< "$line"
  [[ "$path" == /dev/* ]] || continue

  device_type=""
  if [[ "$opt" == "-d" && -n "${dtype:-}" ]]; then
    device_type="$dtype"
  fi

  exit_status=0
  run_detail "$path" "$device_type" || exit_status=$?

  health="$(extract_health "$DETAIL_TMP")"
  temp_c="$(extract_temp_c "$DETAIL_TMP")"
  power_hours="$(extract_power_hours "$DETAIL_TMP")"
  lifetime_used="$(extract_number "percentage_used" "$DETAIL_TMP")"
  critical_warning="$(extract_number "critical_warning" "$DETAIL_TMP")"
  failing_attrs="$(extract_failing_attrs "$DETAIL_TMP")"
  protocol="$(extract_string "protocol" "$DETAIL_TMP")"
  model="$(extract_string "model_name" "$DETAIL_TMP")"
  serial="$(extract_string "serial_number" "$DETAIL_TMP")"

  status="ok"
  if [[ "$exit_status" -eq 124 ]]; then
    status="timeout"
  elif grep -qiE 'permission denied|operation not permitted' "$ERR_TMP"; then
    status="no_permission"
  elif [[ -z "$health" && -z "$temp_c" ]] && grep -qi 'standby' "$DETAIL_TMP" "$ERR_TMP"; then
    status="standby"
  elif [[ ! -s "$DETAIL_TMP" || ( -z "$health" && -z "$temp_c" && -z "$model" && "$exit_status" -ne 0 ) ]]; then
    status="error"
  fi

  case "$status" in
    ok) ok_count=$((ok_count + 1)) ;;
    standby) standby_count=$((standby_count + 1)) ;;
    no_permission) permission_count=$((permission_count + 1)) ;;
    timeout) timeout_count=$((timeout_count + 1)) ;;
    error) error_count=$((error_count + 1)) ;;
  esac

  name="$(basename "$path")"
  device="{\"name\":\"$(json_escape "$name")\",\"source\":\"smartctl\",\"status\":\"$(json_escape "$status")\""
  device+="$(json_string_field "device_path" "$path")"
  device+="$(json_string_field "device_type" "$device_type")"
  device+="$(json_string_field "protocol" "$protocol")"
  device+="$(json_string_field "model" "$model")"
  device+="$(json_string_field "serial" "$serial")"
  if [[ "$exit_status" =~ ^[0-9]+$ ]]; then
    device+=",\"exit_status\":$exit_status"
  fi
  device+="$(json_string_field "health" "$health")"
  device+="$(json_number_field "temp_c" "$temp_c")"
  device+="$(json_number_field "power_on_hours" "$power_hours")"
  device+="$(json_number_field "lifetime_used_percent" "$lifetime_used")"
  device+="$(json_number_field "critical_warning" "$critical_warning")"
  if [[ -n "$failing_attrs" ]]; then
    device+=",\"failing_attrs\":[${failing_attrs}]"
  fi
  device+="}"

  if [[ -n "$devices_json" ]]; then
    devices_json+=","
  fi
  devices_json+="$device"
  count=$((count + 1))
done < "$SCAN_TMP"

if [[ "$count" -eq 0 ]]; then
  write_cache "not_found" ""
  exit 0
fi

top_status="partial"
if [[ "$ok_count" -eq "$count" ]]; then
  top_status="ok"
elif [[ "$standby_count" -eq "$count" ]]; then
  top_status="standby"
elif [[ "$permission_count" -eq "$count" ]]; then
  top_status="no_permission"
elif [[ "$timeout_count" -eq "$count" ]]; then
  top_status="timeout"
elif [[ "$error_count" -eq "$count" ]]; then
  top_status="error"
fi

write_cache "$top_status" "$devices_json"
EOF
  sed -i "s/__RUN_GROUP__/${RUN_GROUP}/g" "$tmp"
  as_root install -d -m 0755 "$(dirname "${SMART_HELPER_FILE}")"
  as_root install -m 0755 "$tmp" "${SMART_HELPER_FILE}"
  rm -f "$tmp"
}

write_smart_cache_service() {
  as_root bash -c "cat > '${SMART_SERVICE_FILE}' <<EOF
[Unit]
Description=Ithiltir node SMART cache refresh

[Service]
Type=oneshot
User=root
Group=${RUN_GROUP}
UMask=0027
ExecStart=${SMART_HELPER_FILE}
EOF"
}

write_smart_cache_timer() {
  as_root bash -c "cat > '${SMART_TIMER_FILE}' <<EOF
[Unit]
Description=Refresh Ithiltir node SMART cache

[Timer]
OnBootSec=1min
OnUnitActiveSec=5min
AccuracySec=30s
Unit=${SMART_SERVICE_NAME}

[Install]
WantedBy=timers.target
EOF"
}

enable_smart_cache_timer() {
  if ! need_cmd systemctl; then
    echo "[!] systemctl not found; skipping SMART cache timer" >&2
    return 0
  fi
  if ! as_root systemctl enable --now "${SMART_TIMER_NAME}" >/dev/null 2>&1; then
    echo "[!] could not enable ${SMART_TIMER_NAME}; node service install continues" >&2
    return 0
  fi
  as_root systemctl start "${SMART_SERVICE_NAME}" >/dev/null 2>&1 || true
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
  ensure_user

  as_root mkdir -p "${INSTALL_DIR}"
  as_root mkdir -p "${RELEASES_DIR}"
  as_root mkdir -p "${DATA_DIR}"
  as_root chown -R "${RUN_USER}:${RUN_GROUP}" "${DATA_DIR}"

  write_tmpfiles
  install_smartmontools

  tmp="$(mktemp)"
  trap "rm -f '$tmp'" EXIT
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
  as_root chown -R "${RUN_USER}:${RUN_GROUP}" "${DATA_DIR}"
  as_root chown -h "${RUN_USER}:${RUN_GROUP}" "${CURRENT_DIR}" >/dev/null 2>&1 || true

  as_root systemctl stop "${APP}.service" >/dev/null 2>&1 || true

  if [[ $# -gt 0 ]]; then
    write_service_push "${interval}" "$@"
  else
    write_service_push "${interval}"
  fi

  local lvm_detected=0
  if has_lvm; then
    lvm_detected=1
    echo "[+] LVM/LVM-thin detected; installing cron and enabling thinpool cache"
    install_cron || true
    write_collector
    write_cron
    as_root "${COLLECTOR}" || true
  else
    echo "[-] No LVM detected; skipping cron/collector"
    as_root rm -f "${CRON_FILE}" >/dev/null 2>&1 || true
    as_root rm -f "${COLLECTOR}" >/dev/null 2>&1 || true
  fi

  write_smart_cache_helper
  write_smart_cache_service
  write_smart_cache_timer

  as_root systemctl daemon-reload
  enable_smart_cache_timer
  as_root systemctl enable --now "${APP}.service"

  echo "[OK] Done: ${APP}.service is running and enabled on boot"
  echo "     Status: systemctl status ${APP}.service"
  echo "     Logs:   journalctl -u ${APP}.service -f"
  echo "     SMART cache timer: ${SMART_TIMER_NAME}"
  if [[ "${lvm_detected}" -eq 1 ]]; then
    echo "     LVM cache: ${CACHE_FILE}"
  fi
}

main "$@"
