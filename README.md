# Ithiltir Dash [![release](https://img.shields.io/github/v/release/Ithildur/Ithiltir?label=release)](https://github.com/Ithildur/Ithiltir/releases) [![downloads](https://img.shields.io/github/downloads/Ithildur/Ithiltir/total?label=downloads)](https://github.com/Ithildur/Ithiltir/releases) [![license](https://img.shields.io/github/license/Ithildur/Ithiltir)](LICENSE)

Ithiltir is a self-hosted server monitoring system. Dash receives reports from Ithiltir nodes and provides the dashboard, history, traffic accounting, alerts, and administration in one process.

[中文](README_CN.md) · [Documentation](https://www.ithiltir.dev/) · [Quick start](https://www.ithiltir.dev/docs/QuickStart) · [Demo](https://demo.ithiltir.dev/) · [Ithiltir-node](https://github.com/Ithildur/Ithiltir-node)

## Screenshot

![Ithiltir Dash dashboard](screenshot/en.jpg)

## Features

- Live CPU, memory, disk, network, temperature, process, and connection metrics
- Online-rate data and metric history backed by PostgreSQL and TimescaleDB
- RAID, SMART, NVMe critical-warning, and Proxmox VE host support
- Monthly traffic accounting, configurable billing cycles, and 95th-percentile data
- Alert rules, event history, and Telegram, SMTP, or webhook notifications
- Node grouping, search, guest visibility, themes, and system administration
- Managed Node installation and updates for Linux, macOS, and Windows

## Install

Dash release packages are available for Linux amd64 and arm64. To install the latest stable release:

```bash
ARCH=amd64
curl -fL -o "Ithiltir_dash_linux_${ARCH}.tar.gz" \
  "https://github.com/Ithildur/Ithiltir/releases/latest/download/Ithiltir_dash_linux_${ARCH}.tar.gz"
tar -xzf "Ithiltir_dash_linux_${ARCH}.tar.gz"
cd Ithiltir-dash
sudo bash install_dash_linux.sh --lang en --service-manager=systemd
```

Use `ARCH=arm64` on AArch64 systems. On supported Debian/Ubuntu systems, the installer can prepare PostgreSQL, TimescaleDB, Redis, migrations, and the systemd service. Other deployment paths are covered in the [installation guide](https://www.ithiltir.dev/docs/Install).

After installation, open the configured `app.public_url`; append `/login` for the admin console. Create a node there and follow the generated install command.

## Run From Source

Source development requires Go 1.26.6+, Bun 1.3.11, PostgreSQL 16+ with a matching TimescaleDB build, and Redis unless Dash is started with `--no-redis`.

```bash
cp configs/config.example.yaml config.local.yaml
# Replace the __...__ placeholders before continuing.
export monitor_dash_pwd='<password>'
go run ./cmd/dash migrate -config config.local.yaml
go run ./cmd/dash -debug
```

The admin password must contain at least eight visible ASCII characters without whitespace. See the [quick start](https://www.ithiltir.dev/docs/QuickStart) for the full source setup.

## Operational Notes

- Dash is single-instance. Do not run multiple Dash processes against the same state.
- `app.public_url` must be a root URL; path prefixes such as `/dash` are unsupported.
- Use HTTPS outside a trusted network. Browser, API, theme, and deployment paths must remain same-origin.
- Redis stores admin sessions and disposable frontend state by default. With `--no-redis`, both are process-local and reset on restart.
- Back up `$DASH_HOME/configs/notify-config.key` separately from PostgreSQL. Notification credentials cannot be recovered without the matching key.

Storage retention, capacity planning, backup, proxy, and security guidance lives in the [operations documentation](https://www.ithiltir.dev/docs/Operations).

## Update

```bash
/opt/Ithiltir-dash/bin/dash update --check
sudo /opt/Ithiltir-dash/bin/dash update
sudo /opt/Ithiltir-dash/bin/dash update --test
```

The default channel installs stable releases; `--test` selects prereleases. If an interrupted migration requires recovery, run:

```bash
sudo /opt/Ithiltir-dash/bin/dash update recover
```

Read the [breaking changes](docs/breaking-changes.md) before upgrading across versions with migration notes.

## Development

```bash
go test ./...
cd web
bun run lint
bun run typecheck
```

Database integration tests require `TEST_DATABASE_URL`. Frontend development can proxy a running Dash:

```bash
cd web
FRONT_TEST_API=http://127.0.0.1:8080 bun run dev
```

Build the frontend and a Linux release package with:

```bash
bash scripts/build_frontend.sh --version 0.0.0-dev -o build/frontend/dist
bash scripts/package.sh --version 0.0.0-dev.0 --node-version 0.0.0-dev.0 \
  -o release -t linux/amd64 --tar-gz
```

Local Node asset layouts are documented in [deploy/node/README.md](deploy/node/README.md).

## Documentation

| Document                                                | Contents                                                   |
| ------------------------------------------------------- | ---------------------------------------------------------- |
| [Quick start](https://www.ithiltir.dev/docs/QuickStart) | Install Dash and connect the first node                    |
| [Installation](https://www.ithiltir.dev/docs/Install)   | Production deployment, proxies, platforms, and upgrades    |
| [Operations](https://www.ithiltir.dev/docs/Operations)  | Backup, retention, capacity, security, and troubleshooting |
| [Architecture](docs/architecture.md)                    | Runtime ownership, data flow, storage, alerts, and updates |
| [API](docs/api.md)                                      | HTTP routes and wire contracts                             |
| [Breaking changes](docs/breaking-changes.md)            | Migration and compatibility notes                          |

## License

AGPL-3.0-only. See [LICENSE](LICENSE).
