#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="release"
TARGETS=()
ARCHIVE_FORMAT=""
FRONTEND_DIST_DIR="build/frontend/dist"
VERSION=""
USE_GIT_TAG="false"
RELEASE_MODE="false"
NODE_RELEASE_VERSION=""
NODE_LOCAL_DIR=""
NODE_LOCAL_DEFAULT_DIR="deploy/node"
NODE_REMOTE_URL="https://github.com/Ithildur/Ithiltir-node.git"
NODE_REPO_SLUG="Ithildur/Ithiltir-node"
BUILD_CHANNEL="release"

usage() {
  cat <<'EOF'
Usage:
  scripts/package.sh [-o OUT_DIR] [-t TARGETS] [--version VERSION|--use-git-tag] [--node-version VERSION] [--node-local|--node-local-dir DIR] [--release] [-z|-zip|--zip|--tar-gz]

Options:
  -o OUT_DIR        Output directory (default: release)
  -t TARGETS        Target os/arch list. Repeatable or comma-separated (default: linux/amd64)
  --version VERSION Build version to inject into dash (default: 0.0.0-dev)
  --use-git-tag     Use the single git tag pointing at HEAD as the build version
  --node-version VERSION
                    Ithiltir-node release version to bundle
                    (default: latest compatible remote tag; prerelease builds fall back to release when release is newer)
  --node-local      Copy node binaries from deploy/node instead of downloading release assets
  --node-local-dir DIR
                    Copy node binaries from local deploy directory or Ithiltir-node-* release binaries
  --release         Require --use-git-tag and a strict SemVer release tag:
                    MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
  -z|-zip|--zip     Write zip file
  --tar-gz          Write tar.gz file (recommended for linux targets)

Examples:
  scripts/package.sh
  scripts/package.sh -o release -t linux/amd64,linux/arm64
  scripts/package.sh -o release -t linux/amd64 -t windows/amd64
  scripts/package.sh -o release -t linux/amd64 -z
  scripts/package.sh -o release -t linux/amd64 --tar-gz
  scripts/package.sh -o release -t linux/amd64 -zip
  scripts/package.sh --version 1.2.3-alpha.1 --node-version 1.2.3-alpha.1 -o release -t linux/amd64 --tar-gz
  scripts/package.sh --version 1.2.3-alpha.1 --node-version 1.2.3-alpha.1 --node-local -o release -t linux/amd64 --tar-gz
  scripts/package.sh --use-git-tag --release -o release -t linux/amd64 --tar-gz
EOF
}

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

version_tool() {
  if ! command -v go >/dev/null 2>&1; then
    echo "go is required to validate release versions" >&2
    return 1
  fi
  go -C "$repo_root" run ./cmd/releasever "$@"
}

get_git_tag() {
  if [[ "${GITHUB_REF_TYPE:-}" == "tag" && -n "${GITHUB_REF_NAME:-}" ]]; then
    printf '%s\n' "$GITHUB_REF_NAME"
    return
  fi

  local tags
  tags="$(git -C "$repo_root" tag --points-at HEAD 2>/dev/null | sed '/^$/d' || true)"
  local count
  count="$(printf '%s\n' "$tags" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [[ "$count" != "1" ]]; then
    echo "current commit must have exactly one git tag" >&2
    if [[ -n "$tags" ]]; then
      printf '%s\n' "$tags" >&2
    fi
    return 1
  fi

  printf '%s\n' "$tags"
}

resolve_version() {
  if [[ "$USE_GIT_TAG" == "true" ]]; then
    if ! VERSION="$(get_git_tag)"; then
      exit 1
    fi
  fi

  if [[ "$RELEASE_MODE" == "true" && "$USE_GIT_TAG" != "true" ]]; then
    echo "release mode requires --use-git-tag" >&2
    exit 2
  fi

  if [[ -z "$VERSION" ]]; then
    VERSION="0.0.0-dev"
  fi
  VERSION="$(trim "$VERSION")"

  if BUILD_CHANNEL="$(version_tool channel "$VERSION" 2>/dev/null)"; then
    return
  fi

  BUILD_CHANNEL="release"
  if [[ "$RELEASE_MODE" == "true" ]]; then
    echo "release version must be strict SemVer without a v prefix: $VERSION" >&2
    exit 2
  fi

  echo "version must be strict SemVer without a v prefix: $VERSION" >&2
  exit 2
}

latest_remote_tag() {
  local remote_url="$1"
  local channel="$2"
  local refs
  local tags latest

  if refs="$(git ls-remote --tags --refs --sort='v:refname' "$remote_url" 2>/dev/null)"; then
    :
  else
    refs="$(git ls-remote --tags --refs "$remote_url")"
  fi

  tags="$(printf '%s\n' "$refs" | awk '{print $2}' | sed 's#^refs/tags/##' | sed '/^$/d')"
  latest="$(version_tool latest "$channel" <<<"$tags" 2>/dev/null)" || return 1
  printf '%s\n' "$latest"
}

resolve_node_version() {
  if [[ -z "$NODE_RELEASE_VERSION" && -n "${ITHILTIR_NODE_VERSION:-}" ]]; then
    NODE_RELEASE_VERSION="$ITHILTIR_NODE_VERSION"
  fi

  if [[ -n "$NODE_LOCAL_DIR" && -z "$NODE_RELEASE_VERSION" ]]; then
    echo "local node packaging requires --node-version or ITHILTIR_NODE_VERSION" >&2
    exit 2
  fi

  if [[ -z "$NODE_RELEASE_VERSION" ]]; then
    if ! command -v git >/dev/null 2>&1; then
      echo "git is required to resolve latest Ithiltir-node release tag" >&2
      exit 1
    fi
    if ! NODE_RELEASE_VERSION="$(latest_remote_tag "$NODE_REMOTE_URL" "$BUILD_CHANNEL")"; then
      echo "no valid compatible Ithiltir-node tags found on $NODE_REMOTE_URL for $BUILD_CHANNEL build" >&2
      exit 1
    fi
  fi

  NODE_RELEASE_VERSION="$(trim "$NODE_RELEASE_VERSION")"
  if ! version_tool validate "$NODE_RELEASE_VERSION" >/dev/null 2>&1; then
    echo "node version must be strict SemVer without a v prefix: $NODE_RELEASE_VERSION" >&2
    exit 2
  fi

  if [[ "$RELEASE_MODE" == "true" ]]; then
    local node_channel
    node_channel="$(version_tool channel "$NODE_RELEASE_VERSION")"
    if [[ "$BUILD_CHANNEL" == "release" && "$node_channel" != "release" ]]; then
      echo "node version channel ($node_channel) must match dash version channel ($BUILD_CHANNEL)" >&2
      exit 2
    fi
  fi
}

set_node_permissions() {
  local deploy_dir="$1"

  chmod 755 "$deploy_dir/linux/node_linux_amd64" \
    "$deploy_dir/linux/node_linux_arm64" \
    "$deploy_dir/macos/node_macos_arm64"
  chmod 644 "$deploy_dir/windows/node_windows_amd64.exe" \
    "$deploy_dir/windows/node_windows_arm64.exe" \
    "$deploy_dir/windows/runner_windows_amd64.exe" \
    "$deploy_dir/windows/runner_windows_arm64.exe"
}

download_file() {
  local url="$1"
  local output="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --connect-timeout 10 --max-time 600 -o "$output" "$url"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -O "$output" "$url"
    return
  fi
  echo "curl or wget is required to download Ithiltir-node release assets" >&2
  exit 1
}

download_node_asset() {
  local asset="$1"
  local output="$2"
  local url

  url="https://github.com/${NODE_REPO_SLUG}/releases/download/${NODE_RELEASE_VERSION}/${asset}"
  echo "downloading node asset: $url"
  mkdir -p "$(dirname "$output")"
  download_file "$url" "$output"
}

prepare_remote_node_deploy() {
  local deploy_dir="$1"

  rm -rf "$deploy_dir"
  mkdir -p "$deploy_dir/linux" "$deploy_dir/macos" "$deploy_dir/windows"

  download_node_asset "Ithiltir-node-linux-amd64" "$deploy_dir/linux/node_linux_amd64"
  download_node_asset "Ithiltir-node-linux-arm64" "$deploy_dir/linux/node_linux_arm64"
  download_node_asset "Ithiltir-node-macos-arm64" "$deploy_dir/macos/node_macos_arm64"
  download_node_asset "Ithiltir-node-windows-amd64.exe" "$deploy_dir/windows/node_windows_amd64.exe"
  download_node_asset "Ithiltir-node-windows-arm64.exe" "$deploy_dir/windows/node_windows_arm64.exe"
  download_node_asset "Ithiltir-runner-windows-amd64.exe" "$deploy_dir/windows/runner_windows_amd64.exe"
  download_node_asset "Ithiltir-runner-windows-arm64.exe" "$deploy_dir/windows/runner_windows_arm64.exe"

  set_node_permissions "$deploy_dir"
}

copy_local_node_asset() {
  local output="$1"
  shift

  local source
  for source in "$@"; do
    if [[ -f "$source" ]]; then
      mkdir -p "$(dirname "$output")"
      cp "$source" "$output"
      return
    fi
  done

  echo "local node asset not found. tried:" >&2
  printf '  %s\n' "$@" >&2
  exit 1
}

prepare_local_node_deploy() {
  local source_dir="$1"
  local deploy_dir="$2"

  if [[ ! -d "$source_dir" ]]; then
    echo "local node deploy directory not found: $source_dir" >&2
    exit 1
  fi

  rm -rf "$deploy_dir"
  mkdir -p "$deploy_dir/linux" "$deploy_dir/macos" "$deploy_dir/windows"

  copy_local_node_asset "$deploy_dir/linux/node_linux_amd64" \
    "$source_dir/linux/node_linux_amd64" \
    "$source_dir/linux/Ithiltir-node-linux-amd64" \
    "$source_dir/Ithiltir-node-linux-amd64"
  copy_local_node_asset "$deploy_dir/linux/node_linux_arm64" \
    "$source_dir/linux/node_linux_arm64" \
    "$source_dir/linux/Ithiltir-node-linux-arm64" \
    "$source_dir/Ithiltir-node-linux-arm64"
  copy_local_node_asset "$deploy_dir/macos/node_macos_arm64" \
    "$source_dir/macos/node_macos_arm64" \
    "$source_dir/macos/Ithiltir-node-macos-arm64" \
    "$source_dir/Ithiltir-node-macos-arm64"
  copy_local_node_asset "$deploy_dir/windows/node_windows_amd64.exe" \
    "$source_dir/windows/node_windows_amd64.exe" \
    "$source_dir/windows/Ithiltir-node-windows-amd64.exe" \
    "$source_dir/Ithiltir-node-windows-amd64.exe"
  copy_local_node_asset "$deploy_dir/windows/node_windows_arm64.exe" \
    "$source_dir/windows/node_windows_arm64.exe" \
    "$source_dir/windows/Ithiltir-node-windows-arm64.exe" \
    "$source_dir/Ithiltir-node-windows-arm64.exe"
  copy_local_node_asset "$deploy_dir/windows/runner_windows_amd64.exe" \
    "$source_dir/windows/runner_windows_amd64.exe" \
    "$source_dir/windows/Ithiltir-runner-windows-amd64.exe" \
    "$source_dir/Ithiltir-runner-windows-amd64.exe"
  copy_local_node_asset "$deploy_dir/windows/runner_windows_arm64.exe" \
    "$source_dir/windows/runner_windows_arm64.exe" \
    "$source_dir/windows/Ithiltir-runner-windows-arm64.exe" \
    "$source_dir/Ithiltir-runner-windows-arm64.exe"

  set_node_permissions "$deploy_dir"
}

set_archive_format() {
  local format="$1"

  if [[ -n "$ARCHIVE_FORMAT" && "$ARCHIVE_FORMAT" != "$format" ]]; then
    echo "archive format already set to $ARCHIVE_FORMAT" >&2
    exit 2
  fi

  ARCHIVE_FORMAT="$format"
}

detect_zip_package_manager() {
  if [[ "$(uname -s)" == "Darwin" ]]; then
    if command -v brew >/dev/null 2>&1; then
      echo "brew"
      return 0
    fi
  fi

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
        if command -v dnf >/dev/null 2>&1; then
          echo "dnf"
        else
          echo "yum"
        fi
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
          if command -v dnf >/dev/null 2>&1; then
            echo "dnf"
          else
            echo "yum"
          fi
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

  for pm in apt-get dnf yum pacman apk zypper brew; do
    if command -v "$pm" >/dev/null 2>&1; then
      echo "$pm"
      return 0
    fi
  done

  return 1
}

run_with_privileges() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo "$@"
    return
  fi

  echo "need root or sudo to install zip automatically" >&2
  return 1
}

ensure_zip() {
  if command -v zip >/dev/null 2>&1; then
    return 0
  fi

  local pm
  if ! pm="$(detect_zip_package_manager)"; then
    echo "zip not found and no supported package manager detected" >&2
    return 1
  fi

  echo "zip not found. installing via $pm..." >&2
  case "$pm" in
    apt-get)
      run_with_privileges apt-get update
      run_with_privileges apt-get install -y zip
      ;;
    dnf)
      run_with_privileges dnf install -y zip
      ;;
    yum)
      run_with_privileges yum install -y zip
      ;;
    pacman)
      run_with_privileges pacman -Sy --noconfirm zip
      ;;
    apk)
      run_with_privileges apk add zip
      ;;
    zypper)
      run_with_privileges zypper --non-interactive install zip
      ;;
    brew)
      brew install zip
      ;;
    *)
      echo "unsupported package manager: $pm" >&2
      return 1
      ;;
  esac

  if ! command -v zip >/dev/null 2>&1; then
    echo "zip installation finished but zip is still unavailable in PATH" >&2
    return 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      OUT_DIR="${2:-}"
      [[ -n "$OUT_DIR" ]] || { echo "missing value for -o" >&2; usage; exit 2; }
      shift 2
      ;;
    -t)
      target_value="${2:-}"
      [[ -n "$target_value" ]] || { echo "missing value for -t" >&2; usage; exit 2; }
      TARGETS+=("$target_value")
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      [[ -n "$VERSION" ]] || { echo "missing value for --version" >&2; usage; exit 2; }
      shift 2
      ;;
    --use-git-tag)
      USE_GIT_TAG="true"
      shift
      ;;
    --node-version)
      NODE_RELEASE_VERSION="${2:-}"
      [[ -n "$NODE_RELEASE_VERSION" ]] || { echo "missing value for --node-version" >&2; usage; exit 2; }
      shift 2
      ;;
    --node-local)
      [[ -n "$NODE_LOCAL_DIR" ]] || NODE_LOCAL_DIR="$NODE_LOCAL_DEFAULT_DIR"
      shift
      ;;
    --node-local-dir)
      NODE_LOCAL_DIR="${2:-}"
      [[ -n "$NODE_LOCAL_DIR" ]] || { echo "missing value for --node-local-dir" >&2; usage; exit 2; }
      shift 2
      ;;
    --release)
      RELEASE_MODE="true"
      shift
      ;;
    -z|-zip|--zip)
      set_archive_format "zip"
      shift
      ;;
    --tar-gz)
      set_archive_format "tar.gz"
      shift
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
if [[ -n "$NODE_LOCAL_DIR" && "$NODE_LOCAL_DIR" != /* ]]; then
  NODE_LOCAL_DIR="$repo_root/$NODE_LOCAL_DIR"
fi
if ! command -v go >/dev/null 2>&1; then
  echo "go is required to build dash" >&2
  exit 1
fi
resolve_version
resolve_node_version
ldflags="-s -w -X dash/internal/version.Current=${VERSION} -X dash/internal/version.BundledNode=${NODE_RELEASE_VERSION}"
echo "dash version: $VERSION"
echo "dash channel: $BUILD_CHANNEL"
echo "node version: $NODE_RELEASE_VERSION"
if [[ -n "$NODE_LOCAL_DIR" ]]; then
  echo "node source: $NODE_LOCAL_DIR"
fi

if [[ "$OUT_DIR" == /* ]]; then
  out_root="$OUT_DIR"
else
  out_root="$repo_root/$OUT_DIR"
fi

mkdir -p "$out_root"

dist_root="$repo_root/build"
node_deploy_dir="$dist_root/node-deploy"

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  TARGETS=("linux/amd64")
fi

target_list=()
for raw_targets in "${TARGETS[@]}"; do
  IFS=',' read -r -a split_targets <<<"$raw_targets"
  for t in "${split_targets[@]}"; do
    t="${t//[[:space:]]/}"
    [[ -n "$t" ]] || continue
    target_list+=("$t")
  done
done

if [[ ${#target_list[@]} -eq 0 ]]; then
  echo "no valid targets provided" >&2
  exit 2
fi

for t in "${target_list[@]}"; do
  os="${t%%/*}"
  arch="${t#*/}"
  if [[ -z "$os" || -z "$arch" || "$os" == "$arch" ]]; then
    echo "invalid target: $t (expected os/arch)" >&2
    exit 2
  fi
done

if [[ -n "$NODE_LOCAL_DIR" ]]; then
  prepare_local_node_deploy "$NODE_LOCAL_DIR" "$node_deploy_dir"
else
  prepare_remote_node_deploy "$node_deploy_dir"
fi

binary_path_for_target() {
  local os="$1"
  local arch="$2"
  local os_dir="$os"
  local bin_name="dash_${arch}"

  if [[ "$os" == "darwin" ]]; then
    os_dir="macos"
  fi
  if [[ "$os" == "windows" ]]; then
    bin_name="${bin_name}.exe"
  fi

  echo "$dist_root/$os_dir/$bin_name"
}

build_dash_binary() {
  local os="$1"
  local arch="$2"
  local output
  output="$(binary_path_for_target "$os" "$arch")"

  mkdir -p "$(dirname "$output")"
  echo "building dash for ${os}/${arch}: $output"
  (
    cd "$repo_root"
    env GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o "$output" ./cmd/dash
  )
}

for t in "${target_list[@]}"; do
  os="${t%%/*}"
  arch="${t#*/}"
  if [[ -z "$os" || -z "$arch" || "$os" == "$arch" ]]; then
    echo "invalid target: $t (expected os/arch)" >&2
    exit 2
  fi
  build_dash_binary "$os" "$arch"
done

bash "$repo_root/scripts/build_frontend.sh" -o "$FRONTEND_DIST_DIR"

frontend_dist_dir="$repo_root/$FRONTEND_DIST_DIR"
if [[ ! -d "$frontend_dist_dir" ]]; then
  echo "frontend build output not found: $frontend_dist_dir" >&2
  exit 1
fi

if [[ "$ARCHIVE_FORMAT" == "zip" ]]; then
  ensure_zip
fi

if [[ "$ARCHIVE_FORMAT" == "tar.gz" ]] && ! command -v tar >/dev/null 2>&1; then
  echo "tar is required to create tar.gz archives" >&2
  exit 1
fi

for t in "${target_list[@]}"; do
  os="${t%%/*}"
  arch="${t#*/}"
  if [[ -z "$os" || -z "$arch" || "$os" == "$arch" ]]; then
    echo "invalid target: $t (expected os/arch)" >&2
    exit 2
  fi

  build_root="$out_root/build"
  pkg_root="$build_root/Ithiltir-dash"
  rm -rf "$pkg_root"
  mkdir -p "$build_root"
  mkdir -p "$pkg_root/bin" "$pkg_root/logs"

  exe_name="dash"
  if [[ "$os" == "windows" ]]; then
    exe_name="dash.exe"
  fi

  source_bin="$(binary_path_for_target "$os" "$arch")"
  if [[ ! -f "$source_bin" ]]; then
    echo "dash build output not found: $source_bin" >&2
    exit 1
  fi

  cp "$source_bin" "$pkg_root/bin/$exe_name"

  cp -R "$repo_root/configs" "$pkg_root/configs"
  cp -R "$frontend_dist_dir" "$pkg_root/dist"
  cp -R "$node_deploy_dir" "$pkg_root/deploy"
  cp "$repo_root/install_dash_linux.sh" "$repo_root/update_dash_linux.sh" "$pkg_root/"

  if [[ "$os" != "windows" ]]; then
    chmod 755 "$pkg_root/bin/$exe_name"
    find "$pkg_root" -type f -name '*.sh' -exec chmod 755 {} +
  fi

  case "$ARCHIVE_FORMAT" in
    zip)
      zip_path="$out_root/Ithiltir_dash_${os}_${arch}.zip"
      rm -f "$zip_path"
      (cd "$build_root" && zip -r "$zip_path" "Ithiltir-dash" >/dev/null)
      ;;
    tar.gz)
      tar_gz_path="$out_root/Ithiltir_dash_${os}_${arch}.tar.gz"
      rm -f "$tar_gz_path"
      (cd "$build_root" && tar -czf "$tar_gz_path" "Ithiltir-dash")
      ;;
  esac
done

echo "done. output: $out_root"
