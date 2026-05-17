# Architecture

Ithiltir Dash is a single-instance application. The root entry point starts one HTTP process that wires the API, SPA, theme assets, install scripts, node asset downloads, and background services.

## Runtime Boundaries

| Component                | Responsibility                                                                                                        |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------- |
| `cmd/dash`               | process entry, config loading, dependency wiring, migration entry, and shutdown                                       |
| HTTP server              | mounts `/api`, `/theme`, `/deploy`, and the SPA                                                                       |
| PostgreSQL + TimescaleDB | durable metrics history, traffic facts, node metadata, alert rules, alert notification outbox, and system settings    |
| Redis                    | sessions, hot snapshots, and alert runtime state in the default mode                                                  |
| process memory           | node auth index and volatile agent update requests; sessions and hot runtime state when `--no-redis` is used          |
| agent                    | pushes metrics and static host data; receives update manifests                                                        |
| Linux SMART cache timer  | root-side `smartctl` helper writes `/run/ithiltir-node/smart.json`; the agent reads the cache without running as root |
| web UI                   | reads dashboard data and submits admin operations                                                                     |

## HTTP Surface

| Prefix            | Role                                                    |
| ----------------- | ------------------------------------------------------- |
| `/api/auth`       | login, refresh, logout, session revoke                  |
| `/api/version`    | Dash version and bundled node version                   |
| `/api/front`      | dashboard reads                                         |
| `/api/metrics`    | metrics history and online-rate queries                 |
| `/api/statistics` | statistics access policy and traffic statistics queries |
| `/api/node`       | agent pushes and node identity reads                    |
| `/api/admin`      | admin writes                                            |
| `/theme`          | active theme CSS, theme manifest, and preview images    |
| `/deploy`         | install scripts and packaged node assets                |
| `/`               | SPA                                                     |

## Data Flow

1. Agents submit metrics and static host data through `/api/node/*`.
2. Successful metrics responses can include an update manifest.
3. PostgreSQL + TimescaleDB store durable history, traffic facts, configuration, and alert notification outbox rows.
4. Redis or process memory stores hot snapshots, sessions, and alert runtime state.
5. Background services evaluate alerts, deliver queued notifications, and aggregate traffic data.

Node IP is an observation from authenticated agent requests: Dash reads the first IP in `X-Forwarded-For` when that header is present, otherwise it falls back to `RemoteAddr`; invalid values are not used. This field is for display and operations, not an auth boundary.

## State And Retention

- Default startup needs PostgreSQL and Redis; with `--no-redis`, Redis-backed runtime state moves to process memory.
- Node auth and pending agent update requests use process memory, not Redis.
- SMART and thermal metrics are runtime state. SMART cache freshness, helper availability, and device health are kept in a separate hot cache and are not written to PostgreSQL metrics snapshots. SMART temperature for confirmed physical disks is reduced into `disk_physical_metrics.temp_c` for per-device history; virtual disks and RAID devices are ignored. The same backend decision produces `disk.temperature_devices` for frontend history navigation. Thermal data is stored with metrics snapshots, reduced into `cpu_temp_c`, and split into a separate frontend field cache; both runtime fields are composed into the frontend node JSON on read.
- Alert evaluation reads hot snapshots. Built-in offline, RAID, SMART health, and NVMe critical warning rules are derived from snapshot freshness and reported disk state.
- Alert events are not opened during the first minute after alert service startup.
- Alert events and runtime state are committed independently from notification delivery. When notification targets are available, notification payloads are stored in the PostgreSQL outbox and delivered by a leased retry worker; if targets cannot be loaded, the transition is committed without new outbox rows.
- Durable history retention defaults to `45 days`; regular monitoring uses `database.retention_days`, while 5-minute traffic facts use `database.traffic_retention_days`.
- Traffic `lite` mode keeps monthly in/out totals and estimated peaks. Traffic `billing` mode also maintains 5-minute facts, daily summaries, P95, coverage, and monthly snapshots.
- Global billing cycle settings are defaults. Nodes may override cycle mode, billing start day, anchor date, and timezone; traffic reads, monthly usage backfill, and monthly snapshots use each node's effective cycle.
- Traffic direction mode changes the selected billing view only; raw inbound and outbound counters remain stored separately.
- Metrics history is private by default. `history_guest_access_mode=by_node` exposes it only for guest-visible nodes.
- Traffic statistics guest access is controlled by traffic settings and still respects node visibility.

## Auth Boundaries

| Area                                                  | Auth                                                                    |
| ----------------------------------------------------- | ----------------------------------------------------------------------- |
| `/api/auth/login`                                     | admin password                                                          |
| `/api/auth/refresh`, `/api/auth/logout`               | refresh cookie + `X-CSRF-Token`                                         |
| `/api/front/*`, `/api/metrics/*`, `/api/statistics/*` | optional bearer; anonymous requests are filtered by visibility settings |
| `/api/node/*`                                         | `X-Node-Secret`                                                         |
| `/api/admin/*`                                        | `Authorization: Bearer <access_token>`                                  |

## Frontend And Reverse Proxies

The frontend can run as a standalone dev server, but the runtime boundary stays same-origin. A dev proxy or production reverse proxy should forward `/api`, `/theme`, and `/deploy` to the backend while Dash serves the SPA at `/`. Do not point browser API requests directly at a cross-origin backend unless CORS, cookie, and CSRF policies are designed together.

## Repository Layout

| Path                          | Contents                                                      |
| ----------------------------- | ------------------------------------------------------------- |
| `cmd/dash`                    | server, migration, and theme packaging entry points           |
| `internal/config`             | config loading, defaults, validation, and runtime directories |
| `internal/transport/http`     | HTTP server, static assets, theme assets, and API mounting    |
| `internal/transport/http/api` | `/api` route tree                                             |
| `internal/store`              | persistence and cache access layer                            |
| `internal/alert`              | alert compilation, runtime, and delivery orchestration        |
| `internal/traffic`            | traffic statistics background service                         |
| `web`                         | SPA frontend source                                           |
| `configs`                     | sample config                                                 |
| `db/migrations`               | schema changes                                                |
| `scripts`                     | build and packaging entry points                              |
