#!/usr/bin/env bash
set -euo pipefail

APP="dash"

INSTALL_DIR="/opt/Ithiltir-dash"
BIN_DIR="${INSTALL_DIR}/bin"
BIN_PATH="${BIN_DIR}/dash"

CONFIG_DIR="${INSTALL_DIR}/configs"
CONFIG_EXAMPLE="${CONFIG_DIR}/config.example.yaml"
CONFIG_LOCAL="${CONFIG_DIR}/config.local.yaml"

SERVICE_FILE="/etc/systemd/system/${APP}.service"

REDIS_INSTALL_METHOD="${REDIS_INSTALL_METHOD:-package}"

OS_ID=""
OS_VERSION_ID=""
OS_VERSION_CODENAME=""
OS_FAMILY=""
PKG_MANAGER=""
PKG_MANAGER_LABEL=""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

INSTALL_LANG="${INSTALL_LANG:-}"

usage() {
	cat <<EOF
Usage: $0 [--lang zh|en]
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
	local dash_ip="$1" listen_port="$2" public_url="$3" db_user="$4" db_pass="$5" db_name="$6" retention_days="$7" redis_addr="$8" offline_threshold="$9" language="${10}" trusted_proxies="${11}"
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

systemd_unit_exists() {
	need_cmd systemctl || return 1
	systemctl cat "${APP}.service" >/dev/null 2>&1
}

enable_time_sync() {
	say "启用系统时间同步（NTP，失败不影响安装）" "Enabling system time sync (NTP; non-fatal)"

	if need_cmd timedatectl && as_root timedatectl set-ntp true >/dev/null 2>&1; then
		say "系统时间同步已启用。" "System time sync is enabled."
		return 0
	fi

	local unit
	for unit in systemd-timesyncd.service chronyd.service ntpd.service ntp.service; do
		if as_root systemctl enable --now "$unit" >/dev/null 2>&1; then
			say "系统时间同步服务已启动：${unit}" "System time sync service started: ${unit}"
			return 0
		fi
	done

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

kill_listeners_on_port() {
	local port="$1"
	local pids
	pids="$(find_listening_pids_on_port "$port")"
	if [[ -z "$pids" ]]; then
		return 0
	fi
	if port_has_redis_listener "$port" "$pids"; then
		say "检测到端口 ${port} 已由 Redis 监听，跳过结束进程。" "Port ${port} is already owned by Redis; skipping listener termination."
		return 10
	fi

	say "检测到端口 ${port} 已被占用，尝试结束占用进程：${pids}" "Port ${port} is in use; terminating listeners: ${pids}"
	as_root systemctl stop redis-server.service >/dev/null 2>&1 || true
	as_root systemctl stop redis.service >/dev/null 2>&1 || true

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

sed_escape_repl() {
	local s="$1"
	s="${s//\\/\\\\}"
	s="${s//&/\\&}"
	s="${s//|/\\|}"
	printf "%s" "$s"
}

write_redis_conf() {
	as_root install -d -m 0755 /etc/redis /var/lib/redis /var/log/redis
	as_root chown -R redis:redis /var/lib/redis /var/log/redis >/dev/null 2>&1 || true

	if [[ -f /etc/redis/redis.conf ]]; then
		local bak="/etc/redis/redis.conf.bak.$(date +%Y%m%d%H%M%S)"
		as_root cp -f /etc/redis/redis.conf "$bak"
	fi

	as_root bash -c "cat > /etc/redis/redis.conf <<'EOF'

bind 127.0.0.1 -::1
protected-mode yes
port 6379
tcp-backlog 511
timeout 0
tcp-keepalive 300

daemonize no
supervised systemd
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
	OS_VERSION_CODENAME="${VERSION_CODENAME:-${UBUNTU_CODENAME:-${DEBIAN_CODENAME:-}}}"

	case "${OS_ID}" in
	debian)
		local major="${OS_VERSION_ID%%.*}"
		[[ "$major" =~ ^[0-9]+$ ]] || die "$(txt "无法解析 Debian VERSION_ID=${OS_VERSION_ID:-}" "Cannot parse Debian VERSION_ID=${OS_VERSION_ID:-}")"
		((major >= 11)) || die "$(txt "仅支持 Debian 11+，当前 VERSION_ID=${OS_VERSION_ID}" "Only Debian 11+ is supported (current VERSION_ID=${OS_VERSION_ID})")"
		OS_FAMILY="debian"
		PKG_MANAGER="apt-get"
		PKG_MANAGER_LABEL="apt-get"
		;;
	ubuntu)
		local major="${OS_VERSION_ID%%.*}"
		[[ "$major" =~ ^[0-9]+$ ]] || die "$(txt "无法解析 Ubuntu VERSION_ID=${OS_VERSION_ID:-}" "Cannot parse Ubuntu VERSION_ID=${OS_VERSION_ID:-}")"
		((major >= 22)) || die "$(txt "仅支持 Ubuntu 22+，当前 VERSION_ID=${OS_VERSION_ID}" "Only Ubuntu 22+ is supported (current VERSION_ID=${OS_VERSION_ID})")"
		OS_FAMILY="debian"
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
	*)
		case " ${ID_LIKE:-} " in
		*" debian "* | *" ubuntu "*)
			OS_FAMILY="debian"
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
			die "$(txt "仅支持 Debian/Ubuntu、RHEL/Rocky/Alma/Oracle/Fedora、Arch/Manjaro 等 systemd 发行版，当前系统 ID=${OS_ID:-unknown}" "Only systemd-based Debian/Ubuntu, RHEL/Rocky/Alma/Oracle/Fedora, and Arch/Manjaro families are supported (current ID=${OS_ID:-unknown})")"
			;;
		esac
		;;
	esac

	need_cmd systemctl || die "$(txt "安装脚本依赖 systemd（未检测到 systemctl）" "This installer requires systemd (systemctl not found)")"
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
	if need_cmd psql; then
		return 0
	fi

	local dir
	for dir in /usr/pgsql-16/bin /usr/lib/postgresql/16/bin /usr/bin; do
		if [[ -x "${dir}/psql" ]]; then
			export PATH="${dir}:${PATH}"
			return 0
		fi
	done
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
	systemd_enable_now_first postgresql-16.service postgresql.service
}

restart_postgres_service() {
	systemd_restart_first postgresql-16.service postgresql.service
}

enable_restart_redis_service() {
	systemd_enable_now_first redis-server.service redis.service >/dev/null 2>&1 || true
	systemd_restart_first redis-server.service redis.service >/dev/null 2>&1 || true
}

write_redis_service() {
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

setup_postgresql_repo() {
	local arch
	arch="$(uname -m)"

	case "${OS_FAMILY}" in
	debian)
		if [[ -z "${OS_VERSION_CODENAME}" ]] && need_cmd lsb_release; then
			OS_VERSION_CODENAME="$(lsb_release -cs 2>/dev/null || true)"
		fi
		[[ -n "${OS_VERSION_CODENAME}" ]] || die "$(txt "无法确定 Debian/Ubuntu 代号（VERSION_CODENAME）" "Cannot determine Debian/Ubuntu codename (VERSION_CODENAME)")"
		local keyring="/usr/share/keyrings/postgresql-archive-keyring.gpg"
		if [[ ! -f "$keyring" ]]; then
			curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor | as_root tee "$keyring" >/dev/null
		fi
		as_root bash -c "cat > /etc/apt/sources.list.d/pgdg.list <<EOF
deb [signed-by=${keyring}] http://apt.postgresql.org/pub/repos/apt ${OS_VERSION_CODENAME}-pgdg main
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

	if [[ -x /usr/pgsql-16/bin/postgresql-16-setup ]]; then
		as_root /usr/pgsql-16/bin/postgresql-16-setup initdb >/dev/null 2>&1 || true
	fi

	if need_cmd postgresql-setup; then
		as_root postgresql-setup --initdb >/dev/null 2>&1 || \
			as_root postgresql-setup --initdb --unit postgresql >/dev/null 2>&1 || \
			as_root postgresql-setup --initdb --unit postgresql-16 >/dev/null 2>&1 || true
	fi

	if ! postgres_cluster_initialized && [[ "${OS_FAMILY}" == "arch" ]] && need_cmd initdb; then
		as_root install -d -m 0700 -o postgres -g postgres /var/lib/postgres/data
		as_postgres initdb -D /var/lib/postgres/data >/dev/null 2>&1 || true
	fi
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
	enable_postgres_service || true
}

postgres_major_version() {
	ensure_postgres_binaries_on_path
	if ! need_cmd psql; then
		echo ""
		return 0
	fi
	local v
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

	if need_cmd pg_config; then
		local sharedir
		sharedir="$(pg_config --sharedir 2>/dev/null || true)"
		if [[ -n "$sharedir" && -f "${sharedir}/extension/timescaledb.control" ]]; then
			return 0
		fi
	fi

	local control
	for control in \
		/usr/pgsql-16/share/extension/timescaledb.control \
		/usr/share/postgresql/16/extension/timescaledb.control \
		/usr/share/postgresql/extension/timescaledb.control \
		/usr/share/postgresql/17/extension/timescaledb.control \
		/usr/share/postgresql/18/extension/timescaledb.control; do
		if [[ -f "$control" ]]; then
			return 0
		fi
	done

	return 1
}

install_timescaledb_for_pg16() {
	ensure_pkg_prereqs

	case "${OS_FAMILY}" in
	debian)
		curl -fsSL https://packagecloud.io/install/repositories/timescale/timescaledb/script.deb.sh | as_root bash
		pkg_install timescaledb-2-postgresql-16
		;;
	rhel | fedora)
		curl -fsSL https://packagecloud.io/install/repositories/timescale/timescaledb/script.rpm.sh | as_root bash
		pkg_install timescaledb-2-postgresql-16
		;;
	arch)
		pkg_install timescaledb
		pkg_install timescaledb-tune >/dev/null 2>&1 || true
		;;
	*)
		die "$(txt "未知系统族：${OS_FAMILY:-unknown}" "Unknown OS family: ${OS_FAMILY:-unknown}")"
		;;
	esac
}

ensure_timescaledb_enabled() {
	if timescaledb_installed; then
		return 0
	fi

	if prompt_yes_no "$(txt "未检测到 TimescaleDB（PostgreSQL 16），是否安装并配置？" "TimescaleDB (for PostgreSQL 16) not detected. Install and configure it?")"; then
		install_timescaledb_for_pg16
	else
		die "$(txt "TimescaleDB 未安装，无法继续" "TimescaleDB is required")"
	fi

	if need_cmd timescaledb-tune; then
		as_root timescaledb-tune --quiet --yes >/dev/null 2>&1 || true
	fi
	restart_postgres_service || true
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

	write_redis_conf
	enable_restart_redis_service
}

install_redis_build_deps() {
	case "${OS_FAMILY}" in
	debian)
		pkg_install build-essential pkg-config tcl libsystemd-dev
		;;
	rhel | fedora)
		pkg_install gcc make pkgconf-pkg-config tcl systemd-devel
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
	if port_has_redis_listener 6379; then
		say "检测到端口 6379 已由 Redis 监听，跳过源码安装。" "Redis is already listening on port 6379; skipping source installation."
		return 10
	fi
	ensure_pkg_prereqs
	install_redis_build_deps

	local tmp
	tmp="$(mktemp -d)"
	trap "rm -rf \"${tmp}\"" EXIT

	local tgz="${tmp}/redis-${ver}.tar.gz"
	curl -fsSL -o "$tgz" "https://download.redis.io/releases/redis-${ver}.tar.gz"
	tar -C "$tmp" -xzf "$tgz"
	pushd "${tmp}/redis-${ver}" >/dev/null
	make USE_SYSTEMD=yes -j"$(nproc)"
	as_root make install
	hash -r || true
	popd >/dev/null

	if ! id -u redis >/dev/null 2>&1; then
		as_root useradd --system --no-create-home --shell /usr/sbin/nologin redis
	fi
	write_redis_conf

	write_redis_service

	if kill_listeners_on_port 6379; then
		:
	else
		local stop_status=$?
		if [[ "$stop_status" -eq 10 ]]; then
			say "检测到端口 6379 已由 Redis 监听，跳过源码安装。" "Redis is already listening on port 6379; skipping source installation."
			return 10
		fi
		return "$stop_status"
	fi

	as_root systemctl daemon-reload
	as_root systemctl enable --now redis-server.service
}

ensure_redis_82plus() {
	local want="8.2.3"
	local v

	case "${REDIS_INSTALL_METHOD}" in
	package | apt)
		v="$(redis_version)"
		if [[ -z "$v" ]]; then
			if ! prompt_yes_no "$(txt "未检测到 Redis，是否先尝试使用系统包管理器安装兼容的 Redis？" "Redis not detected. Try installing a compatible Redis via the system package manager?")"; then
				die "$(txt "Redis 未安装，无法继续" "Redis is required")"
			fi
			if ! install_redis_via_package_manager; then
				say "系统包管理器未提供可直接使用的 redis-server，改为源码安装。" "No usable redis-server package was found in the system repositories. Falling back to source install."
				local target_ver_missing
				target_ver_missing="$(prompt_string "$(txt "请输入要源码安装的 Redis 版本" "Redis version to install from source")" "8.2.5")"
				if install_redis_from_source "$target_ver_missing"; then
					:
				else
					local install_status=$?
					if [[ "$install_status" -eq 10 ]]; then
						return 0
					fi
					return "$install_status"
				fi
				v="$(redis_version)"
				[[ -n "$v" ]] || die "$(txt "Redis 安装失败：未检测到 redis-server" "Redis install failed: redis-server was not detected")"
				version_ge "$v" "$want" || die "$(txt "Redis 版本仍不足（当前 ${v}，需要 >=8.2）" "Redis version is still too old (current ${v}, need >=8.2)")"
				return 0
			fi
			v="$(redis_version)"
		fi

		if [[ -n "$v" ]] && version_ge "$v" "$want"; then
			write_redis_conf
			enable_restart_redis_service
			return 0
		fi

		if [[ -n "$v" ]]; then
			say "检测到 Redis ${v}，但需要 >=8.2。" "Detected Redis ${v}, but >=8.2 is required."
		else
			say "系统包管理器安装后仍未检测到可用的 redis-server。" "A usable redis-server binary is still not available after the package-manager attempt."
		fi
		if ! prompt_yes_no "$(txt "是否源码安装/升级 Redis（默认 8.2.5）？" "Install or upgrade Redis from source instead? (default 8.2.5)")"; then
			die "$(txt "Redis 版本不足，无法继续" "Redis version is insufficient")"
		fi
		local target_ver_pkg
		target_ver_pkg="$(prompt_string "$(txt "请输入要源码安装的 Redis 版本" "Redis version to install from source")" "8.2.5")"
		if install_redis_from_source "$target_ver_pkg"; then
			:
		else
			local install_status=$?
			if [[ "$install_status" -eq 10 ]]; then
				return 0
			fi
			return "$install_status"
		fi
		v="$(redis_version)"
		[[ -n "$v" ]] || die "$(txt "Redis 安装失败：未检测到 redis-server" "Redis install failed: redis-server not found")"
		version_ge "$v" "$want" || die "$(txt "Redis 版本仍不足（当前 ${v}，需要 >=8.2）" "Redis version still too old (current ${v}, need >=8.2)")"
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
			if ! prompt_yes_no "$(txt "检测到 Redis ${v}，需要 >=8.2。是否源码安装/升级？" "Detected Redis ${v}. Need >=8.2. Install/upgrade from source?")"; then
				die "$(txt "Redis 版本不足，无法继续" "Redis version too old")"
			fi
		fi

		local target_ver
		target_ver="$(prompt_string "$(txt "请输入要源码安装的 Redis 版本" "Redis version to install (source build)")" "8.2.5")"
		if install_redis_from_source "$target_ver"; then
			:
		else
			local install_status=$?
			if [[ "$install_status" -eq 10 ]]; then
				return 0
			fi
			return "$install_status"
		fi

		v="$(redis_version)"
		[[ -n "$v" ]] || die "$(txt "Redis 安装失败：未检测到 redis-server" "Redis install failed: redis-server not found")"
		version_ge "$v" "$want" || die "$(txt "Redis 版本仍不足（当前 ${v}，需要 >=8.2）" "Redis version still too old (current ${v}, need >=8.2)")"
		;;
	*)
		die "$(txt "未知 REDIS_INSTALL_METHOD=${REDIS_INSTALL_METHOD}（支持：package/source；apt 仍可作为兼容别名）" "Unknown REDIS_INSTALL_METHOD=${REDIS_INSTALL_METHOD} (supported: package/source; apt remains a compatibility alias)")"
		;;
	esac
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
	role_exists="$(as_postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${db_user}'" | tr -d '[:space:]' || true)"
	if [[ "$role_exists" != "1" ]]; then
		as_postgres psql -v ON_ERROR_STOP=1 -c "CREATE USER ${db_user} WITH PASSWORD '${pass_sql}';" >/dev/null
	else
		as_postgres psql -v ON_ERROR_STOP=1 -c "ALTER USER ${db_user} WITH PASSWORD '${pass_sql}';" >/dev/null
	fi

	db_exists="$(as_postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${db_name}'" | tr -d '[:space:]' || true)"
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
	local offline_threshold="${9:-14s}"
	local language="${10:-${INSTALL_LANG:-zh}}"
	local trusted_proxies_yaml="${11:-[]}"

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

	local dash_ip_esc listen_esc public_url_esc db_user_esc db_pass_esc db_name_esc retention_line_esc redis_addr_esc offline_th_esc language_esc
	dash_ip_esc="$(yaml_dq_escape "$dash_ip")"
	listen_esc="$(yaml_dq_escape "$listen")"
	public_url_esc="$(yaml_dq_escape "$public_url")"
	db_user_esc="$(yaml_dq_escape "$db_user")"
	db_pass_esc="$(yaml_dq_escape "$db_pass")"
	db_name_esc="$(yaml_dq_escape "$db_name")"
	retention_line_esc="$(yaml_dq_escape "$retention_line")"
	redis_addr_esc="$(yaml_dq_escape "$redis_addr")"
	offline_th_esc="$(yaml_dq_escape "$offline_threshold")"
	language_esc="$(yaml_dq_escape "$language")"

	local jwt_signing_key jwt_signing_key_esc
	jwt_signing_key="$(set +o pipefail; tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
	[[ "${#jwt_signing_key}" -ge 32 ]] || die "JWT signing key generation failed"
	jwt_signing_key_esc="$(yaml_dq_escape "$jwt_signing_key")"

	local tmp
	tmp="$(mktemp -t dash-config-local-XXXXXX)"
	trap "rm -f \"${tmp}\" >/dev/null 2>&1 || true" RETURN

	if ! grep -q '__APP_DASH_IP__\|__APP_LISTEN__\|__APP_PUBLIC_URL__\|__APP_LANGUAGE__\|__APP_NODE_OFFLINE_THRESHOLD__\|__HTTP_TRUSTED_PROXIES__\|__DB_USER__\|__DB_PASS__\|__DB_NAME__\|__DB_RETENTION_DAYS_LINE__\|__REDIS_ADDR__\|__JWT_SIGNING_KEY__' "$CONFIG_EXAMPLE"; then
		die "$(txt "写入配置失败：模板缺少占位符（请更新 ${CONFIG_EXAMPLE}）" "Failed to write config: template missing placeholders (please update ${CONFIG_EXAMPLE})")"
	fi
	if ! grep -q '__APP_LANGUAGE__' "$CONFIG_EXAMPLE"; then
		die "$(txt "写入配置失败：模板缺少 __APP_LANGUAGE__（请更新 ${CONFIG_EXAMPLE}）" "Failed to write config: template missing __APP_LANGUAGE__ (please update ${CONFIG_EXAMPLE})")"
	fi
	if ! grep -q '__APP_NODE_OFFLINE_THRESHOLD__' "$CONFIG_EXAMPLE"; then
		die "$(txt "写入配置失败：模板缺少 __APP_NODE_OFFLINE_THRESHOLD__（请更新 ${CONFIG_EXAMPLE}）" "Failed to write config: template missing __APP_NODE_OFFLINE_THRESHOLD__ (please update ${CONFIG_EXAMPLE})")"
	fi

	local r_dash_ip r_listen r_public_url r_language r_offline_th r_http_trusted_proxies r_db_user r_db_pass r_db_name r_db_retention_line r_redis_addr r_jwt_signing_key
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
		-e "s|__JWT_SIGNING_KEY__|${r_jwt_signing_key}|g" \
		"$CONFIG_EXAMPLE" >"$tmp"

	if grep -q '__APP_DASH_IP__\|__APP_LISTEN__\|__APP_PUBLIC_URL__\|__APP_LANGUAGE__\|__APP_NODE_OFFLINE_THRESHOLD__\|__HTTP_TRUSTED_PROXIES__\|__DB_USER__\|__DB_PASS__\|__DB_NAME__\|__DB_RETENTION_DAYS_LINE__\|__REDIS_ADDR__\|__JWT_SIGNING_KEY__' "$tmp"; then
		die "$(txt "写入配置失败：模板占位符未被替换（请确认 config.example.yaml 版本与安装脚本一致）" "Failed to write config: placeholders were not replaced (template/script mismatch)")"
	fi

	as_root install -o root -g root -m 0600 "$tmp" "$CONFIG_LOCAL"
	say "已写入配置：${CONFIG_LOCAL}" "Wrote config: ${CONFIG_LOCAL}"
}

is_ip_literal() {
	local host="$1"
	if [[ "$host" =~ ^\\[[0-9a-fA-F:]+\\]$ ]]; then
		return 0
	fi
	[[ "$host" =~ ^([0-9]{1,3}\\.){3}[0-9]{1,3}$ ]]
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
	[[ "$s" =~ ^[!-~]+$ ]]
}

yaml_sq_escape() {
	local s="$1"
	s="${s//\'/\'\'}"
	printf "%s" "$s"
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
	else
		ip_probe="${hostport%%:*}"
	fi

	local scheme="https"
	if is_ip_literal "$ip_probe"; then
		scheme="http"
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
	local rest="${public_url#*://}"
	[[ "$rest" == */* ]] || return 0

	local path_part="/${rest#*/}"
	path_part="${path_part%%\?*}"
	path_part="${path_part%%#*}"
	[[ -n "$path_part" && "$path_part" != "/" ]] || return 0

	die "$(txt "public_url 不支持路径前缀（${path_part}）。请改为根路径 URL，例如 http://127.0.0.1:8080/ 或 https://dash.example.com/" "public_url does not support path prefixes (${path_part}). Use a root URL such as http://127.0.0.1:8080/ or https://dash.example.com/")"
}

install_app_files_from_cwd() {
	[[ -f "${SCRIPT_DIR}/bin/dash" ]] || die "$(txt "未找到可执行文件 ${SCRIPT_DIR}/bin/dash" "Missing executable: ${SCRIPT_DIR}/bin/dash")"

	as_root install -d -m 0755 "$INSTALL_DIR"
	as_root bash -c "cp -a \"${SCRIPT_DIR}/.\" \"${INSTALL_DIR}/\""

	[[ -f "$BIN_PATH" ]] || die "$(txt "安装后未找到可执行文件 ${BIN_PATH}" "Missing executable after install: ${BIN_PATH}")"
	as_root chmod 0755 "$BIN_PATH" || true
}

tighten_sensitive_file_permissions() {
	if [[ -f "$CONFIG_LOCAL" ]]; then
		as_root chown root:root "$CONFIG_LOCAL"
		as_root chmod 0600 "$CONFIG_LOCAL"
	fi
	if [[ -f "$SERVICE_FILE" ]]; then
		as_root chown root:root "$SERVICE_FILE"
		as_root chmod 0600 "$SERVICE_FILE"
	fi
}

write_systemd_service() {
	local admin_password="$1"
	admin_password="$(one_line "$admin_password")"
	local pwd_escaped
	pwd_escaped="$(systemd_escape_env_value "$admin_password")"

	as_root bash -c "cat > '${SERVICE_FILE}' <<EOF
[Unit]
Description=Dash Server Monitor
After=network-online.target postgresql.service postgresql-16.service redis-server.service redis.service
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=${INSTALL_DIR}

Environment=\"DASH_HOME=${INSTALL_DIR}\"
Environment=\"monitor_dash_pwd=${pwd_escaped}\"

ExecStart=${BIN_PATH}

Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF"

	tighten_sensitive_file_permissions
	as_root systemctl daemon-reload
	as_root systemctl enable --now "${APP}.service"
}

main() {
	if [[ -f "${CONFIG_LOCAL}" ]] && systemd_unit_exists; then
		say "检测到已有安装：" "Existing installation detected:"
		say "  - 配置文件已存在：${CONFIG_LOCAL}" "  - Config file exists: ${CONFIG_LOCAL}"
		say "  - systemd 服务已存在：${APP}.service" "  - systemd unit exists: ${APP}.service"
		echo ""

		if prompt_yes_no "$(txt "是否覆盖配置文件？（将重新生成 config.local.yaml，并更新 systemd 环境密码等）" "Overwrite config file? (Will regenerate config.local.yaml and update systemd env password, etc.)")" "N"; then
			say "选择：覆盖配置（继续完整安装流程）" "Choice: overwrite config (continue full install flow)"
		else
			say "选择：仅更新文件（复制当前目录到 ${INSTALL_DIR}）并重启 ${APP}.service" "Choice: update files only (copy current dir to ${INSTALL_DIR}) and restart ${APP}.service"
			as_root systemctl stop "${APP}.service"
			install_app_files_from_cwd
			tighten_sensitive_file_permissions
			run_db_migrations "$CONFIG_LOCAL"
			as_root systemctl start "${APP}.service"
			say "完成：已更新并重启 ${APP}.service" "Done: updated and restarted ${APP}.service"
			return 0
		fi
	fi

	detect_os

	if [[ -z "${PKG_MANAGER}" ]] || ! need_cmd "${PKG_MANAGER}"; then
		die "$(txt "未检测到系统包管理器（当前需要 ${PKG_MANAGER_LABEL:-unknown}）" "System package manager not found (expected ${PKG_MANAGER_LABEL:-unknown})")"
	fi

	enable_time_sync

	say "1) 检测并准备依赖（PostgreSQL 16+ / TimescaleDB / Redis；默认使用系统包管理器安装：${PKG_MANAGER_LABEL}）" "1) Checking dependencies (PostgreSQL 16+ / TimescaleDB / Redis; default system package manager: ${PKG_MANAGER_LABEL})"
	ensure_postgresql16_and_password
	ensure_timescaledb_enabled
	ensure_redis_82plus

	say "2) 安装 Dash 文件到 ${INSTALL_DIR}" "2) Installing Dash files into ${INSTALL_DIR}"
	install_app_files_from_cwd

	say "3) 交互式生成配置 ${CONFIG_LOCAL}" "3) Interactive configuration: ${CONFIG_LOCAL}"
	local dash_ip listen_port public_url db_user db_pass db_name retention_days redis_addr offline_threshold language admin_pwd trusted_proxies_yaml

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
	public_url="$(prompt_string "$(txt "请输入 public_url（用于生成安装脚本/外网访问，必须是根路径 URL）" "public_url (external access URL, root URL only)")" "http://127.0.0.1:${listen_port}/")"
	public_url="$(normalize_public_url "$public_url")"
	[[ -n "$public_url" ]] || die "$(txt "public_url 不能为空" "public_url is required")"
	require_root_public_url "$public_url"
	trusted_proxies_yaml="[]"
	if prompt_yes_no "$(txt "是否通过本机反向代理（如同机 Nginx/Caddy/Traefik）对外暴露 Dash？启用后仅信任来自本机代理的转发头。" "Is Dash exposed through a local reverse proxy on the same host (for example Nginx/Caddy/Traefik)? This will trust forwarded headers only from that local proxy.")" "N"; then
		trusted_proxies_yaml='["127.0.0.1/32", "::1/128"]'
	fi
	db_user="$(prompt_string "$(txt "请输入数据库账号（database.user）" "database.user")" "monitor")"
	db_pass="$(prompt_secret_confirm "$(txt "请输入数据库密码（database.password）" "database.password")")"
	db_name="$(prompt_string "$(txt "请输入数据库名（database.name）" "database.name")" "monitor")"
	retention_days="$(prompt_retention_days 1)"
	redis_addr="$(prompt_string "$(txt "请输入 Redis 地址（redis.addr）" "redis.addr")" "127.0.0.1:6379")"
	offline_threshold="$(prompt_string "$(txt "请输入离线判定阈值（app.node_offline_threshold，例如：14s/30s/1m）" "app.node_offline_threshold (e.g. 14s/30s/1m)")" "14s")"
	language="$(prompt_language)"

	echo ""
	say "3.1) 配置摘要（写入前确认）" "3.1) Configuration summary (confirm before writing)"

	print_config_summary "$dash_ip" "$listen_port" "$public_url" "$db_user" "$db_pass" "$db_name" "$retention_days" "$redis_addr" "$offline_threshold" "$language" "$trusted_proxies_yaml"
	echo ""

	render_config_local "$dash_ip" "$listen_port" "$public_url" "$db_user" "$db_pass" "$db_name" "$retention_days" "$redis_addr" "$offline_threshold" "$language" "$trusted_proxies_yaml"

	say "4) 初始化数据库（创建用户/库 + 启用 timescaledb 扩展）" "4) Initializing database (user/db + timescaledb extension)"
	create_db_and_user "$db_user" "$db_pass" "$db_name"
	run_db_migrations "$CONFIG_LOCAL"
	grant_db_privileges "$db_user" "$db_name"

	say "5) 写入 systemd 并开机自启" "5) Installing systemd unit and enabling autostart"
	while true; do
		admin_pwd="$(prompt_secret_confirm "$(txt "请设置 Dash 管理员登录密码（环境变量 monitor_dash_pwd）" "Dash admin password (env monitor_dash_pwd)")")"
		admin_pwd="$(trim_spaces "$admin_pwd")"
		if ! admin_password_valid "$admin_pwd"; then
			say_err "Dash 管理员密码仅允许大小写英文、数字和常见符号；会自动忽略输入前后的空格；中间不能包含空格或其他空白字符。" "Dash admin password must use only ASCII letters, digits, and common symbols; leading and trailing spaces are ignored; inner whitespace is not allowed."
			continue
		fi
		break
	done
	write_systemd_service "$admin_pwd"

	warn_if_domain_public_url "$public_url"
	say "完成：systemd 服务 ${APP}.service 已启动" "Done. systemd service ${APP}.service is running."
}

parse_args "$@"
choose_install_lang
main
