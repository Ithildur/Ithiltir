#!/usr/bin/env bash
set -euo pipefail

# Lifecycle contract: run this installer once on a fresh Dash host. It is not
# a reinstall, repair, rollback, or version-update entrypoint. After the first
# successful installation, every version change uses the dash update subcommand
# directly or through the compatibility wrapper update_dash_linux.sh. Installation and
# update workflows are never run concurrently.

APP="dash"

INSTALL_DIR="/opt/Ithiltir-dash"
RELEASES_DIR="${INSTALL_DIR}/releases"
CURRENT_LINK="${INSTALL_DIR}/current"
LAYOUT_MARKER="${INSTALL_DIR}/.release-layout-v1"
BIN_DIR="${INSTALL_DIR}/bin"
BIN_PATH="${BIN_DIR}/dash"

CONFIG_DIR="${INSTALL_DIR}/configs"
CONFIG_EXAMPLE="${CONFIG_DIR}/config.example.yaml"
CONFIG_LOCAL="${CONFIG_DIR}/config.local.yaml"
NOTIFY_CONFIG_KEY="${CONFIG_DIR}/notify-config.key"

SERVICE_FILE="/etc/systemd/system/${APP}.service"
MANUAL_RUN_FILE="${INSTALL_DIR}/run_dash.sh"

REDIS_INSTALL_METHOD="${REDIS_INSTALL_METHOD:-package}"
REDIS_TAKEOVER_CONFIRMED="0"
SERVICE_MANAGER_MODE="${SERVICE_MANAGER_MODE:-auto}"
SERVICE_MANAGER=""
INSTALL_RELEASE=""
INSTALL_RECOVERY_ROOT=""
INSTALL_PREVIOUS_LAYOUT=""
INSTALL_PREVIOUS_TARGET=""
INSTALL_PREVIOUS_MARKER="0"
INSTALL_PREVIOUS_CONFIG="0"
INSTALL_SYSTEMD_WAS_ACTIVE="0"
INSTALL_ROLLBACK_READY="0"
INSTALL_MIGRATION_STARTED="0"

OS_ID=""
OS_VERSION_ID=""
APT_REPO_ID=""
APT_REPO_CODENAME=""
OS_FAMILY=""
PKG_MANAGER=""
PKG_MANAGER_LABEL=""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

INSTALL_LANG="${INSTALL_LANG:-}"

usage() {
	cat <<EOF
Usage: $0 [--lang zh|en] [--service-manager auto|systemd|none]
EOF
}

parse_args() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
		--lang)
			[[ $# -ge 2 ]] || die "missing value for --lang"
			INSTALL_LANG="$2"
			shift 2
			;;
		--lang=*)
			INSTALL_LANG="${1#--lang=}"
			shift
			;;
		--service-manager)
			[[ $# -ge 2 ]] || die "missing value for --service-manager"
			SERVICE_MANAGER_MODE="$2"
			shift 2
			;;
		--service-manager=*)
			SERVICE_MANAGER_MODE="${1#--service-manager=}"
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			die "unknown argument: $1"
			;;
		esac
	done
}

default_install_lang() {
	case "${LANG:-}" in
	zh* | zh_*) echo "zh" ;;
	*) echo "en" ;;
	esac
}

choose_install_lang() {
	case "${INSTALL_LANG}" in
	zh | en) return 0 ;;
	"") ;;
	*) die "INSTALL_LANG must be zh or en: ${INSTALL_LANG}" ;;
	esac

	local default ans
	default="$(default_install_lang)"
	if [[ ! -t 0 ]]; then
		INSTALL_LANG="$default"
		return 0
	fi

	while true; do
		echo "Select installer language / 选择安装脚本语言:" >&2
		echo "  1) English" >&2
		echo "  2) 中文" >&2
		if [[ "$default" == "zh" ]]; then
			read -r -p "Enter number / 请输入序号 [2] " ans || true
			ans="${ans:-2}"
		else
			read -r -p "Enter number / 请输入序号 [1] " ans || true
			ans="${ans:-1}"
		fi
		case "$ans" in
		1) INSTALL_LANG="en"; return 0 ;;
		2) INSTALL_LANG="zh"; return 0 ;;
		*) echo "Please enter 1 or 2 / 请输入 1 或 2" >&2 ;;
		esac
	done
}

txt() {
	if [[ "${INSTALL_LANG:-zh}" == "en" ]]; then
		printf '%s' "$2"
	else
		printf '%s' "$1"
	fi
}

say() { echo "$(txt "$1" "$2")"; }
say_err() { echo "$(txt "$1" "$2")" >&2; }

need_cmd() { command -v "$1" >/dev/null 2>&1; }

as_root() {
	if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
		"$@"
		return
	fi
	if need_cmd sudo; then
		sudo "$@"
		return
	fi
	say_err "需要 root 权限且未安装 sudo，请以 root 运行" "Root privileges are required and sudo is not installed. Please run as root."
	exit 1
}

die() {
	echo "ERROR: $*" >&2
	exit 1
}

print_config_summary() {
	local dash_ip="$1" listen_port="$2" public_url="$3" db_user="$4" db_pass="$5" db_name="$6" retention_days="$7" redis_addr="$8" redis_password="$9" offline_threshold="${10}" language="${11}" trusted_proxies="${12}"
	local retention_label="default (45 days)"
	if [[ "$retention_days" != "default" ]]; then
		retention_label="${retention_days} days"
	fi

	say "即将写入配置：${CONFIG_LOCAL}" "About to write config: ${CONFIG_LOCAL}"
	echo "  app.dash_ip: ${dash_ip}"
	echo "  app.listen: :${listen_port}"
	echo "  app.public_url: ${public_url}"
	echo "  app.language: ${language}"
	echo "  database.user: ${db_user}"
	say "  database.password: (已隐藏，长度 ${#db_pass})" "  database.password: (hidden, length ${#db_pass})"
	echo "  database.name: ${db_name}"
	echo "  database.retention_days: ${retention_label}"
	echo "  app.node_offline_threshold: ${offline_threshold}"
	echo "  redis.addr: ${redis_addr}"
	if [[ -n "$redis_password" ]]; then
		say "  redis.password: (已配置并隐藏)" "  redis.password: (configured and hidden)"
	else
		say "  redis.password: (空)" "  redis.password: (empty)"
	fi
	echo "  http.trusted_proxies: ${trusted_proxies}"
}

as_postgres() {
	if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
		if need_cmd runuser; then
			runuser -u postgres -- "$@"
			return
		fi
		if need_cmd su; then
			su -s /bin/bash postgres -c "$(printf '%q ' "$@")"
			return
		fi
		die "$(txt "缺少 runuser/su，无法以 postgres 用户执行命令" "Missing runuser/su; cannot run commands as postgres")"
	fi

	if need_cmd sudo; then
		sudo -u postgres "$@"
		return
	fi
	die "$(txt "非 root 且未安装 sudo，无法以 postgres 用户执行命令" "Not running as root and sudo is not installed; cannot run commands as postgres")"
}

prompt_yes_no() {
	local prompt="$1" default="${2:-Y}" ans
	while true; do
		local hint="[Y/n]"
		case "${default,,}" in
		y | yes) hint="[Y/n]" ;;
		n | no) hint="[y/N]" ;;
		esac

		read -r -p "${prompt} ${hint} " ans || true
		ans="${ans:-$default}"
		case "${ans,,}" in
		y | yes) return 0 ;;
		n | no) return 1 ;;
		*) say "请输入 y 或 n" "Please enter y or n" ;;
		esac
	done
}

systemd_available() {
	need_cmd systemctl && [[ -d /run/systemd/system ]]
}

detect_service_manager() {
	if systemd_available; then
		printf '%s\n' systemd
		return 0
	fi
	printf '%s\n' none
}

select_service_manager() {
	local detected
	detected="$(detect_service_manager)"
	case "$SERVICE_MANAGER_MODE" in
	auto)
		[[ "$detected" != "none" ]] || die "$(txt "未检测到受支持的服务管理器；如需手动安装，请使用 --service-manager=none" "No supported service manager was detected; use --service-manager=none for a manual installation")"
		SERVICE_MANAGER="$detected"
		;;
	systemd)
		[[ "$detected" == "$SERVICE_MANAGER_MODE" ]] || die "$(txt "请求的服务管理器不可用：${SERVICE_MANAGER_MODE}" "Requested service manager is unavailable: ${SERVICE_MANAGER_MODE}")"
		SERVICE_MANAGER="$SERVICE_MANAGER_MODE"
		;;
	none)
		SERVICE_MANAGER="none"
		;;
	*)
		die "$(txt "无效的服务管理器：${SERVICE_MANAGER_MODE}（支持 auto/systemd/none）" "Invalid service manager: ${SERVICE_MANAGER_MODE} (supported: auto/systemd/none)")"
		;;
	esac
}

ensure_process_control() {
	need_cmd pgrep || die "$(txt "需要 pgrep 检查并停止现有 Dash 进程" "pgrep is required to detect and stop an existing Dash process")"
}

stop_manual_processes() {
	local -a pids=()
	mapfile -t pids < <(pgrep -f -- "$BIN_PATH" 2>/dev/null || true)
	((${#pids[@]} > 0)) || return 0

	say "停止现有手动 Dash 进程" "Stopping existing manually started Dash process"
	as_root kill -TERM "${pids[@]}" >/dev/null 2>&1 || true
	for _ in {1..50}; do
		mapfile -t pids < <(pgrep -f -- "$BIN_PATH" 2>/dev/null || true)
		((${#pids[@]} == 0)) && return 0
		sleep 0.1
	done
	as_root kill -KILL "${pids[@]}" >/dev/null 2>&1 || true
	sleep 0.1
	if pgrep -f -- "$BIN_PATH" >/dev/null 2>&1; then
		die "$(txt "无法停止现有 Dash 进程" "Failed to stop the existing Dash process")"
	fi
}

enable_time_sync() {
	say "启用系统时间同步（NTP，失败不影响安装）" "Enabling system time sync (NTP; non-fatal)"

	if need_cmd timedatectl && as_root timedatectl set-ntp true >/dev/null 2>&1; then
		say "系统时间同步已启用。" "System time sync is enabled."
		return 0
	fi

	local unit
	case "$SERVICE_MANAGER" in
	systemd)
		for unit in systemd-timesyncd.service chronyd.service ntpd.service ntp.service; do
			if as_root systemctl enable --now "$unit" >/dev/null 2>&1; then
				say "系统时间同步服务已启动：${unit}" "System time sync service started: ${unit}"
				return 0
			fi
		done
		;;
	none) ;;
	esac

	say_err "警告：未能自动启用系统时间同步，请手动检查 NTP/chrony/systemd-timesyncd。" "WARNING: Could not enable system time sync automatically; please check NTP/chrony/systemd-timesyncd manually."
	return 0
}

prompt_string() {
	local prompt="$1" default="${2:-}" out
	if [[ -n "$default" ]]; then
		read -r -p "${prompt} [${default}] " out || true
		echo "${out:-$default}"
	else
		read -r -p "${prompt} " out || true
		echo "$out"
	fi
}

prompt_language() {
	local default="${1:-$([[ "${INSTALL_LANG:-zh}" == "en" ]] && echo 1 || echo 2)}" ans
	while true; do
		say_err "请选择默认语言（app.language）：" "Select default language (app.language):"
		say_err "  1) English（en）" "  1) English (en)"
		say_err "  2) 中文（zh）" "  2) 中文 (zh)"
		read -r -p "$(txt "请输入序号 [${default}] " "Enter number [${default}] ")" ans || true
		ans="${ans:-$default}"
		case "$ans" in
		1)
			echo "en"
			return 0
			;;
		2)
			echo "zh"
			return 0
			;;
		*) say_err "请输入 1 或 2" "Please enter 1 or 2" ;;
		esac
	done
}

prompt_retention_days() {
	local default="${1:-1}" ans
	while true; do
		say_err "请选择历史保留时长（database.retention_days）：" "Select history retention (database.retention_days):"
		say_err "如需掌握流量历史或 95 计费历史，建议选择 90 days 或更高。" "Choose 90 days or higher if you need traffic history or 95th percentile billing history."
		say_err "  1) default（45 days）" "  1) default (45 days)"
		echo "  2) 90 days" >&2
		echo "  3) 180 days" >&2
		echo "  4) 365 days" >&2
		read -r -p "$(txt "请输入序号 [${default}] " "Enter number [${default}] ")" ans || true
		ans="${ans:-$default}"
		case "$ans" in
		1)
			echo "default"
			return 0
			;;
		2)
			echo "90"
			return 0
			;;
		3)
			echo "180"
			return 0
			;;
		4)
			echo "365"
			return 0
			;;
		*) say_err "请输入 1、2、3 或 4" "Please enter 1, 2, 3, or 4" ;;
		esac
	done
}

prompt_secret_confirm() {
	local prompt="$1" a b
	while true; do
		IFS= read -r -s -p "${prompt}: " a
		printf '\n' >&2
		IFS= read -r -s -p "$(txt "请再次输入确认: " "Confirm again: ")" b
		printf '\n' >&2
		[[ -n "$a" ]] || {
			say_err "不能为空，请重试" "Cannot be empty, please retry"
			continue
		}
		[[ "$a" == "$b" ]] || {
			say_err "两次输入不一致，请重试" "Passwords do not match, please retry"
			continue
		}
		echo "$a"
		return 0
	done
}

prompt_secret_optional() {
	local prompt="$1" a b
	while true; do
		IFS= read -r -s -p "${prompt}: " a
		printf '\n' >&2
		if [[ -z "$a" ]]; then
			printf '\n'
			return 0
		fi
		if [[ "$a" =~ [[:cntrl:]] ]]; then
			say_err "Redis 密码不能包含控制字符，请重试" "Redis password cannot contain control characters, please retry"
			continue
		fi
		IFS= read -r -s -p "$(txt "请再次输入确认: " "Confirm again: ")" b
		printf '\n' >&2
		[[ "$a" == "$b" ]] || {
			say_err "两次输入不一致，请重试" "Passwords do not match, please retry"
			continue
		}
		printf '%s\n' "$a"
		return 0
	done
}

version_ge() {
	local a="$1" b="$2"
	local IFS=.
	local -a av bv
	read -r -a av <<<"$a"
	read -r -a bv <<<"$b"
	for i in 0 1 2; do
		local ai="${av[i]:-0}" bi="${bv[i]:-0}"
		if ((10#${ai} > 10#${bi})); then return 0; fi
		if ((10#${ai} < 10#${bi})); then return 1; fi
	done
	return 0
}

find_listening_pids_on_port() {
	local port="$1"

	if need_cmd ss; then
		ss -H -ltnp "sport = :${port}" 2>/dev/null | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p' | sort -u
		return 0
	fi

	if need_cmd lsof; then
		lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t 2>/dev/null | sort -u
		return 0
	fi

	if need_cmd fuser; then
		fuser -n tcp "${port}" 2>/dev/null | tr ' ' '\n' | sed -n 's/^\([0-9][0-9]*\)$/\1/p' | sort -u
		return 0
	fi

	return 0
}

pid_comm() {
	local pid="$1"
	if [[ -r "/proc/${pid}/comm" ]]; then
		tr -d '\n' <"/proc/${pid}/comm"
		return 0
	fi
	if need_cmd ps; then
		ps -p "$pid" -o comm= 2>/dev/null | head -n1 | tr -d '[:space:]'
		return 0
	fi
	return 1
}

pid_looks_like_redis() {
	local pid="$1" comm=""
	comm="$(pid_comm "$pid" 2>/dev/null || true)"
	case "$comm" in
	redis | redis-server | redis-sentinel)
		return 0
		;;
	esac
	if [[ -r "/proc/${pid}/cmdline" ]]; then
		tr '\0' ' ' <"/proc/${pid}/cmdline" | grep -Eq '(^|[ /])(redis|redis-server|redis-sentinel)( |$)'
		return $?
	fi
	return 1
}

port_has_redis_listener() {
	local port="$1"
	local pids="${2:-}"
	local pid
	if [[ -z "$pids" ]]; then
		pids="$(find_listening_pids_on_port "$port")"
	fi
	for pid in $pids; do
		if pid_looks_like_redis "$pid"; then
			return 0
		fi
	done
	return 1
}

confirm_redis_takeover() {
	if [[ "$REDIS_TAKEOVER_CONFIRMED" == "1" ]]; then
		return 0
	fi

	local pids
	pids="$(find_listening_pids_on_port 6379)"
	if [[ ! -e /etc/redis/redis.conf && ! -e /etc/systemd/system/redis-server.service && ! -e /etc/init.d/redis && ! -e /etc/init.d/redis-server && -z "$pids" ]]; then
		REDIS_TAKEOVER_CONFIRMED="1"
		return 0
	fi

	if ! prompt_yes_no "$(txt "安装器将备份并覆盖现有 Redis 配置/服务，并停止 6379 端口上的旧进程。是否继续？" "The installer will back up and replace the existing Redis configuration/service and stop the old process on port 6379. Continue?")" "Y"; then
		return 1
	fi
	REDIS_TAKEOVER_CONFIRMED="1"
}

kill_listeners_on_port() {
	local port="$1"
	local pids
	pids="$(find_listening_pids_on_port "$port")"
	if [[ -z "$pids" ]]; then
		return 0
	fi
	if port_has_redis_listener "$port" "$pids"; then
		say "检测到端口 ${port} 已由 Redis 监听，先停止旧 Redis 进程：${pids}" "Port ${port} is already owned by Redis; stopping old Redis listeners: ${pids}"
	else
		say "检测到端口 ${port} 已被占用，尝试结束占用进程：${pids}" "Port ${port} is in use; terminating listeners: ${pids}"
	fi

	case "$SERVICE_MANAGER" in
	systemd)
		as_root systemctl stop redis-server.service >/dev/null 2>&1 || true
		as_root systemctl stop redis.service >/dev/null 2>&1 || true
		;;
	none) ;;
	esac

	for pid in $pids; do
		as_root kill -TERM "$pid" >/dev/null 2>&1 || true
	done
	sleep 1

	pids="$(find_listening_pids_on_port "$port")"
	if [[ -z "$pids" ]]; then
		return 0
	fi
	for pid in $pids; do
		as_root kill -KILL "$pid" >/dev/null 2>&1 || true
	done
	sleep 1

	pids="$(find_listening_pids_on_port "$port")"
	if [[ -n "$pids" ]]; then
		die "$(txt "无法释放端口 ${port}（仍占用的 PID：${pids}）" "Failed to free port ${port} (still listening PIDs: ${pids})")"
	fi
}

sql_escape_literal() {
	local s="$1"
	s="${s//\'/\'\'}"
	printf "%s" "$s"
}

systemd_escape_env_value() {
	local s="$1"
	s="${s//%/%%}"
	s="${s//\\/\\\\}"
	s="${s//\"/\\\"}"
	printf "%s" "$s"
}

shell_quote_arg() {
	local s="$1"
	s="${s//\'/\'\\\'\'}"
	printf "'%s'" "$s"
}

sed_escape_repl() {
	local s="$1"
	s="${s//\\/\\\\}"
	s="${s//&/\\&}"
	s="${s//|/\\|}"
	printf "%s" "$s"
}

write_redis_conf() {
	[[ "$REDIS_TAKEOVER_CONFIRMED" == "1" ]] || die "$(txt "内部错误：覆盖 Redis 配置前未确认" "Internal error: Redis takeover was not confirmed")"
	as_root install -d -m 0755 /etc/redis /var/lib/redis /var/log/redis
	as_root chown -R redis:redis /var/lib/redis /var/log/redis >/dev/null 2>&1 || true

	if [[ -f /etc/redis/redis.conf ]]; then
		local bak="/etc/redis/redis.conf.bak.$(date +%Y%m%d%H%M%S)"
		as_root cp -f /etc/redis/redis.conf "$bak"
	fi

	local supervised="no"
	[[ "$SERVICE_MANAGER" != "systemd" ]] || supervised="systemd"
	as_root bash -c "cat > /etc/redis/redis.conf <<EOF

bind 127.0.0.1 -::1
protected-mode yes
port 6379
tcp-backlog 511
timeout 0
tcp-keepalive 300

daemonize no
supervised ${supervised}
pidfile /run/redis/redis-server.pid

loglevel notice
logfile \"/var/log/redis/redis.log\"

databases 16
always-show-logo yes

save 900 1
save 300 10
save 60 10000
stop-writes-on-bgsave-error yes
rdbcompression yes
rdbchecksum yes
dbfilename dump.rdb
dir /var/lib/redis

appendonly no
appendfsync everysec
no-appendfsync-on-rewrite no
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb
aof-load-truncated yes
aof-use-rdb-preamble yes
EOF"
}

detect_os() {
	[[ -r /etc/os-release ]] || die "$(txt "无法读取 /etc/os-release" "Cannot read /etc/os-release")"
	. /etc/os-release

	OS_ID="${ID:-}"
	OS_VERSION_ID="${VERSION_ID:-}"

	case "${OS_ID}" in
	debian)
		local major="${OS_VERSION_ID%%.*}"
		[[ "$major" =~ ^[0-9]+$ ]] || die "$(txt "无法解析 Debian VERSION_ID=${OS_VERSION_ID:-}" "Cannot parse Debian VERSION_ID=${OS_VERSION_ID:-}")"
		((major >= 11)) || die "$(txt "仅支持 Debian 11+，当前 VERSION_ID=${OS_VERSION_ID}" "Only Debian 11+ is supported (current VERSION_ID=${OS_VERSION_ID})")"
		OS_FAMILY="debian"
		APT_REPO_ID="debian"
		APT_REPO_CODENAME="${DEBIAN_CODENAME:-${VERSION_CODENAME:-}}"
		PKG_MANAGER="apt-get"
		PKG_MANAGER_LABEL="apt-get"
		;;
	ubuntu)
		local major="${OS_VERSION_ID%%.*}"
		[[ "$major" =~ ^[0-9]+$ ]] || die "$(txt "无法解析 Ubuntu VERSION_ID=${OS_VERSION_ID:-}" "Cannot parse Ubuntu VERSION_ID=${OS_VERSION_ID:-}")"
		((major >= 22)) || die "$(txt "仅支持 Ubuntu 22+，当前 VERSION_ID=${OS_VERSION_ID}" "Only Ubuntu 22+ is supported (current VERSION_ID=${OS_VERSION_ID})")"
		OS_FAMILY="debian"
		APT_REPO_ID="ubuntu"
		APT_REPO_CODENAME="${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}"
		PKG_MANAGER="apt-get"
		PKG_MANAGER_LABEL="apt-get"
		;;
	rhel | rocky | almalinux | ol | centos)
		local major="${OS_VERSION_ID%%.*}"
		[[ "$major" =~ ^[0-9]+$ ]] || die "$(txt "无法解析 ${OS_ID} VERSION_ID=${OS_VERSION_ID:-}" "Cannot parse ${OS_ID} VERSION_ID=${OS_VERSION_ID:-}")"
		((major >= 8)) || die "$(txt "仅支持 RHEL/Rocky/Alma/Oracle/CentOS 8+，当前 VERSION_ID=${OS_VERSION_ID}" "Only RHEL/Rocky/Alma/Oracle/CentOS 8+ is supported (current VERSION_ID=${OS_VERSION_ID})")"
		OS_FAMILY="rhel"
		if need_cmd dnf; then
			PKG_MANAGER="dnf"
		elif need_cmd yum; then
			PKG_MANAGER="yum"
		else
			die "$(txt "未检测到 dnf/yum" "Neither dnf nor yum was found")"
		fi
		PKG_MANAGER_LABEL="${PKG_MANAGER}"
		;;
	fedora)
		local major="${OS_VERSION_ID%%.*}"
		[[ "$major" =~ ^[0-9]+$ ]] || die "$(txt "无法解析 Fedora VERSION_ID=${OS_VERSION_ID:-}" "Cannot parse Fedora VERSION_ID=${OS_VERSION_ID:-}")"
		((major >= 33)) || die "$(txt "仅支持 Fedora 33+，当前 VERSION_ID=${OS_VERSION_ID}" "Only Fedora 33+ is supported (current VERSION_ID=${OS_VERSION_ID})")"
		OS_FAMILY="fedora"
		if need_cmd dnf; then
			PKG_MANAGER="dnf"
		elif need_cmd yum; then
			PKG_MANAGER="yum"
		else
			die "$(txt "未检测到 dnf/yum" "Neither dnf nor yum was found")"
		fi
		PKG_MANAGER_LABEL="${PKG_MANAGER}"
		;;
	arch | manjaro)
		OS_FAMILY="arch"
		PKG_MANAGER="pacman"
		PKG_MANAGER_LABEL="pacman"
		;;
	alpine)
		OS_FAMILY="manual"
		PKG_MANAGER=""
		PKG_MANAGER_LABEL="manual"
		say_err "警告：Alpine 上的 Dash 仅支持显式手动模式；PostgreSQL 16+、匹配其主版本的 TimescaleDB 和 Redis 必须预先安装并运行。" "WARNING: Dash supports only explicit manual mode on Alpine; PostgreSQL 16+, TimescaleDB for that PostgreSQL major, and Redis must already be installed and running."
		return 0
		;;
	*)
		case " ${ID_LIKE:-} " in
		*" ubuntu "*)
			OS_FAMILY="debian"
			APT_REPO_ID="ubuntu"
			APT_REPO_CODENAME="${UBUNTU_CODENAME:-}"
			[[ -n "$APT_REPO_CODENAME" ]] || die "$(txt "无法确定 ${OS_ID} 对应的 Ubuntu 基础版本代号（UBUNTU_CODENAME）" "Cannot determine the Ubuntu base codename for ${OS_ID} (UBUNTU_CODENAME)")"
			PKG_MANAGER="apt-get"
			PKG_MANAGER_LABEL="apt-get"
			;;
		*" debian "*)
			OS_FAMILY="debian"
			APT_REPO_ID="debian"
			APT_REPO_CODENAME="${DEBIAN_CODENAME:-}"
			[[ -n "$APT_REPO_CODENAME" ]] || die "$(txt "无法确定 ${OS_ID} 对应的 Debian 基础版本代号（DEBIAN_CODENAME）" "Cannot determine the Debian base codename for ${OS_ID} (DEBIAN_CODENAME)")"
			PKG_MANAGER="apt-get"
			PKG_MANAGER_LABEL="apt-get"
			;;
		*" rhel "* | *" fedora "*)
			OS_FAMILY="rhel"
			if need_cmd dnf; then
				PKG_MANAGER="dnf"
			elif need_cmd yum; then
				PKG_MANAGER="yum"
			else
				die "$(txt "未检测到 dnf/yum" "Neither dnf nor yum was found")"
			fi
			PKG_MANAGER_LABEL="${PKG_MANAGER}"
			;;
		*" arch "*)
			OS_FAMILY="arch"
			PKG_MANAGER="pacman"
			PKG_MANAGER_LABEL="pacman"
			;;
		*)
			if [[ "$SERVICE_MANAGER" == "none" ]]; then
				OS_FAMILY="manual"
				PKG_MANAGER=""
				PKG_MANAGER_LABEL="manual"
				return 0
			fi
			die "$(txt "仅支持 Debian/Ubuntu、RHEL/Rocky/Alma/Oracle/Fedora、Arch/Manjaro；当前系统 ID=${OS_ID:-unknown}" "Only Debian/Ubuntu, RHEL/Rocky/Alma/Oracle/Fedora, and Arch/Manjaro are supported (current ID=${OS_ID:-unknown})")"
			;;
		esac
		;;
	esac
}

pkg_update() {
	case "${PKG_MANAGER}" in
	apt-get)
		as_root apt-get update -y
		;;
	dnf)
		as_root dnf makecache -y
		;;
	yum)
		as_root yum makecache -y
		;;
	pacman)
		as_root pacman -Sy --noconfirm
		;;
	*)
		die "$(txt "不支持的包管理器：${PKG_MANAGER:-unknown}" "Unsupported package manager: ${PKG_MANAGER:-unknown}")"
		;;
	esac
}

pkg_install() {
	case "${PKG_MANAGER}" in
	apt-get)
		as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
		;;
	dnf)
		as_root dnf install -y "$@"
		;;
	yum)
		as_root yum install -y "$@"
		;;
	pacman)
		as_root pacman -S --noconfirm --needed "$@"
		;;
	*)
		die "$(txt "不支持的包管理器：${PKG_MANAGER:-unknown}" "Unsupported package manager: ${PKG_MANAGER:-unknown}")"
		;;
	esac
}

pkg_install_first_available() {
	local pkg
	for pkg in "$@"; do
		if pkg_install "$pkg" >/dev/null 2>&1; then
			printf '%s\n' "$pkg"
			return 0
		fi
	done
	return 1
}

ensure_pkg_prereqs() {
	pkg_update
	case "${OS_FAMILY}" in
	debian)
		pkg_install ca-certificates curl gnupg
		;;
	rhel | fedora)
		pkg_install ca-certificates curl gnupg2
		;;
	arch)
		pkg_install curl gnupg
		;;
	*)
		die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")"
		;;
	esac
}

ensure_postgres_binaries_on_path() {
	local current_major=""
	if need_cmd psql; then
		current_major="$(psql --version 2>/dev/null | sed -nE 's/.* ([0-9]+)(\.[0-9]+)?.*/\1/p')"
	fi

	local dir major best_dir="" best_major=0
	if [[ "$current_major" =~ ^[0-9]+$ ]] && ((current_major >= 16)); then
		best_major="$current_major"
	fi
	local -a dirs=()
	shopt -s nullglob
	dirs=(/usr/pgsql-*/bin /usr/lib/postgresql/*/bin)
	shopt -u nullglob
	for dir in "${dirs[@]}"; do
		[[ -x "${dir}/psql" ]] || continue
		major="$("${dir}/psql" --version 2>/dev/null | sed -nE 's/.* ([0-9]+)(\.[0-9]+)?.*/\1/p')"
		if [[ "$major" =~ ^[0-9]+$ ]] && ((major >= 16 && major > best_major)); then
			best_dir="$dir"
			best_major="$major"
		fi
	done
	if [[ -n "$best_dir" ]]; then
		export PATH="${best_dir}:${PATH}"
	fi
}

systemd_enable_now_first() {
	local unit
	for unit in "$@"; do
		if as_root systemctl enable --now "$unit" >/dev/null 2>&1; then
			return 0
		fi
	done
	return 1
}

systemd_restart_first() {
	local unit
	for unit in "$@"; do
		if as_root systemctl restart "$unit" >/dev/null 2>&1; then
			return 0
		fi
	done
	return 1
}

enable_postgres_service() {
	local pg_major
	pg_major="$(postgres_major_version)"
	[[ "$pg_major" =~ ^[0-9]+$ ]] || pg_major="16"
	case "$SERVICE_MANAGER" in
	systemd)
		case "${OS_FAMILY}" in
		debian | arch) as_root systemctl enable --now postgresql.service ;;
		rhel | fedora) as_root systemctl enable --now "postgresql-${pg_major}.service" ;;
		*) die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")" ;;
		esac
		;;
	none) return 0 ;;
	esac
}

restart_postgres_service() {
	local pg_major
	pg_major="$(postgres_major_version)"
	[[ "$pg_major" =~ ^[0-9]+$ ]] || die "$(txt "无法确定 PostgreSQL 主版本" "Cannot determine PostgreSQL major version")"
	case "$SERVICE_MANAGER" in
	systemd)
		case "${OS_FAMILY}" in
		debian | arch) as_root systemctl restart postgresql.service ;;
		rhel | fedora) as_root systemctl restart "postgresql-${pg_major}.service" ;;
		*) die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")" ;;
		esac
		;;
	none) return 0 ;;
	esac
}

enable_restart_redis_service() {
	case "$SERVICE_MANAGER" in
	systemd)
		systemd_enable_now_first redis-server.service redis6.service redis.service >/dev/null 2>&1 || true
		systemd_restart_first redis-server.service redis6.service redis.service >/dev/null 2>&1 || true
		;;
	none) return 0 ;;
	esac
}

write_systemd_redis_service() {
	[[ "$REDIS_TAKEOVER_CONFIRMED" == "1" ]] || die "$(txt "内部错误：覆盖 Redis 服务前未确认" "Internal error: Redis takeover was not confirmed")"
	local unit="/etc/systemd/system/redis-server.service"
	as_root install -d -m 0755 /etc/systemd/system
	if [[ -e "$unit" || -L "$unit" ]]; then
		local bak="${unit}.bak.$(date +%Y%m%d%H%M%S)"
		as_root cp -a "$unit" "$bak" >/dev/null 2>&1 || true
		as_root rm -f "$unit"
	fi

	as_root bash -c "cat > ${unit} <<'EOF'
[Unit]
Description=Redis In-Memory Data Store
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
User=redis
Group=redis
RuntimeDirectory=redis
RuntimeDirectoryMode=0755
ExecStart=/usr/local/bin/redis-server /etc/redis/redis.conf
ExecStop=/usr/local/bin/redis-cli shutdown
Restart=always
RestartSec=2
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF"
}

write_redis_service() {
	[[ "$REDIS_TAKEOVER_CONFIRMED" == "1" ]] || die "$(txt "内部错误：覆盖 Redis 服务前未确认" "Internal error: Redis takeover was not confirmed")"
	case "$SERVICE_MANAGER" in
	systemd) write_systemd_redis_service ;;
	none) return 0 ;;
	esac
}

install_repo_key() {
	local url="$1" fingerprints="$2" destination="$3" format="${4:-armored}"
	local tmp actual expected
	tmp="$(mktemp -d)"
	if ! curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL -o "${tmp}/key" "$url"; then
		rm -rf "$tmp"
		die "$(txt "下载仓库签名密钥失败：${url}" "Failed to download repository signing key: ${url}")"
	fi
	if ! actual="$(gpg --batch --with-colons --show-keys "${tmp}/key" 2>/dev/null |
		awk -F: '$1 == "pub" {primary=1; next} primary && $1 == "fpr" {print $10; primary=0}' |
		sort -u)"; then
		rm -rf "$tmp"
		die "$(txt "无法读取仓库签名密钥：${url}" "Failed to inspect repository signing key: ${url}")"
	fi
	expected="$(printf '%s\n' "$fingerprints" | tr ',' '\n' | sort -u)"
	if [[ -z "$actual" || "$actual" != "$expected" ]]; then
		rm -rf "$tmp"
		die "$(txt "仓库签名密钥指纹不匹配：${url}" "Repository signing-key fingerprint mismatch: ${url}")"
	fi

	local source="${tmp}/key"
	if [[ "$format" == "gpg" ]]; then
		if ! gpg --batch --yes --dearmor --output "${tmp}/key.gpg" "${tmp}/key"; then
			rm -rf "$tmp"
			die "$(txt "转换仓库签名密钥失败" "Failed to convert repository signing key")"
		fi
		source="${tmp}/key.gpg"
	fi
	as_root install -D -m 0644 "$source" "$destination"
	rm -rf "$tmp"
}

setup_postgresql_repo() {
	local arch
	arch="$(uname -m)"

	case "${OS_FAMILY}" in
	debian)
		if [[ -z "${APT_REPO_CODENAME}" && "$OS_ID" == "$APT_REPO_ID" ]] && need_cmd lsb_release; then
			APT_REPO_CODENAME="$(lsb_release -cs 2>/dev/null || true)"
		fi
		[[ -n "${APT_REPO_CODENAME}" ]] || die "$(txt "无法确定 Debian/Ubuntu 基础版本代号" "Cannot determine the Debian/Ubuntu base codename")"
		local keyring="/usr/share/keyrings/postgresql-archive-keyring.gpg"
		install_repo_key \
			https://www.postgresql.org/media/keys/ACCC4CF8.asc \
			B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8 \
			"$keyring" \
			gpg
		as_root bash -c "cat > /etc/apt/sources.list.d/pgdg.list <<EOF
deb [signed-by=${keyring}] https://apt.postgresql.org/pub/repos/apt ${APT_REPO_CODENAME}-pgdg main
EOF"
		;;
	rhel)
		local major="${OS_VERSION_ID%%.*}"
		pkg_install "https://download.postgresql.org/pub/repos/yum/reporpms/EL-${major}-${arch}/pgdg-redhat-repo-latest.noarch.rpm"
		if [[ "${PKG_MANAGER}" == "dnf" ]]; then
			as_root dnf -qy module disable postgresql >/dev/null 2>&1 || true
		else
			as_root yum -qy module disable postgresql >/dev/null 2>&1 || true
		fi
		;;
	fedora)
		local major="${OS_VERSION_ID%%.*}"
		pkg_install "https://download.postgresql.org/pub/repos/yum/reporpms/F-${major}-${arch}/pgdg-fedora-repo-latest.noarch.rpm"
		if [[ "${PKG_MANAGER}" == "dnf" ]]; then
			as_root dnf -qy module disable postgresql >/dev/null 2>&1 || true
		else
			as_root yum -qy module disable postgresql >/dev/null 2>&1 || true
		fi
		;;
	arch)
		return 0
		;;
	*)
		die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")"
		;;
	esac
}

postgres_cluster_initialized() {
	local pg_version_file
	for pg_version_file in \
		/var/lib/postgresql/16/main/PG_VERSION \
		/var/lib/postgresql/data/PG_VERSION \
		/var/lib/pgsql/16/data/PG_VERSION \
		/var/lib/pgsql/data/PG_VERSION \
		/var/lib/postgres/data/PG_VERSION; do
		if [[ -f "$pg_version_file" ]]; then
			return 0
		fi
	done
	return 1
}

init_postgres_cluster_if_needed() {
	postgres_cluster_initialized && return 0

	ensure_postgres_binaries_on_path
	case "${OS_FAMILY}" in
	debian)
		need_cmd pg_createcluster || die "$(txt "缺少 pg_createcluster，无法初始化 PostgreSQL 16" "pg_createcluster is required to initialize PostgreSQL 16")"
		as_root pg_createcluster 16 main
		;;
	rhel | fedora)
		[[ -x /usr/pgsql-16/bin/postgresql-16-setup ]] || die "$(txt "缺少 postgresql-16-setup" "postgresql-16-setup is missing")"
		as_root /usr/pgsql-16/bin/postgresql-16-setup initdb
		;;
	arch)
		need_cmd initdb || die "$(txt "缺少 initdb" "initdb is missing")"
		as_root install -d -m 0700 -o postgres -g postgres /var/lib/postgres/data
		as_postgres initdb -D /var/lib/postgres/data
		;;
	*)
		die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")"
		;;
	esac
	postgres_cluster_initialized || die "$(txt "PostgreSQL 数据目录初始化失败" "PostgreSQL data directory was not initialized")"
}

install_postgresql16() {
	ensure_pkg_prereqs
	setup_postgresql_repo
	pkg_update

	case "${OS_FAMILY}" in
	debian)
		pkg_install postgresql-16 postgresql-client-16
		;;
	rhel | fedora)
		pkg_install postgresql16-server postgresql16
		;;
	arch)
		pkg_install postgresql
		;;
	*)
		die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")"
		;;
	esac

	ensure_postgres_binaries_on_path
	init_postgres_cluster_if_needed
	enable_postgres_service
}

postgres_major_version() {
	ensure_postgres_binaries_on_path
	if ! need_cmd psql; then
		echo ""
		return 0
	fi
	local server_num v
	server_num="$(as_postgres psql -d postgres -tAc 'SHOW server_version_num' 2>/dev/null | tr -d '[:space:]' || true)"
	if [[ "$server_num" =~ ^[0-9]+$ ]] && ((server_num >= 10000)); then
		echo "$((server_num / 10000))"
		return 0
	fi
	v="$(psql --version 2>/dev/null | awk '{print $3}' || true)"
	echo "${v%%.*}"
}

ensure_postgresql16_and_password() {
	local installed_by_script="0"
	local major
	major="$(postgres_major_version)"

	if [[ -z "$major" ]] || ! [[ "$major" =~ ^[0-9]+$ ]]; then
		if prompt_yes_no "$(txt "未检测到 PostgreSQL，是否安装 PostgreSQL 16？" "PostgreSQL not detected. Install PostgreSQL 16?")"; then
			install_postgresql16
			installed_by_script="1"
		else
			die "$(txt "未安装 PostgreSQL，无法继续" "PostgreSQL is required")"
		fi
	elif ((major < 16)); then
		if prompt_yes_no "$(txt "检测到 PostgreSQL ${major}，需要 16+。是否安装 PostgreSQL 16？" "Detected PostgreSQL ${major}. Need 16+. Install PostgreSQL 16?")"; then
			install_postgresql16
			installed_by_script="1"
		else
			die "$(txt "PostgreSQL 版本不足，无法继续" "PostgreSQL version too old")"
		fi
	fi

	if [[ "$installed_by_script" == "1" ]]; then
		local pw pw_sql
		pw="$(prompt_secret_confirm "$(txt "请设置 PostgreSQL 管理员（postgres）密码" "Set PostgreSQL admin (postgres) password")")"
		pw_sql="$(sql_escape_literal "$pw")"
		as_postgres psql -v ON_ERROR_STOP=1 -c "ALTER USER postgres WITH PASSWORD '${pw_sql}';" >/dev/null
	fi
}

timescaledb_installed() {
	ensure_postgres_binaries_on_path
	local pg_major
	pg_major="$(postgres_major_version)"
	[[ "$pg_major" =~ ^[0-9]+$ ]] || return 1
	case "${OS_FAMILY}" in
	debian) [[ -f "/usr/share/postgresql/${pg_major}/extension/timescaledb.control" ]] ;;
	rhel | fedora) [[ -f "/usr/pgsql-${pg_major}/share/extension/timescaledb.control" ]] ;;
	arch)
		need_cmd pg_config || return 1
		local sharedir
		sharedir="$(pg_config --sharedir)" || return 1
		[[ -f "${sharedir}/extension/timescaledb.control" ]]
		;;
	manual)
		need_cmd pg_config || return 1
		local sharedir
		sharedir="$(pg_config --sharedir)" || return 1
		[[ -f "${sharedir}/extension/timescaledb.control" ]]
		;;
	*) return 1 ;;
	esac
}

install_timescaledb_for_postgres() {
	ensure_pkg_prereqs
	local pg_major
	pg_major="$(postgres_major_version)"
	[[ "$pg_major" =~ ^[0-9]+$ ]] && ((pg_major >= 16)) || die "$(txt "无法确定受支持的 PostgreSQL 主版本" "Cannot determine a supported PostgreSQL major version")"

	case "${OS_FAMILY}" in
	debian)
		[[ -n "${APT_REPO_ID}" && -n "${APT_REPO_CODENAME}" ]] || die "$(txt "无法确定 Debian/Ubuntu 基础仓库身份" "Cannot determine the Debian/Ubuntu base repository identity")"
		local keyring="/etc/apt/keyrings/timescale_timescaledb-archive-keyring.gpg"
		install_repo_key \
			https://packagecloud.io/timescale/timescaledb/gpgkey \
			1005FB68604CE9B8F6879CF759F18EDF47F24417,0641009A8366FDE4444A7B62E7391C94080429FF \
			"$keyring" \
			gpg
		as_root bash -c "cat > /etc/apt/sources.list.d/timescale_timescaledb.list <<EOF
deb [signed-by=${keyring}] https://packagecloud.io/timescale/timescaledb/${APT_REPO_ID} ${APT_REPO_CODENAME} main
EOF"
		pkg_update
		pkg_install "timescaledb-2-postgresql-${pg_major}"
		;;
	rhel | fedora)
		local repo_os="fedora"
		if [[ "${OS_FAMILY}" == "rhel" ]]; then
			repo_os="el"
		fi
		local os_major="${OS_VERSION_ID%%.*}"
		local key="/etc/pki/rpm-gpg/timescale-timescaledb.asc"
		install_repo_key \
			https://packagecloud.io/timescale/timescaledb/gpgkey \
			1005FB68604CE9B8F6879CF759F18EDF47F24417,0641009A8366FDE4444A7B62E7391C94080429FF \
			"$key"
		as_root bash -c "cat > /etc/yum.repos.d/timescale_timescaledb.repo <<EOF
[timescale_timescaledb]
name=timescale_timescaledb
baseurl=https://packagecloud.io/timescale/timescaledb/${repo_os}/${os_major}/\$basearch
repo_gpgcheck=1
gpgcheck=0
enabled=1
gpgkey=file://${key}
sslverify=1
metadata_expire=300
EOF"
		pkg_update
		pkg_install "timescaledb-2-postgresql-${pg_major}"
		;;
	arch)
		pkg_install timescaledb
		;;
	*)
		die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")"
		;;
	esac
}

ensure_timescaledb_enabled() {
	local pg_major
	pg_major="$(postgres_major_version)"
	[[ "$pg_major" =~ ^[0-9]+$ ]] || die "$(txt "无法确定 PostgreSQL 主版本" "Cannot determine PostgreSQL major version")"
	if timescaledb_installed; then
		return 0
	fi

	if prompt_yes_no "$(txt "未检测到匹配 PostgreSQL ${pg_major} 的 TimescaleDB，是否安装并配置？" "TimescaleDB for PostgreSQL ${pg_major} was not detected. Install and configure it?")"; then
		install_timescaledb_for_postgres
	else
		die "$(txt "TimescaleDB 未安装，无法继续" "TimescaleDB is required")"
	fi

	if need_cmd timescaledb-tune; then
		as_root timescaledb-tune --quiet --yes
	fi
	restart_postgres_service
}

redis_version() {
	local best="" bin v
	local -a bins=()
	if need_cmd redis-server; then
		bins+=("$(command -v redis-server)")
	fi
	bins+=(/usr/local/bin/redis-server /usr/bin/redis-server /bin/redis-server)

	for bin in "${bins[@]}"; do
		[[ -x "$bin" ]] || continue
		v="$("$bin" --version 2>/dev/null | sed -n 's/.*v=\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\).*/\1/p' | head -n1 || true)"
		[[ -n "$v" ]] || continue
		if [[ -z "$best" ]] || version_ge "$v" "$best"; then
			best="$v"
		fi
	done

	echo "$best"
}

install_redis_via_package_manager() {
	ensure_pkg_prereqs

	case "${OS_FAMILY}" in
	debian)
		pkg_install redis-server || return 1
		;;
	rhel | fedora)
		pkg_install_first_available redis6 redis >/dev/null || return 1
		;;
	arch)
		pkg_install redis || return 1
		;;
	*)
		return 1
		;;
	esac

	if ! id -u redis >/dev/null 2>&1; then
		as_root useradd --system --no-create-home --shell /usr/sbin/nologin redis
	fi
}

install_redis_build_deps() {
	case "${OS_FAMILY}" in
	debian)
		if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
			pkg_install build-essential pkg-config tcl libsystemd-dev
		else
			pkg_install build-essential pkg-config tcl
		fi
		;;
	rhel | fedora)
		if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
			pkg_install gcc make pkgconf-pkg-config tcl systemd-devel
		else
			pkg_install gcc make pkgconf-pkg-config tcl
		fi
		;;
	arch)
		pkg_install base-devel pkgconf tcl
		;;
	*)
		die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")"
		;;
	esac
}

install_redis_from_source() {
	local ver="${1:-8.2.5}"
	if ! confirm_redis_takeover; then
		return 1
	fi

	ensure_pkg_prereqs
	install_redis_build_deps

	(
		local tmp tgz
		tmp="$(mktemp -d)"
		trap 'rm -rf "$tmp"' EXIT

		tgz="${tmp}/redis-${ver}.tar.gz"
		curl -fsSL -o "$tgz" "https://download.redis.io/releases/redis-${ver}.tar.gz"
		tar -C "$tmp" -xzf "$tgz"
		pushd "${tmp}/redis-${ver}" >/dev/null
		if [[ "$SERVICE_MANAGER" == "systemd" ]]; then
			make USE_SYSTEMD=yes -j"$(nproc)"
		else
			make -j"$(nproc)"
		fi
		as_root make install
		hash -r || true
		popd >/dev/null

		if ! id -u redis >/dev/null 2>&1; then
			as_root useradd --system --no-create-home --shell /usr/sbin/nologin redis
		fi
		write_redis_conf
		write_redis_service
		kill_listeners_on_port 6379

		case "$SERVICE_MANAGER" in
		systemd)
			as_root systemctl daemon-reload
			as_root systemctl enable --now redis-server.service
			;;
		none) ;;
		esac
	)
}

ensure_redis_82plus() {
	local want="8.2.3"
	local v
	local tried_pkg=0

	case "${REDIS_INSTALL_METHOD}" in
	package | apt)
		v="$(redis_version)"
		if [[ -z "$v" ]]; then
			if prompt_yes_no "$(txt "未检测到 Redis，是否先尝试使用系统包管理器安装兼容的 Redis？" "Redis not detected. Try installing a compatible Redis via the system package manager?")"; then
				tried_pkg=1
				if ! install_redis_via_package_manager; then
					say "系统包管理器未提供可直接使用的 redis-server。" "No usable redis-server package was found in the system repositories."
				fi
				v="$(redis_version)"
			else
				say "已跳过系统包管理器安装 Redis。" "Skipped Redis installation via the system package manager."
			fi
		fi

		if [[ -n "$v" ]] && version_ge "$v" "$want"; then
			if confirm_redis_takeover; then
				write_redis_conf
				enable_restart_redis_service
			else
				say "保留现有 Redis ${v} 的配置和服务，不执行覆盖。" "Keeping the existing Redis ${v} configuration and service; no replacement was performed."
			fi
			return 0
		fi

		if [[ -n "$v" ]]; then
			say "检测到 Redis ${v}，但需要 >=8.2.3。" "Detected Redis ${v}, but >=8.2.3 is required."
		elif [[ "$tried_pkg" -eq 1 ]]; then
			say "系统包管理器安装后仍未检测到可用的 redis-server。" "A usable redis-server binary is still not available after the package-manager attempt."
		else
			say "未检测到 Redis。" "Redis was not detected."
		fi
		if ! prompt_yes_no "$(txt "是否源码安装/升级 Redis（默认 8.2.5）？" "Install or upgrade Redis from source instead? (default 8.2.5)")"; then
			if [[ -n "$v" ]]; then
				die "$(txt "Redis 版本不足，无法继续" "Redis version is insufficient")"
			fi
			die "$(txt "Redis 未安装，无法继续" "Redis is required")"
		fi
		local target_ver_pkg
		target_ver_pkg="$(prompt_string "$(txt "请输入要源码安装的 Redis 版本" "Redis version to install from source")" "8.2.5")"
		install_redis_from_source "$target_ver_pkg" || die "$(txt "已取消覆盖现有 Redis，无法完成源码安装/升级" "Redis takeover was declined; the source install/upgrade cannot continue")"
		v="$(redis_version)"
		[[ -n "$v" ]] || die "$(txt "Redis 安装失败：未检测到 redis-server" "Redis install failed: redis-server not found")"
		version_ge "$v" "$want" || die "$(txt "Redis 版本仍不足（当前 ${v}，需要 >=8.2.3）" "Redis version still too old (current ${v}, need >=8.2.3)")"
		;;
	source)
		v="$(redis_version)"
		if [[ -n "$v" ]] && version_ge "$v" "$want"; then
			return 0
		fi

		if [[ -z "$v" ]]; then
			if ! prompt_yes_no "$(txt "未检测到 Redis，是否源码安装 Redis（默认 8.2.5）？" "Redis not detected. Install Redis (default 8.2.5) from source?")"; then
				die "$(txt "Redis 未安装，无法继续" "Redis is required")"
			fi
		else
			if ! prompt_yes_no "$(txt "检测到 Redis ${v}，需要 >=8.2.3。是否源码安装/升级？" "Detected Redis ${v}. Need >=8.2.3. Install/upgrade from source?")"; then
				die "$(txt "Redis 版本不足，无法继续" "Redis version too old")"
			fi
		fi

		local target_ver
		target_ver="$(prompt_string "$(txt "请输入要源码安装的 Redis 版本" "Redis version to install (source build)")" "8.2.5")"
		install_redis_from_source "$target_ver" || die "$(txt "已取消覆盖现有 Redis，无法完成源码安装/升级" "Redis takeover was declined; the source install/upgrade cannot continue")"

		v="$(redis_version)"
		[[ -n "$v" ]] || die "$(txt "Redis 安装失败：未检测到 redis-server" "Redis install failed: redis-server not found")"
		version_ge "$v" "$want" || die "$(txt "Redis 版本仍不足（当前 ${v}，需要 >=8.2.3）" "Redis version still too old (current ${v}, need >=8.2.3)")"
		;;
	*)
		die "$(txt "未知 REDIS_INSTALL_METHOD=${REDIS_INSTALL_METHOD}（支持：package/source；apt 仍可作为兼容别名）" "Unknown REDIS_INSTALL_METHOD=${REDIS_INSTALL_METHOD} (supported: package/source; apt remains a compatibility alias)")"
		;;
	esac
}

check_preinstalled_dependencies() {
	local major
	major="$(postgres_major_version)"
	[[ "$major" =~ ^[0-9]+$ ]] && ((major >= 16)) || die "$(txt "手动依赖模式要求已安装 PostgreSQL 16+" "Manual dependency mode requires PostgreSQL 16+")"
	timescaledb_installed || die "$(txt "手动依赖模式要求已安装与当前 PostgreSQL 匹配的 TimescaleDB" "Manual dependency mode requires TimescaleDB for the installed PostgreSQL")"
	as_postgres psql -d postgres -tAc 'SELECT 1' >/dev/null || die "$(txt "无法连接本机 PostgreSQL；请先启动服务" "Cannot connect to local PostgreSQL; start it before continuing")"
}

check_redis_endpoint() {
	local addr="$1" password="${2:-}"
	local checker="${SCRIPT_DIR}/bin/dash"
	[[ -x "$checker" ]] || die "$(txt "安装包缺少可执行的 ${checker}" "The package is missing executable ${checker}")"
	if [[ -z "$password" ]]; then
		"$checker" check-redis --addr "$addr"
		return
	fi

	(
		local password_file
		umask 077
		password_file="$(mktemp -t dash-redis-password-XXXXXX)"
		trap 'rm -f "$password_file"' EXIT
		chmod 0600 "$password_file"
		printf '%s' "$password" >"$password_file"
		"$checker" check-redis --addr "$addr" --password-file "$password_file"
	)
}

validate_ident() {
	local name="$1" what="$2"
	[[ "$name" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]] || die "$(txt "${what} 仅允许字母/数字/下划线，且不能以数字开头：${name}" "${what} must match [a-zA-Z_][a-zA-Z0-9_]* (got: ${name})")"
}

create_db_and_user() {
	local db_user="$1" db_pass="$2" db_name="$3"
	validate_ident "$db_user" "database.user"
	validate_ident "$db_name" "database.name"

	local pass_sql
	pass_sql="$(sql_escape_literal "$db_pass")"

	local role_exists db_exists
	role_exists="$(as_postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${db_user}'" | tr -d '[:space:]')"
	if [[ "$role_exists" != "1" ]]; then
		as_postgres psql -v ON_ERROR_STOP=1 -c "CREATE USER ${db_user} WITH PASSWORD '${pass_sql}';" >/dev/null
	else
		as_postgres psql -v ON_ERROR_STOP=1 -c "ALTER USER ${db_user} WITH PASSWORD '${pass_sql}';" >/dev/null
	fi

	db_exists="$(as_postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${db_name}'" | tr -d '[:space:]')"
	if [[ "$db_exists" != "1" ]]; then
		as_postgres psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db_name} OWNER ${db_user};" >/dev/null
	fi

	as_postgres psql -d "${db_name}" -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS timescaledb;" >/dev/null
}

grant_db_privileges() {
	local db_user="$1" db_name="$2"
	validate_ident "$db_user" "database.user"
	validate_ident "$db_name" "database.name"

	as_postgres psql -d "${db_name}" -v ON_ERROR_STOP=1 <<SQL >/dev/null
GRANT CONNECT ON DATABASE ${db_name} TO ${db_user};
GRANT USAGE ON SCHEMA public TO ${db_user};
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO ${db_user};
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO ${db_user};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ${db_user};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO ${db_user};
SQL
}

render_config_local() {
	local dash_ip="$1" listen_port="$2" public_url="$3" db_user="$4" db_pass="$5" db_name="$6"
	local retention_days="${7:-default}"
	local redis_addr="$8"
	local redis_password="$9"
	local offline_threshold="${10:-14s}"
	local language="${11:-${INSTALL_LANG:-zh}}"
	local trusted_proxies_yaml="${12:-[]}"

	as_root install -d -m 0755 "$CONFIG_DIR"
	[[ -f "$CONFIG_EXAMPLE" ]] || die "$(txt "缺少模板 ${CONFIG_EXAMPLE}（请在安装目录放置 config.example.yaml）" "Missing template ${CONFIG_EXAMPLE}")"

	local listen=":${listen_port}"

	listen="$(one_line "$listen")"
	public_url="$(one_line "$public_url")"
	dash_ip="$(one_line "$dash_ip")"
	db_user="$(one_line "$db_user")"
	db_pass="$(one_line "$db_pass")"
	db_name="$(one_line "$db_name")"
	retention_days="$(one_line "$retention_days")"
	redis_addr="$(one_line "$redis_addr")"
	redis_password="$(one_line "$redis_password")"
	offline_threshold="$(one_line "$offline_threshold")"
	offline_threshold="${offline_threshold:-14s}"
	language="$(one_line "$language")"
	case "${language}" in
	zh | en) ;;
	*) die "$(txt "app.language 只能是 zh 或 en：${language}" "app.language must be zh or en: ${language}")" ;;
	esac
	case "${retention_days}" in
	default | 90 | 180 | 365) ;;
	*) die "$(txt "database.retention_days 只能是 default/90/180/365：${retention_days}" "database.retention_days must be one of default/90/180/365: ${retention_days}")" ;;
	esac
	trusted_proxies_yaml="$(one_line "$trusted_proxies_yaml")"
	trusted_proxies_yaml="${trusted_proxies_yaml:-[]}"

	local retention_line="# retention_days: 45"
	if [[ "${retention_days}" != "default" ]]; then
		retention_line="retention_days: ${retention_days}"
	fi

	local dash_ip_esc listen_esc public_url_esc db_user_esc db_pass_esc db_name_esc retention_line_esc redis_addr_esc redis_password_esc offline_th_esc language_esc
	dash_ip_esc="$(yaml_dq_escape "$dash_ip")"
	listen_esc="$(yaml_dq_escape "$listen")"
	public_url_esc="$(yaml_dq_escape "$public_url")"
	db_user_esc="$(yaml_dq_escape "$db_user")"
	db_pass_esc="$(yaml_dq_escape "$db_pass")"
	db_name_esc="$(yaml_dq_escape "$db_name")"
	retention_line_esc="$(yaml_dq_escape "$retention_line")"
	redis_addr_esc="$(yaml_dq_escape "$redis_addr")"
	redis_password_esc="$(yaml_dq_escape "$redis_password")"
	offline_th_esc="$(yaml_dq_escape "$offline_threshold")"
	language_esc="$(yaml_dq_escape "$language")"

	local jwt_signing_key jwt_signing_key_esc
	jwt_signing_key="$(set +o pipefail; tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
	[[ "${#jwt_signing_key}" -ge 32 ]] || die "JWT signing key generation failed"
	jwt_signing_key_esc="$(yaml_dq_escape "$jwt_signing_key")"

	local tmp
	tmp="$(mktemp -t dash-config-local-XXXXXX)"
	trap "rm -f \"${tmp}\" >/dev/null 2>&1 || true" RETURN

	local placeholder
	local -a required_placeholders=(
		__APP_DASH_IP__
		__APP_LISTEN__
		__APP_PUBLIC_URL__
		__APP_LANGUAGE__
		__APP_NODE_OFFLINE_THRESHOLD__
		__HTTP_TRUSTED_PROXIES__
		__DB_USER__
		__DB_PASS__
		__DB_NAME__
		__DB_RETENTION_DAYS_LINE__
		__REDIS_ADDR__
		__REDIS_PASSWORD__
		__JWT_SIGNING_KEY__
	)
	for placeholder in "${required_placeholders[@]}"; do
		if ! grep -qF "$placeholder" "$CONFIG_EXAMPLE"; then
			die "$(txt "写入配置失败：模板缺少 ${placeholder}（请更新 ${CONFIG_EXAMPLE}）" "Failed to write config: template missing ${placeholder} (please update ${CONFIG_EXAMPLE})")"
		fi
	done

	local r_dash_ip r_listen r_public_url r_language r_offline_th r_http_trusted_proxies r_db_user r_db_pass r_db_name r_db_retention_line r_redis_addr r_redis_password r_jwt_signing_key
	r_dash_ip="$(sed_escape_repl "$dash_ip_esc")"
	r_listen="$(sed_escape_repl "$listen_esc")"
	r_public_url="$(sed_escape_repl "$public_url_esc")"
	r_language="$(sed_escape_repl "$language_esc")"
	r_offline_th="$(sed_escape_repl "$offline_th_esc")"
	r_http_trusted_proxies="$(sed_escape_repl "$trusted_proxies_yaml")"
	r_db_user="$(sed_escape_repl "$db_user_esc")"
	r_db_pass="$(sed_escape_repl "$db_pass_esc")"
	r_db_name="$(sed_escape_repl "$db_name_esc")"
	r_db_retention_line="$(sed_escape_repl "$retention_line_esc")"
	r_redis_addr="$(sed_escape_repl "$redis_addr_esc")"
	r_redis_password="$(sed_escape_repl "$redis_password_esc")"
	r_jwt_signing_key="$(sed_escape_repl "$jwt_signing_key_esc")"

	sed \
		-e "s|__APP_DASH_IP__|${r_dash_ip}|g" \
		-e "s|__APP_LISTEN__|${r_listen}|g" \
		-e "s|__APP_PUBLIC_URL__|${r_public_url}|g" \
		-e "s|__APP_LANGUAGE__|${r_language}|g" \
		-e "s|__APP_NODE_OFFLINE_THRESHOLD__|${r_offline_th}|g" \
		-e "s|__HTTP_TRUSTED_PROXIES__|${r_http_trusted_proxies}|g" \
		-e "s|__DB_USER__|${r_db_user}|g" \
		-e "s|__DB_PASS__|${r_db_pass}|g" \
		-e "s|__DB_NAME__|${r_db_name}|g" \
		-e "s|__DB_RETENTION_DAYS_LINE__|${r_db_retention_line}|g" \
		-e "s|__REDIS_ADDR__|${r_redis_addr}|g" \
		-e "s|__REDIS_PASSWORD__|${r_redis_password}|g" \
		-e "s|__JWT_SIGNING_KEY__|${r_jwt_signing_key}|g" \
		"$CONFIG_EXAMPLE" >"$tmp"

	for placeholder in "${required_placeholders[@]}"; do
		if grep -qF "$placeholder" "$tmp"; then
			die "$(txt "写入配置失败：模板占位符 ${placeholder} 未被替换（请确认 config.example.yaml 版本与安装脚本一致）" "Failed to write config: placeholder ${placeholder} was not replaced (template/script mismatch)")"
		fi
	done

	as_root install -o root -g root -m 0600 "$tmp" "$CONFIG_LOCAL"
	say "已写入配置：${CONFIG_LOCAL}" "Wrote config: ${CONFIG_LOCAL}"
}

is_ip_literal() {
	local host="$1"
	if [[ "$host" =~ ^\[[0-9a-fA-F:]+\]$ ]]; then
		return 0
	fi
	[[ "$host" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

yaml_dq_escape() {
	local s="$1"
	s="${s//\\/\\\\}"
	s="${s//\"/\\\"}"
	printf "%s" "$s"
}

one_line() {
	local s="$1"
	s="${s//$'\r'/}"
	s="${s//$'\n'/}"
	printf "%s" "$s"
}

trim_spaces() {
	local s="$1"
	s="${s#"${s%%[![:space:]]*}"}"
	s="${s%"${s##*[![:space:]]}"}"
	printf "%s" "$s"
}

admin_password_valid() {
	local LC_ALL=C
	local s="$1"
	[[ -n "$s" ]] || return 1
	[[ "$s" =~ ^[!-~]+$ ]] || return 1
	(( ${#s} >= 8 ))
}

run_db_migrations() {
	local config_path="$1"
	if [[ -x "$BIN_PATH" ]]; then
		as_root env DASH_HOME="$INSTALL_DIR" "$BIN_PATH" migrate -config "$config_path"
		return 0
	fi
	die "$(txt "无法执行 migrate：未找到可执行文件 ${BIN_PATH}" "Cannot run migrate: executable not found: ${BIN_PATH}")"
}

normalize_public_url() {
	local in="$1"

	in="${in#"${in%%[![:space:]]*}"}"
	in="${in%"${in##*[![:space:]]}"}"

	if [[ -z "$in" ]]; then
		echo ""
		return 0
	fi

	if [[ "$in" =~ ^[a-zA-Z][a-zA-Z0-9+.-]*:// ]]; then
		echo "$in"
		return 0
	fi

	local hostport="${in%%/*}"
	local ip_probe="$hostport"
	if [[ "$hostport" == \[* ]]; then
		ip_probe="${hostport%%]*}"
		ip_probe="${ip_probe}]"
	elif [[ "$hostport" == *:*:* && "$hostport" =~ ^[0-9a-fA-F:]+$ ]]; then
		in="[${hostport}]${in:${#hostport}}"
		ip_probe="[${hostport}]"
	else
		ip_probe="${hostport%%:*}"
	fi

	local scheme="http"
	if ! is_ip_literal "$ip_probe"; then
		scheme="https"
	fi

	local out="${scheme}://${in}"
	if [[ "$in" != */* ]]; then
		out="${out}/"
	fi
	echo "$out"
}

warn_if_domain_public_url() {
	local public_url="$1"
	local hostport="${public_url#*://}"
	hostport="${hostport%%/*}"

	local host="$hostport"
	if [[ "$hostport" == \[* ]]; then
		host="${hostport%%]*}"
		host="${host}]"
	else
		host="${hostport%%:*}"
	fi
	[[ -n "$host" ]] || return 0
	if ! is_ip_literal "$host"; then
		say "提示：public_url 看起来是域名（${host}），请确保已配置前置 Web 服务器（如 Nginx/Caddy）做反代与 HTTPS。" "NOTE: public_url looks like a domain (${host}). You likely need a reverse proxy (Nginx/Caddy) for HTTPS and forwarding."
	fi
}

require_root_public_url() {
	local public_url="$1"
	if [[ ! "$public_url" =~ ^([hH][tT][tT][pP]|[hH][tT][tT][pP][sS]):// ]]; then
		die "$(txt "public_url 只支持 HTTP 或 HTTPS" "public_url supports only HTTP or HTTPS")"
	fi
	local rest="${public_url#*://}"
	[[ -n "$rest" ]] || die "$(txt "public_url 缺少主机名" "public_url is missing a host")"
	[[ "$rest" != *"@"* ]] || die "$(txt "public_url 不支持用户信息" "public_url must not include user information")"
	[[ "$rest" != *"?"* ]] || die "$(txt "public_url 不支持查询参数" "public_url must not include a query")"
	[[ "$rest" != *"#"* ]] || die "$(txt "public_url 不支持片段" "public_url must not include a fragment")"

	local hostport="${rest%%/*}"
	local path_part="${rest:${#hostport}}"
	[[ -n "$hostport" ]] || die "$(txt "public_url 缺少主机名" "public_url is missing a host")"
	local host="$hostport"
	if [[ "$hostport" == \[* ]]; then
		[[ "$hostport" == *"]"* ]] || die "$(txt "public_url 的 IPv6 主机格式无效" "public_url has an invalid IPv6 host")"
		host="${hostport#\[}"
		host="${host%%]*}"
	else
		host="${hostport%%:*}"
	fi
	[[ -n "$host" ]] || die "$(txt "public_url 缺少主机名" "public_url is missing a host")"
	[[ -z "$path_part" || "$path_part" == "/" ]] || die "$(txt "public_url 不支持路径前缀（${path_part}）。请改为根路径 URL，例如 http://127.0.0.1:8080/ 或 https://dash.example.com/" "public_url does not support path prefixes (${path_part}). Use a root URL such as http://127.0.0.1:8080/ or https://dash.example.com/")"
}

stage_app_files_from_cwd() {
	# Full installation always consumes the files beside this script. A release
	# here is only the local immutable directory selected by current.
	need_cmd mv || die "$(txt "缺少命令：mv" "Missing command: mv")"
	mv --version 2>/dev/null | grep -q 'GNU coreutils' || die "$(txt "安装器需要 GNU coreutils 的 mv 以保证原子切换" "The installer requires GNU coreutils mv for atomic cutover")"
	[[ -f "${SCRIPT_DIR}/bin/dash" ]] || die "$(txt "未找到可执行文件 ${SCRIPT_DIR}/bin/dash" "Missing executable: ${SCRIPT_DIR}/bin/dash")"
	[[ -d "${SCRIPT_DIR}/dist" && -d "${SCRIPT_DIR}/deploy" && -d "${SCRIPT_DIR}/configs" ]] || die "$(txt "安装包缺少 dist、deploy 或 configs 目录" "Package is missing the dist, deploy, or configs directory")"

	local stage
	stage="$(as_root mktemp -d "${INSTALL_DIR}.stage.XXXXXX")"
	as_root chown "$(id -u):$(id -g)" "$stage"
	if ! as_root cp -a \
		"${SCRIPT_DIR}/bin" \
		"${SCRIPT_DIR}/configs" \
		"${SCRIPT_DIR}/dist" \
		"${SCRIPT_DIR}/deploy" \
		"${SCRIPT_DIR}/install_dash_linux.sh" \
		"${SCRIPT_DIR}/update_dash_linux.sh" \
		"${SCRIPT_DIR}/release.env" \
		"$stage/"; then
		as_root rm -rf "$stage"
		die "$(txt "暂存安装包失败" "Failed to stage package files")"
	fi
	[[ -f "${stage}/bin/dash" ]] || {
		as_root rm -rf "$stage"
		die "$(txt "暂存后未找到 Dash 二进制" "Dash binary is missing from the staged package")"
	}
	as_root chmod 0755 "${stage}/bin/dash"
	if ! as_root "${stage}/bin/dash" --version >/dev/null 2>&1; then
		as_root rm -rf "$stage"
		die "$(txt "Dash 二进制无法在当前系统运行；Alpine 需要静态或 musl 兼容产物" "The Dash binary cannot run on this system; Alpine requires a static or musl-compatible artifact")"
	fi
	printf '%s\n' "$stage"
}

prepare_staged_release() {
	local stage="$1" version release
	version="$(as_root "${stage}/bin/dash" --version)" || return
	version="${version//$'\r'/}"
	version="${version//$'\n'/}"
	[[ "$version" =~ ^[0-9A-Za-z][0-9A-Za-z.+-]{0,127}$ ]] || return 1
	need_cmd mv || return 1
	mv --version 2>/dev/null | grep -q 'GNU coreutils' || return 1

	release="${RELEASES_DIR}/${version}-$(date -u +%Y%m%dT%H%M%SZ)-$$"
	as_root install -d -m 0755 "$INSTALL_DIR" "$RELEASES_DIR" "${INSTALL_DIR}/configs" || return
	[[ ! -e "$release" ]] || return 1
	as_root mv "$stage" "$release" || return
	if ! as_root chown -R root:root "$release" ||
		! as_root chmod 0755 "${release}/bin/dash" ||
		! as_root find "$release" -maxdepth 1 -type f -name '*.sh' -exec chmod 0755 {} +; then
		as_root rm -rf "$release"
		return 1
	fi
	printf '%s\n' "$release"
}

switch_current_target() {
	local target="$1" link
	[[ ! -e "$CURRENT_LINK" || -L "$CURRENT_LINK" ]] || return 1
	link="${INSTALL_DIR}/.current.$$.$RANDOM"
	as_root rm -f "$link" || return
	as_root ln -s "$target" "$link" || return
	as_root mv -Tf "$link" "$CURRENT_LINK" || {
		as_root rm -f "$link"
		return 1
	}
}

switch_current_release() {
	local release="$1"
	[[ -d "$release" ]] || return 1
	switch_current_target "releases/$(basename "$release")"
}

install_compat_alias() {
	local path="$1" target="$2" link
	link="${path}.new.$$.$RANDOM"
	as_root rm -f "$link" || return
	as_root ln -s "$target" "$link" || return
	if [[ -d "$path" && ! -L "$path" ]]; then
		if ! as_root rm -rf "$path"; then
			as_root rm -f "$link"
			return 1
		fi
	fi
	as_root mv -Tf "$link" "$path" || {
		as_root rm -f "$link"
		return 1
	}
}

install_compat_aliases() {
	as_root install -d -m 0755 "${INSTALL_DIR}/configs" || return

	# Compatibility bridge for the legacy flat layout. Remove these aliases
	# after the next breaking-release migration window.
	install_compat_alias "${INSTALL_DIR}/bin" "current/bin" || return
	install_compat_alias "${INSTALL_DIR}/dist" "current/dist" || return
	install_compat_alias "${INSTALL_DIR}/deploy" "current/deploy" || return
	install_compat_alias "${INSTALL_DIR}/install_dash_linux.sh" "current/install_dash_linux.sh" || return
	install_compat_alias "${INSTALL_DIR}/update_dash_linux.sh" "current/update_dash_linux.sh" || return
	install_compat_alias "${INSTALL_DIR}/release.env" "current/release.env" || return

	install_compat_alias "${INSTALL_DIR}/configs/config.example.yaml" "../current/configs/config.example.yaml" || return
}

backup_legacy_install() {
	local backup_dir="$1" rel
	as_root install -d -m 0700 "$backup_dir" || return
	for rel in bin configs dist deploy install_dash_linux.sh update_dash_linux.sh release.env; do
		[[ -e "${INSTALL_DIR}/${rel}" || -L "${INSTALL_DIR}/${rel}" ]] || continue
		as_root cp -a "${INSTALL_DIR}/${rel}" "$backup_dir/" || return
	done
}

restore_legacy_install() {
	local backup_dir="$1"
	[[ -d "$backup_dir" ]] || return 1
	as_root install -d -m 0755 "$INSTALL_DIR" || return
	as_root rm -rf \
		"${INSTALL_DIR}/bin" \
		"${INSTALL_DIR}/configs" \
		"${INSTALL_DIR}/dist" \
		"${INSTALL_DIR}/deploy" \
		"$CURRENT_LINK" || return
	as_root rm -f \
		"${INSTALL_DIR}/install_dash_linux.sh" \
		"${INSTALL_DIR}/update_dash_linux.sh" \
		"${INSTALL_DIR}/release.env" \
		"$LAYOUT_MARKER" || return
	as_root cp -a "${backup_dir}/." "$INSTALL_DIR/" || return
	tighten_sensitive_file_permissions || return
}

legacy_install_present() {
	local rel
	for rel in bin/dash dist deploy install_dash_linux.sh update_dash_linux.sh release.env configs/config.local.yaml configs/config.yaml configs/config.example.yaml; do
		[[ -e "${INSTALL_DIR}/${rel}" || -L "${INSTALL_DIR}/${rel}" ]] && return 0
	done
	return 1
}

prepare_install_recovery() {
	[[ ! -e "$CURRENT_LINK" || -L "$CURRENT_LINK" ]] || return 1
	INSTALL_RECOVERY_ROOT="$(as_root mktemp -d "${INSTALL_DIR}.recovery.XXXXXX")" || return
	as_root chown "$(id -u):$(id -g)" "$INSTALL_RECOVERY_ROOT" || return
	INSTALL_PREVIOUS_LAYOUT="fresh"
	INSTALL_PREVIOUS_TARGET=""
	INSTALL_PREVIOUS_MARKER="0"
	INSTALL_PREVIOUS_CONFIG="0"
	INSTALL_SYSTEMD_WAS_ACTIVE="0"

	if [[ -L "$CURRENT_LINK" ]]; then
		INSTALL_PREVIOUS_LAYOUT="release"
		INSTALL_PREVIOUS_TARGET="$(readlink "$CURRENT_LINK")"
		[[ -f "$LAYOUT_MARKER" ]] && INSTALL_PREVIOUS_MARKER="1"
		if [[ -f "$CONFIG_LOCAL" ]]; then
			as_root cp -a "$CONFIG_LOCAL" "${INSTALL_RECOVERY_ROOT}/config.local.yaml" || return
			INSTALL_PREVIOUS_CONFIG="1"
		fi
	elif legacy_install_present; then
		INSTALL_PREVIOUS_LAYOUT="legacy"
		backup_legacy_install "${INSTALL_RECOVERY_ROOT}/legacy" || return
	else
		as_root install -d -m 0700 "${INSTALL_RECOVERY_ROOT}/legacy" || return
	fi

	if systemd_available && [[ -f "$SERVICE_FILE" ]] && as_root systemctl is-active --quiet "${APP}.service"; then
		INSTALL_SYSTEMD_WAS_ACTIVE="1"
	fi
	INSTALL_ROLLBACK_READY="1"
}

restore_release_install() {
	[[ -n "$INSTALL_PREVIOUS_TARGET" ]] || return 1
	switch_current_target "$INSTALL_PREVIOUS_TARGET" || return
	install_compat_aliases || return
	if [[ "$INSTALL_PREVIOUS_MARKER" == "1" ]]; then
		as_root touch "$LAYOUT_MARKER" || return
	else
		as_root rm -f "$LAYOUT_MARKER" || return
	fi
	if [[ "$INSTALL_PREVIOUS_CONFIG" == "1" ]]; then
		as_root install -o root -g root -m 0600 "${INSTALL_RECOVERY_ROOT}/config.local.yaml" "$CONFIG_LOCAL" || return
	else
		as_root rm -f "$CONFIG_LOCAL" || return
	fi
}

rollback_install() {
	local restored="true"
	say_err "安装提交前失败，正在恢复原文件入口和服务。" "Installation failed before commit; restoring the previous file entrypoint and service."
	case "$INSTALL_PREVIOUS_LAYOUT" in
		release) restore_release_install || restored="false" ;;
		legacy|fresh) restore_legacy_install "${INSTALL_RECOVERY_ROOT}/legacy" || restored="false" ;;
		*) restored="false" ;;
	esac
	if [[ "$restored" == "true" && -n "$INSTALL_RELEASE" ]] && ! install_release_is_current; then
		as_root rm -rf "$INSTALL_RELEASE" || restored="false"
		[[ "$restored" != "true" ]] || INSTALL_RELEASE=""
	fi
	if [[ "$restored" == "true" && "$INSTALL_SYSTEMD_WAS_ACTIVE" == "1" ]]; then
		as_root systemctl start "${APP}.service" || restored="false"
	fi
	if [[ "$restored" == "true" ]]; then
		as_root rm -rf "$INSTALL_RECOVERY_ROOT"
		INSTALL_RECOVERY_ROOT=""
		return 0
	fi
	return 1
}

activate_release() {
	local release="$1"
	switch_current_release "$release" || return
	install_compat_aliases || return
	as_root touch "$LAYOUT_MARKER" || return
}

prune_inactive_releases() {
	[[ -L "$CURRENT_LINK" && -d "$RELEASES_DIR" ]] || return 0
	local current entry entries
	current="$(readlink "$CURRENT_LINK")"
	current="${current##*/}"
	entries="$(mktemp)" || return
	if ! find "$RELEASES_DIR" -mindepth 1 -maxdepth 1 -type d -print0 >"$entries"; then
		rm -f "$entries"
		return 1
	fi
	while IFS= read -r -d '' entry; do
		[[ "$(basename "$entry")" == "$current" ]] && continue
		if ! as_root rm -rf "$entry"; then
			rm -f "$entries"
			return 1
		fi
	done <"$entries"
	rm -f "$entries" || return
}

prepare_staged_app_files() {
	local stage="$1" release=""
	[[ -d "$stage" && -x "${stage}/bin/dash" ]] || die "$(txt "暂存安装包无效" "Invalid staged package")"
	INSTALL_RELEASE=""
	if ! release="$(prepare_staged_release "$stage")"; then
		as_root rm -rf "$stage"
		die "$(txt "准备不可变 release 目录失败；线上文件和进程未改动" "Failed to prepare the immutable release directory; live files and processes were not changed")"
	fi
	INSTALL_RELEASE="$release"
}

install_staged_app_files() {
	[[ -n "$INSTALL_RELEASE" && -d "$INSTALL_RELEASE" ]] || die "$(txt "尚未准备可切换的 release" "No prepared release is available for cutover")"
	activate_release "$INSTALL_RELEASE" || die "$(txt "强制覆盖 Dash 文件入口失败" "Failed to replace the managed Dash file entrypoints")"

	[[ -x "$BIN_PATH" ]] || die "$(txt "安装后未找到可执行文件 ${BIN_PATH}" "Missing executable after install: ${BIN_PATH}")"
}

tighten_sensitive_file_permissions() {
	if [[ -f "$CONFIG_LOCAL" ]]; then
		as_root chown root:root "$CONFIG_LOCAL"
		as_root chmod 0600 "$CONFIG_LOCAL"
	fi
	if [[ -f "$NOTIFY_CONFIG_KEY" ]]; then
		as_root chown root:root "$NOTIFY_CONFIG_KEY"
		as_root chmod 0600 "$NOTIFY_CONFIG_KEY"
	fi
	if [[ -f "$SERVICE_FILE" ]]; then
		as_root chown root:root "$SERVICE_FILE"
		as_root chmod 0600 "$SERVICE_FILE"
	fi
	if [[ -f "$MANUAL_RUN_FILE" ]]; then
		as_root chown root:root "$MANUAL_RUN_FILE"
		as_root chmod 0700 "$MANUAL_RUN_FILE"
	fi
}

write_systemd_service() {
	local admin_password="$1"
	admin_password="$(one_line "$admin_password")"
	local pwd_escaped
	pwd_escaped="$(systemd_escape_env_value "$admin_password")"
	local pg_major pg_units="postgresql.service"
	pg_major="$(postgres_major_version)"
	if [[ "$pg_major" =~ ^[0-9]+$ ]]; then
		pg_units+=" postgresql-${pg_major}.service"
	fi
	local tmp
	tmp="$(mktemp)"

	cat >"$tmp" <<EOF
[Unit]
Description=Dash Server Monitor
After=network-online.target ${pg_units}
Wants=network-online.target
ConditionPathExists=!${INSTALL_DIR}/runtime/dash-update/update.block

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${INSTALL_DIR}

Environment="DASH_HOME=${INSTALL_DIR}"
Environment="monitor_dash_pwd=${pwd_escaped}"

ExecStart=${BIN_PATH}

Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF
	as_root install -m 0600 "$tmp" "$SERVICE_FILE"
	rm -f "$tmp"

	tighten_sensitive_file_permissions
	as_root systemctl daemon-reload
}

write_manual_runner() {
	local admin_password="$1"
	admin_password="$(one_line "$admin_password")"
	local pwd_quoted
	pwd_quoted="$(shell_quote_arg "$admin_password")"
	local tmp
	tmp="$(mktemp)"
	cat >"$tmp" <<EOF
#!/usr/bin/env bash
set -euo pipefail
export DASH_HOME=$(shell_quote_arg "$INSTALL_DIR")
export monitor_dash_pwd=${pwd_quoted}
cd $(shell_quote_arg "$INSTALL_DIR")
exec $(shell_quote_arg "$BIN_PATH")
EOF
	as_root install -m 0700 "$tmp" "$MANUAL_RUN_FILE"
	rm -f "$tmp"
	tighten_sensitive_file_permissions
}

service_install() {
	case "$SERVICE_MANAGER" in
	systemd) write_systemd_service "$1" ;;
	none) write_manual_runner "$1" ;;
	esac
}

service_enable() {
	case "$SERVICE_MANAGER" in
	systemd) as_root systemctl enable "${APP}.service" ;;
	none) return 0 ;;
	esac
}

service_start() {
	case "$SERVICE_MANAGER" in
	systemd) as_root systemctl start "${APP}.service" ;;
	none) return 0 ;;
	esac
}

service_status() {
	case "$SERVICE_MANAGER" in
	systemd) say "状态：systemctl status ${APP}.service；日志：journalctl -u ${APP}.service -f" "Status: systemctl status ${APP}.service; logs: journalctl -u ${APP}.service -f" ;;
	none) say "手动模式未启动服务。运行：${MANUAL_RUN_FILE}" "Manual mode did not start a service. Run: ${MANUAL_RUN_FILE}" ;;
	esac
}

stop_installed_services() {
	if systemd_available && [[ -f "$SERVICE_FILE" ]]; then
		if ! as_root systemctl stop "${APP}.service"; then
			die "$(txt "停止已安装的 ${APP} 服务失败" "Failed to stop the installed ${APP} service")"
		fi
	fi
	stop_manual_processes
}

install_release_is_current() {
	[[ -n "$INSTALL_RELEASE" && -L "$CURRENT_LINK" ]] || return 1
	[[ "$(readlink "$CURRENT_LINK")" == "releases/$(basename "$INSTALL_RELEASE")" ]]
}

finish_dash_install() {
	local status=$?
	trap - EXIT
	set +e
	if ((status != 0)) && [[ "$INSTALL_ROLLBACK_READY" == "1" && "$INSTALL_MIGRATION_STARTED" != "1" ]]; then
		if ! rollback_install; then
			say_err "自动恢复失败；恢复文件保留在 ${INSTALL_RECOVERY_ROOT}" "Automatic recovery failed; recovery files were preserved at ${INSTALL_RECOVERY_ROOT}"
		fi
	elif ((status != 0)) && [[ -n "$INSTALL_RELEASE" ]] && ! install_release_is_current; then
		as_root rm -rf "$INSTALL_RELEASE"
		INSTALL_RELEASE=""
	fi
	if ((status != 0)) && [[ "$INSTALL_ROLLBACK_READY" != "1" && "$INSTALL_MIGRATION_STARTED" != "1" && -n "$INSTALL_RECOVERY_ROOT" ]]; then
		as_root rm -rf "$INSTALL_RECOVERY_ROOT"
		INSTALL_RECOVERY_ROOT=""
	fi
	if ((status != 0)) && [[ "$INSTALL_MIGRATION_STARTED" == "1" && -n "$INSTALL_RECOVERY_ROOT" ]]; then
		say_err "数据库迁移已开始，不能自动恢复旧程序；恢复文件保留在 ${INSTALL_RECOVERY_ROOT}" "Database migration had started, so the old program was not restored; recovery files were preserved at ${INSTALL_RECOVERY_ROOT}"
	fi
	if ((status == 0)) && ! prune_inactive_releases; then
		say_err "安装已完成，但旧 release 清理失败" "Installation completed, but old release cleanup failed"
	fi
	if ((status == 0)) && [[ -n "$INSTALL_RECOVERY_ROOT" ]]; then
		as_root rm -rf "$INSTALL_RECOVERY_ROOT"
		INSTALL_RECOVERY_ROOT=""
	fi
	exit "$status"
}

cleanup_other_service_managers() {
	case "$SERVICE_MANAGER" in
	systemd)
		as_root rm -f "$MANUAL_RUN_FILE"
		;;
	none)
		if systemd_available; then
			as_root systemctl disable --now "${APP}.service" >/dev/null 2>&1 || true
		fi
		as_root rm -f "$SERVICE_FILE"
		;;
	esac
}

main() {
	local stage=""
	select_service_manager
	ensure_process_control

	detect_os

	if [[ "$OS_FAMILY" != "manual" && "$SERVICE_MANAGER" != "none" ]]; then
		if [[ -z "${PKG_MANAGER}" ]] || ! need_cmd "${PKG_MANAGER}"; then
			die "$(txt "未检测到系统包管理器（当前需要 ${PKG_MANAGER_LABEL:-unknown}）" "System package manager not found (expected ${PKG_MANAGER_LABEL:-unknown})")"
		fi
	fi

	if [[ "$SERVICE_MANAGER" != "none" ]]; then
		enable_time_sync
	fi

	say "1) 检测并准备数据库依赖（PostgreSQL 16+ / 匹配主版本的 TimescaleDB；Redis 地址将在配置后校验；模式：${PKG_MANAGER_LABEL}）" "1) Checking database dependencies (PostgreSQL 16+ / matching TimescaleDB major; the configured Redis endpoint is validated next; mode: ${PKG_MANAGER_LABEL})"
	if [[ "$OS_FAMILY" == "manual" || "$SERVICE_MANAGER" == "none" ]]; then
		check_preinstalled_dependencies
	else
		ensure_postgresql16_and_password
		ensure_timescaledb_enabled
		ensure_redis_82plus
	fi

	say "2) 交互式生成配置 ${CONFIG_LOCAL}" "2) Interactive configuration: ${CONFIG_LOCAL}"
	local dash_ip listen_port public_url db_user db_pass db_name retention_days redis_addr redis_password offline_threshold language admin_pwd trusted_proxies_yaml

	while true; do
	while true; do
		dash_ip="$(prompt_string "$(txt "请输入 Dash 服务端 IP（回车查看本机IP：ip addr）" "Dash server IP (press Enter to show local IPs via: ip addr)")")"
		dash_ip="$(trim_spaces "$(one_line "$dash_ip")")"
		if [[ -n "$dash_ip" ]]; then
			break
		fi
		say "未输入 IP，显示本机网络信息（ip addr）：" "No IP entered; showing local network info (ip addr):"
		if need_cmd ip; then
			ip addr || true
		else
			say_err "未找到 ip 命令（iproute2），无法显示本机 IP" "ip command not found (iproute2)."
		fi
	done

	listen_port="$(prompt_string "$(txt "请输入监听端口" "Listen port")" "8080")"
	listen_port="${listen_port#:}"
	[[ "$listen_port" =~ ^[0-9]+$ ]] || die "$(txt "监听端口必须是数字：${listen_port}" "Listen port must be numeric (got: ${listen_port})")"
	((10#$listen_port >= 1 && 10#$listen_port <= 65535)) || die "$(txt "监听端口必须在 1 到 65535 之间：${listen_port}" "Listen port must be between 1 and 65535 (got: ${listen_port})")"
	public_url="$(prompt_string "$(txt "请输入 public_url（用于生成安装脚本/外网访问，必须是根路径 URL）" "public_url (external access URL, root URL only)")" "http://127.0.0.1:${listen_port}/")"
	public_url="$(normalize_public_url "$public_url")"
	[[ -n "$public_url" ]] || die "$(txt "public_url 不能为空" "public_url is required")"
	require_root_public_url "$public_url"
	trusted_proxies_yaml="[]"
	if prompt_yes_no "$(txt "是否通过本机反向代理（如同机 Nginx/Caddy/Traefik）对外暴露 Dash？IP 部署请选择否；启用后仅信任来自本机代理的转发头。" "Is Dash exposed through a local reverse proxy on the same host (for example Nginx/Caddy/Traefik)? Choose no for a direct IP deployment; enabling this trusts forwarded headers only from that local proxy.")" "N"; then
		trusted_proxies_yaml='["127.0.0.1/32", "::1/128"]'
	fi
	db_user="$(prompt_string "$(txt "请输入数据库账号（database.user）" "database.user")" "monitor")"
	db_pass="$(prompt_secret_confirm "$(txt "请输入数据库密码（database.password）" "database.password")")"
	db_name="$(prompt_string "$(txt "请输入数据库名（database.name）" "database.name")" "monitor")"
	retention_days="$(prompt_retention_days 1)"
	while true; do
		redis_addr="$(prompt_string "$(txt "请输入 Redis 地址（redis.addr）" "redis.addr")" "127.0.0.1:6379")"
		redis_addr="$(trim_spaces "$(one_line "$redis_addr")")"
		redis_password="$(prompt_secret_optional "$(txt "请输入 Redis 密码（redis.password；无密码直接回车）" "redis.password (press Enter when authentication is disabled)")")"
		if check_redis_endpoint "$redis_addr" "$redis_password"; then
			break
		fi
		say_err "无法使用该 Redis 地址，请确认服务可达、允许 PING/INFO server，且版本不低于 6.2.0。" "Cannot use this Redis endpoint. Ensure it is reachable, permits PING and INFO server, and runs Redis 6.2.0 or newer."
	done
	offline_threshold="$(prompt_string "$(txt "请输入离线判定阈值（app.node_offline_threshold，例如：14s/30s/1m）" "app.node_offline_threshold (e.g. 14s/30s/1m)")" "14s")"
	language="$(prompt_language)"

	echo ""
	say "2.1) 配置摘要（写入前确认）" "2.1) Configuration summary (confirm before writing)"

	print_config_summary "$dash_ip" "$listen_port" "$public_url" "$db_user" "$db_pass" "$db_name" "$retention_days" "$redis_addr" "$redis_password" "$offline_threshold" "$language" "$trusted_proxies_yaml"
	echo ""
	if prompt_yes_no "$(txt "确认写入以上配置并继续？" "Write the configuration above and continue?")" "Y"; then
		break
	fi
	say "已取消本次配置，请重新填写。" "Configuration was not written; please enter it again."
	echo ""
	done

	say "3) 初始化数据库用户和数据库" "3) Initializing database user and database"
	create_db_and_user "$db_user" "$db_pass" "$db_name"

	say "4) 配置运行方式：${SERVICE_MANAGER}" "4) Configuring runtime mode: ${SERVICE_MANAGER}"
	while true; do
		admin_pwd="$(prompt_secret_confirm "$(txt "请设置 Dash 管理员登录密码（环境变量 monitor_dash_pwd）" "Dash admin password (env monitor_dash_pwd)")")"
		admin_pwd="$(trim_spaces "$admin_pwd")"
		if ! admin_password_valid "$admin_pwd"; then
			say_err "Dash 管理员密码至少需要 8 个字符，仅允许大小写英文、数字和常见符号；会自动忽略输入前后的空格；中间不能包含空格或其他空白字符。" "Dash admin password must contain at least 8 characters using only ASCII letters, digits, and common symbols; leading and trailing spaces are ignored; inner whitespace is not allowed."
			continue
		fi
		break
	done

	say "5) 安装 Dash 文件到 ${INSTALL_DIR}" "5) Installing Dash files into ${INSTALL_DIR}"
	stage="$(stage_app_files_from_cwd)"
	prepare_staged_app_files "$stage"
	prepare_install_recovery || die "$(txt "准备旧安装恢复现场失败；线上文件和进程未改动" "Failed to prepare recovery for the previous installation; live files and processes were not changed")"
	stop_installed_services
	install_staged_app_files
	render_config_local "$dash_ip" "$listen_port" "$public_url" "$db_user" "$db_pass" "$db_name" "$retention_days" "$redis_addr" "$redis_password" "$offline_threshold" "$language" "$trusted_proxies_yaml"
	INSTALL_MIGRATION_STARTED="1"
	run_db_migrations "$CONFIG_LOCAL"
	grant_db_privileges "$db_user" "$db_name"

	cleanup_other_service_managers
	service_install "$admin_pwd"
	if [[ "$SERVICE_MANAGER" != "none" ]]; then
		service_enable
		service_start
	fi

	warn_if_domain_public_url "$public_url"
	service_status
}

parse_args "$@"
choose_install_lang
trap finish_dash_install EXIT
main
