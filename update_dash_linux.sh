#!/usr/bin/env bash
set -euo pipefail

if [[ "${DASH_UPDATE_REEXEC:-}" != "1" ]]; then
  self_path="${BASH_SOURCE[0]}"
  self_dir="$(cd "$(dirname "$self_path")" && pwd)"
  self_tmp_root="$(mktemp -d)"
  self_copy="${self_tmp_root}/update_dash_linux.sh"
  cp "$self_path" "$self_copy"
  chmod 0755 "$self_copy"
  export DASH_UPDATE_REEXEC="1"
  export DASH_UPDATE_ORIGINAL_DIR="$self_dir"
  export DASH_UPDATE_SELF_TMP_ROOT="$self_tmp_root"
  exec bash "$self_copy" "$@"
fi

APP="dash"
SERVICE="${APP}.service"
SERVICE_FILE="/etc/systemd/system/${SERVICE}"
INSTALL_DIR="/opt/Ithiltir-dash"
BIN_PATH="${INSTALL_DIR}/bin/dash"
SCRIPT_DIR="${DASH_UPDATE_ORIGINAL_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
SCRIPT_BIN_PATH="${SCRIPT_DIR}/bin/dash"
CONFIG_LOCAL="${INSTALL_DIR}/configs/config.local.yaml"
REMOTE_URL="${REMOTE_URL:-https://github.com/Ithildur/Ithiltir.git}"
REPO_SLUG="${REPO_SLUG:-Ithildur/Ithiltir}"

ASSUME_YES="false"
CHECK_ONLY="false"
TEST_CHANNEL="false"
SCRIPT_LANG="${SCRIPT_LANG:-}"
SHOW_HELP="false"
SELF_TMP_ROOT="${DASH_UPDATE_SELF_TMP_ROOT:-}"
INSTALL_TMP_ROOT=""
KEEP_INSTALL_TMP="false"

need_cmd() { command -v "$1" >/dev/null 2>&1; }

cleanup_tmp() {
  if [[ "$KEEP_INSTALL_TMP" != "true" && -n "${INSTALL_TMP_ROOT:-}" ]]; then
    rm -rf "$INSTALL_TMP_ROOT"
  fi
  if [[ -n "${SELF_TMP_ROOT:-}" ]]; then
    rm -rf "$SELF_TMP_ROOT"
  fi
}

trap cleanup_tmp EXIT

die() {
  echo "error: $*" >&2
  exit 1
}

default_script_lang() {
  case "${LANG:-}" in
    zh* | zh_*) echo "zh" ;;
    *) echo "en" ;;
  esac
}

choose_script_lang() {
  case "$SCRIPT_LANG" in
    zh | en) return 0 ;;
    "") ;;
    *) die "SCRIPT_LANG must be zh or en: $SCRIPT_LANG" ;;
  esac

  local default ans
  default="$(default_script_lang)"
  if [[ ! -t 0 ]]; then
    SCRIPT_LANG="$default"
    return 0
  fi

  while true; do
    echo "Select updater language / 选择更新脚本语言:" >&2
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
      1) SCRIPT_LANG="en"; return 0 ;;
      2) SCRIPT_LANG="zh"; return 0 ;;
      *) echo "Please enter 1 or 2 / 请输入 1 或 2" >&2 ;;
    esac
  done
}

txt() {
  if [[ "${SCRIPT_LANG:-en}" == "zh" ]]; then
    printf '%s' "$1"
  else
    printf '%s' "$2"
  fi
}

say() { echo "$(txt "$1" "$2")"; }
say_err() { echo "$(txt "$1" "$2")" >&2; }

as_root() {
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    "$@"
    return
  fi
  if need_cmd sudo; then
    sudo "$@"
    return
  fi
  die "$(txt "需要 root 权限且未安装 sudo" "root privileges are required and sudo is not installed")"
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

backup_install_dir() {
  local backup_dir="$1"
  [[ -d "$INSTALL_DIR" ]] || return 0
  mkdir -p "$backup_dir"
  as_root cp -a "${INSTALL_DIR}/." "$backup_dir/"
}

restore_install_dir() {
  local backup_dir="$1"
  [[ -d "$backup_dir" ]] || return 1
  as_root rm -rf "$INSTALL_DIR"
  as_root install -d -m 0755 "$INSTALL_DIR"
  as_root cp -a "${backup_dir}/." "$INSTALL_DIR/"
  tighten_sensitive_file_permissions
}

start_service_if_needed() {
  local was_active="$1"
  if [[ "$was_active" == "true" ]]; then
    as_root systemctl start "$SERVICE"
  fi
}

detect_package_manager() {
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    case "${ID:-}" in
      ubuntu|debian)
        echo "apt-get"
        return 0
        ;;
      fedora)
        echo "dnf"
        return 0
        ;;
      rhel|centos|rocky|almalinux|ol)
        if need_cmd dnf; then echo "dnf"; else echo "yum"; fi
        return 0
        ;;
      arch|manjaro)
        echo "pacman"
        return 0
        ;;
      alpine)
        echo "apk"
        return 0
        ;;
      opensuse*|sles)
        echo "zypper"
        return 0
        ;;
    esac

    for like in ${ID_LIKE:-}; do
      case "$like" in
        debian)
          echo "apt-get"
          return 0
          ;;
        fedora|rhel)
          if need_cmd dnf; then echo "dnf"; else echo "yum"; fi
          return 0
          ;;
        arch)
          echo "pacman"
          return 0
          ;;
        alpine)
          echo "apk"
          return 0
          ;;
        suse)
          echo "zypper"
          return 0
          ;;
      esac
    done
  fi

  for pm in apt-get dnf yum pacman apk zypper; do
    if need_cmd "$pm"; then
      echo "$pm"
      return 0
    fi
  done
  return 1
}

install_packages() {
  local pm="$1"
  shift
  case "$pm" in
    apt-get)
      as_root apt-get update -y
      as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
      ;;
    dnf)
      as_root dnf install -y "$@"
      ;;
    yum)
      as_root yum install -y "$@"
      ;;
    pacman)
      as_root pacman -Sy --noconfirm "$@"
      ;;
    apk)
      as_root apk add --no-cache "$@"
      ;;
    zypper)
      as_root zypper --non-interactive install "$@"
      ;;
    *)
      return 1
      ;;
  esac
}

ensure_cmd() {
  local cmd="$1" pkg="${2:-$1}" pm
  if need_cmd "$cmd"; then
    return 0
  fi
  pm="$(detect_package_manager)" || die "$(txt "$cmd 是必需命令，但未找到支持的包管理器" "$cmd is required and no supported package manager was found")"
  say_err "未找到 $cmd，正在安装包：$pkg" "$cmd not found. installing package: $pkg"
  install_packages "$pm" "$pkg"
  need_cmd "$cmd" || die "$(txt "$cmd 安装完成但命令仍不可用" "$cmd installation finished but command is still unavailable")"
}

ensure_download_tool() {
  if need_cmd curl || need_cmd wget; then
    return 0
  fi
  ensure_cmd curl curl
}

ensure_dependencies() {
  ensure_cmd git git
  ensure_cmd tar tar
  ensure_download_tool
  need_cmd systemctl || die "$(txt "更新 ${SERVICE} 需要 systemctl" "systemctl is required to update ${SERVICE}")"
}

usage() {
  if [[ "${SCRIPT_LANG:-$(default_script_lang)}" == "zh" ]]; then
    cat <<EOF
用法：
  update_dash_linux.sh [--check] [--test] [-y|--yes] [--lang zh|en]

选项：
  --check      只检查目标通道是否存在更新的 Git tag
  --test       更新到最新 prerelease；不带该参数时只更新到最新 release
  -y|--yes     不交互确认，直接更新
  --lang       设置脚本语言：zh 或 en
  -h|--help    显示帮助

环境变量：
  REMOTE_URL   用于发现 tag 的 Git remote
  REPO_SLUG    用于下载 release asset 的 GitHub owner/repo
EOF
    return
  fi

  cat <<EOF
Usage:
  update_dash_linux.sh [--check] [--test] [-y|--yes] [--lang zh|en]

Options:
  --check      Only check whether a newer Git tag exists in the target channel
  --test       Update to the latest prerelease; without it, only update to the latest release
  -y|--yes     Update without an interactive confirmation
  --lang       Set script language: zh or en
  -h|--help    Show this help

Environment:
  REMOTE_URL   Git remote used for tag discovery
  REPO_SLUG    GitHub owner/repo used for release asset downloads
EOF
}

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

is_numeric_identifier() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

valid_number() {
  [[ "$1" =~ ^(0|[1-9][0-9]*)$ ]]
}

valid_identifiers() {
  local value="$1"
  local check_number="$2"
  local ident
  local -a identifiers

  [[ -n "$value" ]] || return 1
  [[ "$value" != .* && "$value" != *. && "$value" != *..* ]] || return 1

  IFS=. read -ra identifiers <<<"$value"
  for ident in "${identifiers[@]}"; do
    [[ "$ident" =~ ^[0-9A-Za-z-]+$ ]] || return 1
    if [[ "$check_number" == "true" ]] && is_numeric_identifier "$ident"; then
      valid_number "$ident" || return 1
    fi
  done
}

valid_semver() {
  local version main build core pre major minor patch extra
  version="$(trim "$1")"
  [[ -n "$version" ]] || return 1
  [[ "$version" != v* ]] || return 1
  [[ ! "$version" =~ [[:space:]] ]] || return 1

  main="$version"
  if [[ "$version" == *+* ]]; then
    main="${version%%+*}"
    build="${version#*+}"
    [[ "$build" != *+* ]] || return 1
    valid_identifiers "$build" false || return 1
  fi

  core="$main"
  if [[ "$main" == *-* ]]; then
    core="${main%%-*}"
    pre="${main#*-}"
    valid_identifiers "$pre" true || return 1
  fi

  IFS=. read -r major minor patch extra <<<"$core"
  [[ -z "${extra:-}" ]] || return 1
  valid_number "${major:-}" || return 1
  valid_number "${minor:-}" || return 1
  valid_number "${patch:-}" || return 1
}

release_channel_for_version() {
  local version
  version="$(trim "$1")"
  valid_semver "$version" || return 1
  release_channel_from_semver "$version"
}

release_channel_from_semver() {
  local main
  main="${1%%+*}"
  if [[ "$main" == *-* ]]; then
    echo "prerelease"
    return 0
  fi

  echo "release"
}

semver_core() {
  local version main core
  version="$(trim "$1")"
  main="${version%%+*}"
  core="${main%%-*}"
  printf '%s\n' "$core"
}

semver_prerelease() {
  local version main
  version="$(trim "$1")"
  main="${version%%+*}"
  if [[ "$main" == *-* ]]; then
    printf '%s\n' "${main#*-}"
  fi
}

compare_number() {
  local a="$1" b="$2"
  if ((${#a} > ${#b})); then echo 1; return; fi
  if ((${#a} < ${#b})); then echo -1; return; fi
  if [[ "$a" > "$b" ]]; then echo 1; return; fi
  if [[ "$a" < "$b" ]]; then echo -1; return; fi
  echo 0
}

compare_prerelease_identifier() {
  local a="$1" b="$2" a_numeric="false" b_numeric="false"
  if is_numeric_identifier "$a"; then a_numeric="true"; fi
  if is_numeric_identifier "$b"; then b_numeric="true"; fi

  if [[ "$a_numeric" == "true" && "$b_numeric" == "true" ]]; then
    compare_number "$a" "$b"
    return
  fi
  if [[ "$a_numeric" == "true" ]]; then echo -1; return; fi
  if [[ "$b_numeric" == "true" ]]; then echo 1; return; fi
  if [[ "$a" > "$b" ]]; then echo 1; return; fi
  if [[ "$a" < "$b" ]]; then echo -1; return; fi
  echo 0
}

compare_prerelease() {
  local a="$1" b="$2" i limit cmp
  local -a a_parts b_parts
  IFS=. read -ra a_parts <<<"$a"
  IFS=. read -ra b_parts <<<"$b"
  limit="${#a_parts[@]}"
  if ((${#b_parts[@]} < limit)); then
    limit="${#b_parts[@]}"
  fi
  for ((i = 0; i < limit; i++)); do
    cmp="$(compare_prerelease_identifier "${a_parts[$i]}" "${b_parts[$i]}")"
    [[ "$cmp" == "0" ]] || { echo "$cmp"; return; }
  done
  if ((${#a_parts[@]} > ${#b_parts[@]})); then echo 1; return; fi
  if ((${#a_parts[@]} < ${#b_parts[@]})); then echo -1; return; fi
  echo 0
}

compare_versions() {
  local a="$1" b="$2" a_core b_core a_pre b_pre a1 a2 a3 b1 b2 b3 cmp
  valid_semver "$a" || return 2
  valid_semver "$b" || return 2

  a_core="$(semver_core "$a")"
  b_core="$(semver_core "$b")"
  IFS=. read -r a1 a2 a3 <<<"$a_core"
  IFS=. read -r b1 b2 b3 <<<"$b_core"
  for pair in "$a1:$b1" "$a2:$b2" "$a3:$b3"; do
    cmp="$(compare_number "${pair%%:*}" "${pair#*:}")"
    [[ "$cmp" == "0" ]] || { echo "$cmp"; return; }
  done

  a_pre="$(semver_prerelease "$a")"
  b_pre="$(semver_prerelease "$b")"
  if [[ -z "$a_pre" && -z "$b_pre" ]]; then echo 0; return; fi
  if [[ -z "$a_pre" ]]; then echo 1; return; fi
  if [[ -z "$b_pre" ]]; then echo -1; return; fi
  compare_prerelease "$a_pre" "$b_pre"
}

version_gt() {
  local cmp
  if ! valid_semver "$1"; then
    return 1
  fi
  if ! valid_semver "$2"; then
    return 0
  fi
  cmp="$(compare_versions "$1" "$2")"
  [[ "$cmp" == "1" ]]
}

current_version() {
  if [[ -x "$BIN_PATH" ]]; then
    "$BIN_PATH" --version 2>/dev/null || true
    return
  fi
  if [[ -x "$SCRIPT_BIN_PATH" ]]; then
    "$SCRIPT_BIN_PATH" --version 2>/dev/null || true
    return
  fi
  printf '0.0.0-dev\n'
}

latest_remote_version() {
  local channel="$1" refs latest tag tag_channel
  refs="$(git ls-remote --tags --refs "$REMOTE_URL")" || die "$(txt "无法从 $REMOTE_URL 获取 tags" "failed to fetch tags from $REMOTE_URL")"
  latest=""

  while read -r _ ref; do
    [[ -n "${ref:-}" ]] || continue
    tag="${ref#refs/tags/}"
    tag_channel="$(release_channel_for_version "$tag" || true)"
    [[ "$tag_channel" == "$channel" ]] || continue
    if [[ -z "$latest" ]] || version_gt "$tag" "$latest"; then
      latest="$tag"
    fi
  done <<<"$refs"

  [[ -n "$latest" ]] || die "$(txt "未在 $REMOTE_URL 找到有效的 $channel 版本 tag" "no valid $channel version tags found on $REMOTE_URL")"
  printf '%s\n' "$latest"
}

detect_arch() {
  local m
  m="$(uname -m)"
  case "$m" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) die "$(txt "不支持的架构：$m" "unsupported architecture: $m")" ;;
  esac
}

download_file() {
  local url="$1" out="$2"
  if need_cmd curl; then
    curl -fL --retry 3 --connect-timeout 10 --max-time 600 -o "$out" "$url"
    return
  fi
  if need_cmd wget; then
    wget -O "$out" "$url"
    return
  fi
  die "$(txt "下载 release 资源需要 curl 或 wget" "curl or wget is required to download release assets")"
}

confirm_update() {
  local current="$1" latest="$2" answer
  if [[ "$ASSUME_YES" == "true" ]]; then
    return 0
  fi
  printf '%s' "$(txt "是否将 Dash 从 ${current} 更新到 ${latest}？[y/N] " "Update Dash from ${current} to ${latest}? [y/N] ")"
  read -r answer
  [[ "$answer" =~ ^[Yy]([Ee][Ss])?$ ]]
}

download_release() {
  local version="$1" out="$2" arch asset url
  arch="$(detect_arch)"
  asset="Ithiltir_dash_linux_${arch}.tar.gz"
  url="https://github.com/${REPO_SLUG}/releases/download/${version}/${asset}"
  say "下载：$url" "download: $url"
  download_file "$url" "$out"
}

install_release() {
  local version="$1" archive extract_dir backup_dir pkg_root new_version was_active
  INSTALL_TMP_ROOT="$(mktemp -d)"
  KEEP_INSTALL_TMP="false"
  archive="${INSTALL_TMP_ROOT}/dash.tar.gz"
  extract_dir="${INSTALL_TMP_ROOT}/extract"
  backup_dir="${INSTALL_TMP_ROOT}/backup"
  mkdir -p "$extract_dir"

  download_release "$version" "$archive"
  tar -xzf "$archive" -C "$extract_dir"

  pkg_root="${extract_dir}/Ithiltir-dash"
  [[ -x "${pkg_root}/bin/dash" ]] || die "$(txt "release 包缺少 bin/dash" "release package is missing bin/dash")"
  new_version="$("${pkg_root}/bin/dash" --version)"
  [[ "$new_version" == "$version" ]] || die "$(txt "release 资源版本不匹配：实际 $new_version，期望 $version" "release asset version mismatch: got $new_version, want $version")"

  [[ -f "$CONFIG_LOCAL" ]] || die "$(txt "缺少配置文件：$CONFIG_LOCAL" "missing config: $CONFIG_LOCAL")"

  was_active="false"
  if systemctl is-active --quiet "$SERVICE"; then
    was_active="true"
  fi

  as_root systemctl stop "$SERVICE"
  backup_install_dir "$backup_dir"
  as_root install -d -m 0755 "$INSTALL_DIR"
  as_root cp -a "${pkg_root}/." "$INSTALL_DIR/"
  as_root chmod 0755 "$BIN_PATH"
  tighten_sensitive_file_permissions

  if ! as_root env DASH_HOME="$INSTALL_DIR" "$BIN_PATH" migrate -config "$CONFIG_LOCAL"; then
    say_err "迁移阶段升级失败，正在回滚文件" "upgrade failed during migration; rolling back files"
    if ! restore_install_dir "$backup_dir"; then
      KEEP_INSTALL_TMP="true"
      die "$(txt "迁移失败且文件回滚失败；备份保留在 ${backup_dir}" "migration failed and rollback failed; backup preserved at ${backup_dir}")"
    fi
    if ! start_service_if_needed "$was_active"; then
      KEEP_INSTALL_TMP="true"
      die "$(txt "迁移失败；文件已恢复但重启 ${SERVICE} 失败；备份保留在 ${backup_dir}" "migration failed; files restored but failed to restart ${SERVICE}; backup preserved at ${backup_dir}")"
    fi
    die "$(txt "迁移失败；文件已恢复" "migration failed; files restored")"
  fi

  if ! as_root systemctl start "$SERVICE"; then
    say_err "服务启动失败：${SERVICE}" "service start failed: ${SERVICE}"
    die "$(txt "升级已提交，文件和数据库已保留。请检查：systemctl status ${SERVICE} && journalctl -u ${SERVICE} -n 100 --no-pager" "upgrade committed; files and database were kept. Check: systemctl status ${SERVICE} && journalctl -u ${SERVICE} -n 100 --no-pager")"
  fi

  say "已更新：$("$BIN_PATH" --version)" "updated: $("$BIN_PATH" --version)"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --check)
        CHECK_ONLY="true"
        shift
        ;;
      --test)
        TEST_CHANNEL="true"
        shift
        ;;
      -y|--yes)
        ASSUME_YES="true"
        shift
        ;;
      --lang)
        [[ $# -ge 2 ]] || die "missing value for --lang"
        SCRIPT_LANG="$2"
        shift 2
        ;;
      --lang=*)
        SCRIPT_LANG="${1#--lang=}"
        shift
        ;;
      -h|--help)
        SHOW_HELP="true"
        shift
        ;;
      *)
        say_err "未知选项：$1" "unknown option: $1"
        SHOW_HELP="true"
        return 2
        ;;
    esac
  done
}

parse_args "$@" || {
  usage >&2
  exit 2
}
if [[ "$SHOW_HELP" == "true" ]]; then
  usage
  exit 0
fi
choose_script_lang

ensure_dependencies

current="$(trim "$(current_version)")"
current_channel="$(release_channel_for_version "$current" || echo release)"
target_channel="release"
if [[ "$TEST_CHANNEL" == "true" ]]; then
  target_channel="prerelease"
fi
latest="$(latest_remote_version "$target_channel")"

say "当前版本：${current:-unknown}" "current: ${current:-unknown}"
say "当前通道：$current_channel" "current channel: $current_channel"
say "目标通道：$target_channel" "target channel:  $target_channel"
say "最新版本：$latest" "latest:  $latest"

if [[ "$TEST_CHANNEL" != "true" && "$current_channel" == "prerelease" ]] && version_gt "$current" "$latest"; then
  say_err \
    "当前部署版本 ${current} 是 prerelease，且高于最新 release ${latest}。如需继续测试通道，请加 --test；默认 release 更新已停止。" \
    "current deployed version ${current} is a prerelease and is newer than the latest release ${latest}. Pass --test to stay on the test channel; default release update stopped."
  exit 1
fi

if ! version_gt "$latest" "$current"; then
  say "Dash 已是最新版本。" "Dash is up to date."
  exit 0
fi

say "发现新的 Dash release。" "A newer Dash release is available."
if [[ "$CHECK_ONLY" == "true" ]]; then
  exit 0
fi

if ! confirm_update "${current:-unknown}" "$latest"; then
  say "已取消更新" "update canceled"
  exit 0
fi

install_release "$latest"
