# API

This document summarizes the stable HTTP contract. Public paths, methods, and field semantics are compatibility boundaries: existing meaning is not changed in place, and new behavior uses new endpoints or added fields.

## Basics

- API base path: `/api`
- Dash is served from root paths only. Path prefixes in `app.public_url` are not supported.
- JSON error format:

```json
{ "code": "<string>", "message": "<string>" }
```

## Auth Model

| Method                                 | Usage                                             |
| -------------------------------------- | ------------------------------------------------- |
| admin password                         | `POST /api/auth/login`                            |
| refresh cookie + `X-CSRF-Token`        | `POST /api/auth/refresh`, `POST /api/auth/logout` |
| `Authorization: Bearer <access_token>` | admin APIs and optional authenticated reads       |
| `X-Node-Secret`                        | agent pushes and node identity reads              |

Optional bearer endpoints treat a missing, malformed, expired, revoked, or otherwise invalid bearer token as an anonymous request. This is intentional compatibility behavior: clients that need admin data must check whether the response is the authenticated view or the guest-filtered view.

## Namespaces

| Prefix                                        | Auth                                                                                                      | Resources                                                                                                                         |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `/api/auth`                                   | admin password for login; refresh cookie + `X-CSRF-Token` for refresh and logout; bearer for `/sessions*` | `POST /login`, `POST /refresh`, `POST /logout`, session revoke                                                                    |
| `/api/version`                                | none                                                                                                      | `GET /`                                                                                                                           |
| `/api/front`                                  | optional bearer                                                                                           | `GET /brand`, `GET /metrics`, `GET /groups`                                                                                       |
| `/api/metrics`                                | optional bearer; metrics history is authenticated by default                                              | `GET /online`, `GET /history`                                                                                                     |
| `/api/statistics`                             | optional bearer                                                                                           | `GET /access`                                                                                                                     |
| `/api/statistics/traffic`                     | optional bearer; `PATCH /settings` requires bearer                                                        | `GET /settings`, `PATCH /settings`, `GET /ifaces`, `GET /summary`, `GET /daily`, `GET /monthly`                                   |
| `/api/node`                                   | `X-Node-Secret`                                                                                           | `POST /identity`, `POST /metrics`, `POST /static`                                                                                 |
| `/api/admin/groups`                           | bearer                                                                                                    | `GET /`, `GET /map`, `POST /`, `PATCH /{id}`, `DELETE /{id}`                                                                      |
| `/api/admin/nodes`                            | bearer                                                                                                    | `GET /`, `GET /deploy`, `POST /`, `PUT /display-order`, `PATCH /traffic-p95`, `PATCH /{id}`, `POST /{id}/upgrade`, `DELETE /{id}` |
| `/api/admin/alerts/rules`                     | bearer                                                                                                    | `GET /`, `POST /`, `PATCH /{id}`, `DELETE /{id}`                                                                                  |
| `/api/admin/alerts/mounts`                    | bearer                                                                                                    | `GET /`, `PUT /`                                                                                                                  |
| `/api/admin/alerts/settings`                  | bearer                                                                                                    | `GET /`, `PUT /`                                                                                                                  |
| `/api/admin/alerts/channels`                  | bearer                                                                                                    | `GET /`, `POST /`, `GET /{id}`, `PUT /{id}`, `PUT /{id}/enabled`, `POST /{id}/test`, `DELETE /{id}`                               |
| `/api/admin/alerts/channels/telegram/mtproto` | bearer                                                                                                    | `POST /code`, `POST /verify`, `POST /password`, `POST /ping`                                                                      |
| `/api/admin/system/settings`                  | bearer                                                                                                    | `GET /`, `PUT /`, `PATCH /`                                                                                                       |
| `/api/admin/system/themes`                    | bearer                                                                                                    | `GET /`, `POST /upload`, `POST /{id}/apply`, `DELETE /{id}`                                                                       |

## Anonymous Reads

- `/api/front/brand` is public.
- `/api/front/metrics` and `/api/front/groups` allow anonymous reads, but anonymous results include only guest-visible nodes.
- `/api/metrics/online` allows anonymous reads for guest-visible nodes.
- `/api/metrics/history` requires bearer by default. If `history_guest_access_mode` is `by_node`, anonymous reads are limited to guest-visible nodes.
- `/api/statistics/access` is public.
- Anonymous reads under `/api/statistics/traffic/*` are controlled by traffic settings and still respect node guest visibility.
- `GET /api/front/metrics` node metadata includes `node.tags` as a string array when tags are configured for the node.

## Admin Nodes

- `GET /api/admin/nodes/` includes `traffic_p95_enabled`, `traffic_cycle_mode`, `traffic_billing_start_day`, `traffic_billing_anchor_date`, `traffic_billing_timezone`, `tags`, and `version`. `tags` is always a string array.
- `version.version` is the last reported agent version. `version.is_outdated` is true when it is missing, invalid, or older than the bundled node version.
- `PATCH /api/admin/nodes/{id}` accepts `traffic_p95_enabled`, `tags`, and node billing cycle override fields. Omitted fields are unchanged. `tags` accepts a string array; values are trimmed, empty values and duplicates are removed, and `[]` clears tags. `traffic_cycle_mode` allows `default`, `calendar_month`, `whmcs_compatible`, and `clamp_to_month_end`.
- `PATCH /api/admin/nodes/traffic-p95` accepts `ids` and `enabled`. `enabled` is required. `ids` must be a non-empty positive integer array, cannot contain duplicates, and is capped at 10000 entries. The command validates every node ID first, then updates all selected nodes in one transaction. Success returns `204`; missing or deleted nodes return `404 not_found` and no node is updated.
- Invalid `tags` returns `400 invalid_tags`.
- Node cycle normalization is stable: `default` inherits the global billing cycle and clears stored node cycle fields; `calendar_month` stores `traffic_billing_start_day=1`; non-`whmcs_compatible` modes store an empty `traffic_billing_anchor_date`; non-default modes with an empty `traffic_billing_timezone` use the application timezone at read time.
- Invalid node cycle fields return `400 invalid_traffic_cycle_mode`, `invalid_traffic_billing_start_day`, `invalid_traffic_billing_anchor_date`, or `invalid_traffic_billing_timezone`.
- `POST /api/admin/nodes/{id}/upgrade` returns `204` or `409` when the bundled version, platform, or asset is unavailable.

## Agent Updates

- Successful `POST /api/node/metrics` responses include `update`.
- `update` is `null` when no upgrade is pending.
- A pending update contains `id`, `version`, `url`, `sha256`, and `size`.
- Pending updates are volatile and clear when the agent reports the target version or newer.

## Node Metrics Runtime Fields

- `POST /api/node/metrics` accepts optional `metrics.disk.smart` and `metrics.thermal`. Older agents may omit both fields.
- `metrics.disk.smart` is disk SMART runtime state. It is kept in a separate hot cache and is not written to PostgreSQL metrics snapshots. SMART temperature for confirmed physical disks may be reduced into per-device `disk.temp_c` history. `metrics.thermal` stores hardware temperature sensors at the metrics root; thermal data is written to PostgreSQL metrics snapshots but kept as a separate field cache in the frontend cache.
- `disk.smart.devices` and `thermal.sensors` are arrays. Empty results are `[]`, not `null`, when the field is present.
- Optional numeric fields such as `temp_c`, `power_on_hours`, `lifetime_used_percent`, `critical_warning`, `high_c`, and `critical_c` are omitted when unavailable. Missing values are not converted to `0`.
- `disk.smart.devices[].critical_warning` is the raw NVMe critical warning bitset. `disk.smart.devices[].failing_attrs[]` contains ATA SMART attributes currently reported as `FAILING_NOW`.
- SMART and thermal `status` values are open strings. Known values include `ok`, `partial`, `unsupported`, `not_found`, `no_permission`, `timeout`, `error`, `no_cache`, `stale`, `no_tool`, and `standby`.
- `disk.smart.status` is collection state. `disk.smart.devices[].health` is disk health. `status=ok` with `health=failed` means collection succeeded and the disk health check failed.
- `status=no_cache`, `no_tool`, or `unsupported` is not a disk failure. `status=stale` preserves the last `devices[]` while marking the cache expired.
- `GET /api/front/metrics` combines the latest hot node snapshot with the SMART and thermal field caches and returns `disk.smart`, `disk.temperature_devices`, and top-level `thermal` in each node view when present. `disk.temperature_devices` is the backend-derived list of physical disk names that can be used as `device` for `disk.temp_c` history.
- `/api/metrics/history` supports `cpu.temp_c` and `disk.temp_c`. CPU temperature comes from thermal CPU sensors. Disk temperature comes from SMART devices that are confirmed physical disks; virtual disks and RAID devices are not persisted. Passing `device` scopes `disk.temp_c` to one physical disk from `disk.temperature_devices`; omitting it aggregates the persisted physical disk rows.

## Alert Metrics

- Built-in SMART health failure and NVMe critical warning rules are mounted by default, like the built-in RAID failure rule.
- Optional user rules may use `disk.smart.failed`, `disk.smart.nvme.critical_warning`, `disk.smart.attribute_failing`, `disk.smart.max_temp_c`, and `thermal.max_temp_c`.
- `disk.smart.failed` counts devices whose SMART health is `failed`. It does not count `no_cache`, `no_tool`, `unsupported`, or other collection states as disk failures.
- `disk.smart.nvme.critical_warning` counts devices whose `critical_warning` bitset is non-zero. `disk.smart.attribute_failing` counts current `FAILING_NOW` SMART attributes.
- Missing `disk.smart` data is not a SMART failure. Built-in SMART rules do not trigger when a node has no SMART report.

## Traffic Statistics

- `GET /api/statistics/traffic/settings` returns `guest_access_mode`, `usage_mode`, `cycle_mode`, `billing_start_day`, `billing_anchor_date`, `billing_timezone`, and `direction_mode`.
- `PATCH /api/statistics/traffic/settings` accepts partial updates and rejects unknown values with `400 invalid_fields`.
- Allowed values: `guest_access_mode`: `disabled`, `by_node`; `usage_mode`: `lite`, `billing`; `cycle_mode`: `calendar_month`, `whmcs_compatible`, `clamp_to_month_end`; `direction_mode`: `out`, `both`, `max`.
- Traffic reads use the node's effective billing cycle configuration. Nodes with `traffic_cycle_mode=default` inherit the global cycle mode, billing start day, anchor date, and timezone; other nodes use their own `traffic_*` cycle fields.
- `GET /daily` requires `usage_mode=billing`; otherwise it returns `409 traffic_daily_requires_billing`. Optional `period` accepts `current` and `previous`; omitted means `current`.
- `GET /monthly` supports `months` and `period`. `months` is capped at 24; `period=current` starts from the current cycle, `period=previous` starts from the previous cycle, and omitted means `current`. The response field `includes_current` is `true` for `period=current` and `false` for `period=previous`.
- Direction mode selects the billing view: outbound, inbound plus outbound, or the larger inbound/outbound value per metric.
- Traffic summary, daily, and monthly responses keep raw `in_*` and `out_*` fields and expose the configured billing view through `selected_bytes`, `selected_p95_bytes_per_sec`, `selected_peak_bytes_per_sec`, and their direction fields.
- Clients should use `coverage_ratio` to display sample coverage and accuracy warnings. `partial` is kept for compatibility and should not be used for new display decisions.
- P95 fields are `null` unless `p95_status` is `available`.

## Non-API HTTP Paths

| Path                          | Role                                                    |
| ----------------------------- | ------------------------------------------------------- |
| `/theme/active.css`           | active theme CSS                                        |
| `/theme/active.json`          | active theme manifest; the default theme may return 404 |
| `/theme/preview/{id}.png`     | theme preview image                                     |
| `/deploy/linux/install.sh`    | Linux agent install script                              |
| `/deploy/macos/install.sh`    | macOS agent install script                              |
| `/deploy/windows/install.ps1` | Windows agent install script                            |
| `/deploy/*`                   | packaged node release assets                            |
| `/`                           | SPA                                                     |

## Compatibility Rules

- Existing paths, methods, and field meaning stay stable.
- New behavior uses new endpoints or added fields.
- Deprecations keep the old entry point before adding a replacement.
