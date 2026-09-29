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

OpenRC uses `supervise-daemon` for the foreground node process. Alpine BusyBox cron refreshes SMART every five minutes and detected LVM thin-pool state every minute. systemd uses service/timer units. Packaged Linux node binaries must be static or musl-compatible for Alpine. The installer is a force-install entrypoint: it copies the download into a staging directory under `/var/lib/ithiltir-node/releases` and executes `--version` there before stopping systemd, OpenRC, and remaining manually started node processes, then replaces the managed release, report configuration, service definition, and collectors before atomically switching `current`. Reinstalling the same version deliberately replaces that release without installer rollback; recovery for version upgrades belongs to the self-updater. The runtime user owns the data and release tree because the unprivileged self-updater creates and switches releases. A `noexec` system temporary directory therefore does not make a compatible binary look invalid. Agent version upgrades are owned by the separate node self-update path.

When systemd is selected, the installer migrates the legacy `/etc/cron.d/ithiltir-node-thinpool` schedule only after the replacement timer is enabled, active, and completes an initial collection. If activation or the initial collection fails, the timer is disabled and the legacy cron file is retained.

Node install commands use the structured `GET /api/admin/nodes/deploy` response. Current agents download packaged node binaries with `X-Node-Secret`; Dash may include a short-lived, asset-bound query token when upgrading older supported agents.

Linux, macOS, and Windows installers follow at most five download redirects. Every hop must keep the original host; same-scheme hops must keep the effective port, HTTP may upgrade to HTTPS, and HTTPS downgrade or cross-host redirects are rejected before `X-Node-Secret` is sent.

Installer messages follow Dash `app.language`.

## Optional PVE Monitoring

Use `scripts/package.sh --with-pve` (PowerShell: `-WithPVE`) to bundle precompiled helpers from the selected node release. Local inputs are `linux/pve_cache_linux_amd64` and `linux/pve_cache_linux_arm64`, or flat `Ithiltir-pve-cache-linux-<arch>` assets. Both helpers share `--node-version`; packages without this option retain the ordinary node assets and installation behavior.

`deploy/linux/pve-cache.env` contains `format_version=1`, `node_version`, `amd64_sha256`, and `arm64_sha256`. These fields are deliberately separate from the existing `release.env` v1 contract. New Dash updaters verify the optional manifest and both assets; older Dash updaters continue validating their original assets. The Linux PVE installer verifies the selected helper's SHA-256 and executable version before stopping the installed node.

Append `--pve` to the generated Linux install command on a PVE/systemd host. This installs root-owned `/usr/local/libexec/ithiltir-node/pve-cache`, `ithiltir-node-pve-cache.service` running `pve-cache --serve`, and a node service drop-in enabling `ITHILTIR_NODE_VIRT_CACHE=/run/ithiltir-node/virt.json`. The helper queries local QEMU VMs as root; the ordinary node remains unprivileged. No compiler, PVE user or API token is required. PVE collection is not supported by the OpenRC or `none` adapters.

Reinstalling without either PVE flag preserves the existing PVE installation choice. `--no-pve` disables/removes the PVE service, legacy timers, helper, drop-in and public cache. Guest lock files and cooldowns remain until reboot; never unlink live lock files. Reinstall with `--pve` to update the helper and units; node self-update does not modify root-owned assets. Existing node identity and report targets remain managed by `report install`.

Collection runs 30 seconds after each completed attempt, with a shared 20-second query budget and a 90-second cache TTL. Logs: `journalctl -u ithiltir-node-pve-cache.service`. Only the local host's QEMU VMs are reported, including stopped VMs and templates. VM reporting uses a separate authenticated `/api/node/virt` endpoint; administrator reads use `/api/admin/nodes/{id}/virt`. No UI, HA configuration, LXC, or controls are included. VM history requires a gRPC node session and the helper service.

Guest Agent IPs are returned in optional `ips` arrays, up to 128 unique IPv4/IPv6 addresses per VM, including private addresses and excluding loopback, link-local, unspecified, multicast and invalid addresses. PVE must enable the guest agent and the agent must be running inside the VM. Missing IPs do not fail basic collection. IP collection runs independently inside `pve-cache --serve` (manual one-shot: `--guest`): the scheduler wakes 60 seconds after completion, successful VMs wait five minutes, and failures back off for 5, 10, 20, then 30 minutes (maximum). Only running, non-template, non-paused VMs from a fresh inventory are queried. Four workers use five-second query deadlines within a 50-second slow-collection budget. Per-VM file locks are inherited by query processes; timeout kills the process group and waits for exit. A still-running query blocks any replacement for that VM. Cooldowns are saved before launch in root-only `/run/ithiltir-node/pve-guest`; this state is volatile and resets on reboot. Hot collection only reads this cache.


## gRPC deployment

Set `ITHILTIR_NODE_TRANSPORT=grpc` or `auto` in the Node service environment and restart it; `http` remains the default. The report URL and key stay unchanged. RPC uses the same URL origin and the root `/ithiltir.node.v1.Node/` service path; a custom HTTP path prefix does not change that RPC path. TLS termination must advertise HTTP/2. Example Nginx location inside an existing TLS server:

```nginx
location /ithiltir.node.v1.Node/ {
    grpc_pass grpc://127.0.0.1:8080;
    grpc_set_header Host $host;
    grpc_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    grpc_read_timeout 75s;
    grpc_send_timeout 75s;
}
```

Use the actual Dash backend address. Keep the existing HTTP locations for APIs, deployment, themes and the SPA. No UDP or additional public port is required. For systemd Node installations, add `Environment=ITHILTIR_NODE_TRANSPORT=grpc` to a `[Service]` drop-in, run `systemctl daemon-reload`, then restart `ithiltir-node.service`. Reinstall with `--pve` to update the root helper and enable its history socket. Confirm the administrator `/virt/capabilities` endpoint returns both `connected` and `pve_history`. Node self-update does not replace the root helper.

The configured report URL scheme must match the actual endpoint. If a legacy HTTPS URL relied on plaintext fallback, configure the actual trusted-network HTTP URL or a working TLS proxy before enabling gRPC/auto.
