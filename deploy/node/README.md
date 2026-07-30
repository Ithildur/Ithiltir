# Local Node Binaries

## Accepted Layouts

- `linux/node_linux_amd64`
- `linux/node_linux_arm64`
- `macos/node_macos_arm64`
- `windows/node_windows_amd64.exe`
- `windows/node_windows_arm64.exe`
- `windows/runner_windows_amd64.exe`
- `windows/runner_windows_arm64.exe`

## Accepted Flat Asset Names

- `Ithiltir-node-linux-amd64`
- `Ithiltir-node-linux-arm64`
- `Ithiltir-node-macos-arm64`
- `Ithiltir-node-windows-amd64.exe`
- `Ithiltir-node-windows-arm64.exe`
- `Ithiltir-runner-windows-amd64.exe`
- `Ithiltir-runner-windows-arm64.exe`

## Package Command

```bash
bash scripts/package.sh --version 0.0.0-dev.0 --node-version 0.0.0-dev.0 --node-local -o release -t linux/amd64 --tar-gz
```

The package scripts bind every copied node/runner asset into `release.env` by SHA-256. The declared node version is also compiled into Dash; the updater rejects a package when that metadata or any asset digest differs from the manifest.

## Linux Connections Cache

On systemd, the Linux node installer compiles a small root-side helper when `cc`, `gcc`, or `clang` is available. The helper is used only for complete host/container network-namespace TCP/UDP connection counts; if compilation is unavailable, the installed node uses its built-in connection counting, which may miss container connections. OpenRC always uses the built-in counter because its one-second cache interval cannot be represented faithfully by BusyBox cron.

## Linux Runtime

The Linux installer has `systemd`, `openrc`, and `none` service-manager adapters and requires `pgrep` to stop an existing manually launched node before cutover. Auto detection accepts systemd only when `/run/systemd/system` exists, and accepts OpenRC only when `rc-service`, `rc-update`, and `supervise-daemon` are available. Alpine/OpenRC is officially supported; other OpenRC distributions are best effort. `none` must be selected explicitly and only installs files and configuration.

OpenRC uses `supervise-daemon` for the foreground node process. Until periodic collection moves into the node process, Alpine BusyBox cron refreshes SMART every five minutes and detected LVM thin-pool state every minute. systemd keeps the existing service/timer implementation. Packaged Linux node binaries must be static or musl-compatible for Alpine. The installer is a force-install entrypoint: it copies the download into a staging directory under `/var/lib/ithiltir-node/releases` and executes `--version` there before stopping systemd, OpenRC, and remaining manually started node processes, then replaces the managed release, report configuration, service definition, and collectors before atomically switching `current`. Reinstalling the same version deliberately replaces that release without installer rollback; recovery for version upgrades belongs to the self-updater. The runtime user owns the data and release tree because the unprivileged self-updater creates and switches releases. A `noexec` system temporary directory therefore does not make a compatible binary look invalid. Agent version upgrades are owned by the separate node self-update path.

When systemd is selected, the installer migrates the legacy `/etc/cron.d/ithiltir-node-thinpool` schedule only after the replacement timer is enabled, active, and completes an initial collection. If activation or the initial collection fails, the timer is disabled and the legacy cron file is retained.

Node install commands use the structured `GET /api/admin/nodes/deploy` response. Current agents download packaged node binaries with `X-Node-Secret`; Dash may include a short-lived, asset-bound query token when upgrading older supported agents.

Linux, macOS, and Windows installers follow at most five download redirects. Every hop must keep the original host; same-scheme hops must keep the effective port, HTTP may upgrade to HTTPS, and HTTPS downgrade or cross-host redirects are rejected before `X-Node-Secret` is sent.

Installer messages follow Dash `app.language`.
