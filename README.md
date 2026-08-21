# Ithiltir Dash [![release](https://img.shields.io/github/v/release/Ithildur/Ithiltir?label=release)](https://github.com/Ithildur/Ithiltir/releases) [![downloads](https://img.shields.io/github/downloads/Ithildur/Ithiltir/total?label=downloads)](https://github.com/Ithildur/Ithiltir/releases) [![license](https://img.shields.io/github/license/Ithildur/Ithiltir)](LICENSE)

Ithiltir Dash is a single-instance, self-hosted server monitoring dashboard. One Dash process serves the web UI, HTTP API, theme assets, install scripts, and node binary download paths.

**Resources:** [中文](README_CN.md) · [Documentation](https://www.ithiltir.dev/) · [Architecture](docs/architecture.md) · [API](docs/api.md) · [Breaking changes](docs/breaking-changes.md) · [Ithiltir-node](https://github.com/Ithildur/Ithiltir-node)

## Screenshot

![Ithiltir Dash dashboard](screenshot/en.jpg)

## Scope

- Live node dashboard, online-rate data, and metrics history
- Node search, group filters, and guest visibility controls
- PVE / Proxmox VE host support
- RAID checks and failure alerts
- SMART status, critical warnings, SMART temperature, and thermal sensor runtime fields
- Traffic statistics, monthly cycles, and 95th percentile billing data
- Node, group, alert, theme, and system settings management
- Node metrics, static host data, and bundled update manifests
- Built-in Linux, macOS, and Windows Node install scripts
- One process for the SPA, API, admin console, and deployment assets

## Requirements

- PostgreSQL 16+ with TimescaleDB built for the same PostgreSQL major version. Existing compatible versions remain supported; automatic fresh dependency provisioning is pinned to PostgreSQL 16.15 and TimescaleDB 2.29.1
- Redis persists admin sessions by default and stores the disposable frontend cache; `--no-redis` keeps both in process memory. Alert runtime and MTProto login handshakes always stay in memory and reset on restart
- Go 1.26.6+ to run from source or build packages
- Bun 1.3.11 to build the frontend

## Quick Start

1. Copy `configs/config.example.yaml` to `config.local.yaml`.
2. Replace every `__...__` placeholder in `config.local.yaml`.
3. Set an admin password of at least 8 characters in `monitor_dash_pwd`.
4. Run database migrations.
5. Start the server.

```bash
cp configs/config.example.yaml config.local.yaml
export monitor_dash_pwd='<password>'
go run ./cmd/dash migrate -config config.local.yaml
go run ./cmd/dash -debug
```

When it starts, open `app.public_url` for the dashboard and `app.public_url + "/login"` for the admin console. `app.public_url` must be a root URL whose host is an IP literal or ASCII DNS name. Configure internationalized domains in IDNA/punycode form. Path prefixes such as `/dash` are not supported. IP deployments may use HTTP; use HTTPS whenever the network boundary supports it.

## Configuration

Minimum config fields:

- `app.listen`
- `app.public_url`
- `database.driver`
- `database.host`
- `database.port`
- `database.user`
- `database.name`
- `auth.jwt_signing_key`

Redis mode additionally requires `redis.addr`; `--no-redis` does not load or validate Redis configuration.

The admin login password is read only from the `monitor_dash_pwd` environment variable. It must contain at least 8 visible ASCII characters without whitespace.
`auth.jwt_signing_key` must be at least 32 bytes and must not contain surrounding whitespace. The Linux installer generates a 32-byte random alphanumeric key.
`dash migrate` creates the binary notification-configuration key at `$DASH_HOME/configs/notify-config.key` with owner-only permissions. When `DASH_HOME` is unset, Dash uses the discovered application home and maps a managed `releases/<version>` directory back to the stable installation root. Back up this file separately from PostgreSQL and never commit it. A database backup without the matching key cannot recover stored Telegram, SMTP, or webhook credentials; Dash refuses to replace a missing key when encrypted configurations already exist.
When a supported environment variable is present, its value overrides YAML even when it is empty. Required fields then reject the empty value; optional fields apply their documented empty/default semantics, and credential fields may be explicitly cleared. An empty integer environment variable means zero; a non-empty value that is not an integer is a configuration error.
Database pool sizes must be non-negative. A positive `database.max_open_conns` must be at least `database.max_idle_conns`; zero keeps the existing default/unlimited semantics. `database.conn_max_lifetime` must be non-negative, and zero disables age-based expiry.
Redis configuration and `REDIS_*` overrides are loaded only when Redis mode is enabled. Pool values must be non-negative; `redis.pool_size=0` uses the go-redis default and requires `redis.min_idle_conns=0`, while a positive pool size must be at least the minimum idle count. `--no-redis` and migration commands do not read or validate Redis-specific settings.
`redis.username` and `redis.password` are optional authentication fields. The Linux installer prompts for an optional Redis password, validates the endpoint with that password without placing it in the process arguments, and writes it only to the root-readable local config.

`app.timezone` is optional. Empty uses the local timezone; a non-empty value must be a valid IANA timezone name such as `Asia/Shanghai` or `UTC`, otherwise startup stops with a config error.

Config lookup order:

- `config.local.yaml`
- `config.yaml`
- `configs/config.local.yaml`
- `configs/config.yaml`
- `$DASH_HOME/configs/config.local.yaml`
- `$DASH_HOME/configs/config.yaml`

`database.retention_days` is optional and defaults to `45` days. The 5-minute traffic fact table uses independent `database.traffic_retention_days`; when omitted it uses `max(database.retention_days, 45)`. It is kept writable and pruned by rolling retention; historical 95th percentile billing values are stored in monthly snapshots. For 95th percentile billing history, set it to `90` or higher.

## Deployment Baseline

- Fresh-install target: `PostgreSQL 16.15 + TimescaleDB 2.29.1 + Redis`. Existing compatible PostgreSQL 16+ and matching TimescaleDB installations remain unchanged.
- `install_dash_linux.sh` uses signed PostgreSQL/TimescaleDB repositories. When either database dependency is missing, it resolves the distribution-specific package revision for the pinned upstream version and verifies the installed server or extension version; it stops instead of drifting to a newer upstream version when the configured repository no longer carries the pin. It first tries the system package manager for Redis. The installer provisions Redis `8.2.3+` as the recommended baseline; if the packaged Redis is unavailable or older, it can build or upgrade Redis from source (default `8.2.5`). Before replacing an existing Redis configuration or service, or stopping a listener on port 6379, it asks for explicit confirmation with a default of yes and backs up replaced files. Before cutover, the packaged Dash binary validates the configured `redis.addr` and optional `redis.password` with `PING`, `INFO server`, and the supported `6.2.0+` version floor; it does not infer remote service health from a local `redis-server` executable. Default startup repeats the same endpoint validation and logs a warning below the recommended `8.2.3`. `--no-redis` skips Redis connection and version checks.
- Node install script messages follow `app.language`.
- The Dash Linux installer registers a service only when systemd is actually running. It requires `pgrep` so cutover can stop an existing process launched manually from the installed binary. Non-systemd hosts must explicitly use `--service-manager=none`; this installs files and a manual runner without claiming that a service was started or enabled. Its supported lifecycle is one initial run on a fresh host: it is not a reinstall, repair, rollback, or version-update entrypoint, and it is never run concurrently with the updater. It installs only the package content beside `install_dash_linux.sh`, validates the packaged `dash` binary, and does not select an online release. After installation, every version change is executed by `/opt/Ithiltir-dash/bin/dash update`; `update_dash_linux.sh` is only a compatibility wrapper. Managed packages are installed under `/opt/Ithiltir-dash/releases`; one atomic `current` symlink selects the active package, while legacy paths such as `/opt/Ithiltir-dash/bin/dash` remain compatibility aliases and mutable `configs`, `runtime`, `logs`, `themes`, and `install_id` stay outside release directories. Installer backups and recovery paths only contain failure within that one initial run; they do not define a supported rerun or downgrade. The updater holds one root-owned cross-process lock across installed-state validation and cutover. Failure before database migration starts restores the pre-cutover state; after migration starts, the old binary is never restored. A persistent transaction and systemd start guard keep an interrupted migration stopped until `dash update recover` completes it forward. Package staging uses a sibling directory beside `/opt/Ithiltir-dash`, and initial-install cutover requires GNU coreutils `mv`. Alpine Dash hosts must also preinstall and start PostgreSQL 16+, TimescaleDB built for that PostgreSQL major version, and Redis 6.2.0+; PostgreSQL 16.15 and TimescaleDB 2.29.1 are the fresh-environment target, while existing compatible versions are accepted. Redis 8.2.3+ remains recommended.
- The Linux node installer officially supports systemd and Alpine/OpenRC; other OpenRC distributions are best effort. Alpine must have `bash`, `ca-certificates`, `curl`, and `coreutils` available before running the Bash installer. This installer always replaces its managed node release, report configuration, service definition, and collectors; Node version upgrades are owned by the separate node self-update path. It stages the downloaded binary under `/var/lib/ithiltir-node/releases` and executes `--version` before stopping the installed runtime, then force-replaces the target release and atomically switches `current`. The runtime user owns the data/release tree so the unprivileged self-updater can create and switch releases. Downloads follow at most five redirects to the original host; same-scheme hops keep the effective port and only HTTP-to-HTTPS upgrades may change scheme. systemd uses service/timer units for SMART, connection-count, and detected LVM thin-pool collectors. OpenRC uses `supervise-daemon` for the Node process, and BusyBox `crond` refreshes SMART every five minutes and LVM every minute. The one-second root connection helper is not degraded to minute-level cron under OpenRC; the Node uses its built-in connection counting there, which may miss container connections.
- On systemd, Linux node installation compiles a small root-side connections helper when `cc`, `gcc`, or `clang` is available. This helper is required for full host/container network-namespace TCP/UDP counts because the node service runs with low privileges. Install a C compiler with the system package manager and rerun the installer to enable it.
- Recommended minimum: `1 vCPU / 2 GB RAM / 40 GB SSD/NVMe`
- Setups below `4 GB RAM` should enable `SWAP`
- Reverse proxies must preserve same-origin paths: proxy `/api`, `/theme`, and `/deploy` to Dash, and let Dash serve the SPA at `/`
- Prefer HTTPS when exposing Dash beyond a trusted network. Node install commands and asset downloads carry the node secret.
- Do not point browser requests directly at a cross-origin backend unless CORS, cookie, and CSRF policies are designed together

## Development

Standalone frontend development:

```bash
cd web
FRONT_TEST_API=http://127.0.0.1:8080 bun run dev
```

`FRONT_TEST_API` points to a running Dash backend. The Vite dev server proxies only `/api` and `/theme`; frontend code still uses same-origin relative paths.

Common checks:

```bash
go test ./...
cd web && bun run lint
cd web && bun run typecheck
```

Backend integration tests that touch PostgreSQL or TimescaleDB require `TEST_DATABASE_URL`. Without it, those tests are skipped. For a full backend test pass, use a PostgreSQL superuser or a role that can create and drop temporary databases:

```bash
TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable' go test ./...
```

## Build And Package

Build the frontend:

```bash
bash scripts/build_frontend.sh --version 0.0.0-dev -o build/frontend/dist
```

The frontend build embeds the supplied Dash version so an open browser can detect a backend upgrade and reload the matching assets. The default is `0.0.0-dev`; release packaging always passes its resolved Dash version. The official build also fails unless both `dist/index.html` and `dist/theme-bootstrap.js` are non-empty. CI release packaging always runs this build; arbitrary custom frontend distributions are not a supported release input.

Build a Linux package:

```bash
bash scripts/package.sh --version 1.2.3-alpha.1 --node-version 1.2.3-alpha.1 -o release -t linux/amd64 --tar-gz
```

Dash server release packages currently target Linux amd64 and Linux arm64. macOS and Windows deploy assets are Node assets.

The packaging scripts include only `configs/config.example.yaml`; they do not package `config.local.yaml`, `configs/config.local.yaml`, or other local configuration.

PowerShell:

```powershell
powershell -File scripts/build_frontend.ps1 -Version 0.0.0-dev -OutDir build/frontend/dist
powershell -File scripts/package.ps1 -Version 1.2.3-alpha.1 -NodeVersion 1.2.3-alpha.1 -OutDir release -Targets linux/amd64 -Zip
```

Node version source:

- Omit `--node-version` / `-NodeVersion`: use the latest compatible tag from `https://github.com/Ithildur/Ithiltir-node.git`. Prerelease Dash builds use the newer of the latest node prerelease and latest node release, and fall back to the latest node release when no node prerelease exists.
- Pass `--node-local` / `-NodeLocal`: read local binaries from `deploy/node`.

Update an installed Linux service:

```bash
/opt/Ithiltir-dash/bin/dash update --check
sudo /opt/Ithiltir-dash/bin/dash update
sudo /opt/Ithiltir-dash/bin/dash update --test
sudo /opt/Ithiltir-dash/bin/dash update reinstall --test
```

By default the updater installs the latest release. Pass `--test` to install the latest prerelease. If the installed Dash is a prerelease newer than the latest release, the default release update stops with a warning.
Use `reinstall` to install the selected latest package again even when the Dash version is unchanged, for example after repacking a test release with newer bundled node assets.
The built-in manual updater supports systemd and explicit manual installations without requiring Git, tar, curl/wget, or a shell implementation of the update transaction. Before cutover it stops both the selected managed service and any remaining Dash process started manually from the installed binary. A manual installation is not started again automatically. The admin-console background controller still requires `systemd-run`. The packaged `update_dash_linux.sh` accepts the old flags and delegates to `dash update`.
Every Linux release package uses release format v1 and carries `release.env` with `format_version=1`, the Dash version, bundled node version, target OS/architecture, and SHA-256 digests for all seven bundled node/runner assets covering the five supported platform/architecture targets. It must contain a version-matched `bin/dash`, `dist/index.html`, those assets under `deploy`, `configs/config.example.yaml`, and both Linux install/update scripts; required files must be non-empty. Before stopping the live service, the built-in updater validates the runtime tree and archive bounds, checks every bundled asset against the manifest, and requires the candidate Dash binary to report both the manifest Dash version and bundled node version. An older installation reaches this native-updater generation through its existing updater; after that transition, archives without the manifest are rejected. Staging and recovery directories are created beside `/opt/Ithiltir-dash`, on the same filesystem as the installation. If migration or the post-migration start fails, run `sudo /opt/Ithiltir-dash/bin/dash update recover`. Database migrations are forward-only: `goose_db_version` is the schema-version source of truth, normal startup requires it to match the binary's embedded migrations exactly, and manually starting an older binary after an upgrade is unsupported.
The Linux updater treats the official GitHub release source as the root of trust. A compromised GitHub account, token, repository, workflow, or release permission is equivalent to a compromised update source.

## Repository Layout

| Path | Contents |
| --- | --- |
| `cmd/dash` | server, migration, update, and theme packaging entry points |
| `internal` | backend application code |
| `web` | SPA source bundled into the app |
| `configs` | sample config |
| `db/migrations` | SQL migrations executed directly by Goose |
| `db/migrationdata` | SQL bodies executed by Go-owned migrations |
| `scripts` | frontend build and release packaging entry points |
| `deploy/node` | local node binaries for offline packaging |

## License

AGPL-3.0-only. See [LICENSE](LICENSE).
