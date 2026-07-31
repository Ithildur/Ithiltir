# API

This document defines the current HTTP contract.

## Basics

- API base path: `/api`
- Dash is served from root paths only. Path prefixes in `app.public_url` are not supported.
- `app.public_url` accepts HTTP and HTTPS and requires an IP literal or ASCII DNS hostname with an optional port from 1 through 65535. Internationalized domains must use IDNA/punycode form. Bare IP addresses default to HTTP; bare domain names default to HTTPS.
- JSON error format:

```json
{ "code": "<string>", "message": "<string>" }
```

## Auth Model

| Method                                 | Usage                                                                       |
| -------------------------------------- | --------------------------------------------------------------------------- |
| admin password                         | `POST /api/auth/login`                                                      |
| refresh cookie + `X-CSRF-Token`        | `POST /api/auth/refresh`, `POST /api/auth/logout`                           |
| `Authorization: Bearer <access_token>` | admin APIs and optional authenticated reads                                 |
| `X-Node-Secret`                        | agent pushes, node identity reads, and deploy asset downloads               |
| `upgrade_token` query                  | temporary deploy asset download grant issued only for legacy agent upgrades |

Optional bearer endpoints treat a missing, malformed, expired, revoked, or otherwise invalid bearer token as an anonymous request. This is intentional: these are public endpoints with an optional authenticated view, not authenticated endpoints with a guest fallback. Refresh cookies use `SameSite=Strict`, and refresh/logout require the matching `X-CSRF-Token`.
The admin password is supplied through `monitor_dash_pwd` and must contain at least 8 visible ASCII characters without whitespace.

## Namespaces

| Prefix                                        | Auth                                                                                                      | Resources                                                                                                                                                                               |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/api/auth`                                   | admin password for login; refresh cookie + `X-CSRF-Token` for refresh and logout; bearer for `/sessions*` | `POST /login`, `POST /refresh`, `POST /logout`, session list and revoke                                                                                                                 |
| `/api/version`                                | none                                                                                                      | `GET /`                                                                                                                                                                                 |
| `/api/front`                                  | optional bearer                                                                                           | `GET /brand`, `GET /metrics`, `GET /groups`                                                                                                                                             |
| `/api/metrics`                                | optional bearer; metrics history is authenticated by default                                              | `GET /online`, `GET /history`                                                                                                                                                           |
| `/api/statistics`                             | optional bearer                                                                                           | `GET /access`                                                                                                                                                                           |
| `/api/statistics/traffic`                     | optional bearer; `PATCH /settings` requires bearer                                                        | `GET /settings`, `PATCH /settings`, `GET /ifaces`, `GET /summary`, `GET /daily`, `GET /monthly`                                                                                         |
| `/api/node`                                   | `X-Node-Secret`                                                                                           | `POST /identity`, `POST /metrics`, `POST /static`                                                                                                                                       |
| `/api/admin/groups`                           | bearer                                                                                                    | `GET /`, `GET /map`, `POST /`, `PATCH /{id}`, `DELETE /{id}`                                                                                                                            |
| `/api/admin/nodes`                            | bearer                                                                                                    | `GET /`, `GET /deploy`, `POST /`, `PUT /display-order`, `PATCH /traffic-p95`, `PATCH /{id}`, `POST /{id}/upgrade`, `GET /traffic/rebuild`, `POST /{id}/traffic/rebuild`, `DELETE /{id}` |
| `/api/admin/alerts/events`                    | bearer                                                                                                    | `GET /`, `GET /summary`, `GET /servers`                                                                                                                                                 |
| `/api/admin/alerts/rules`                     | bearer                                                                                                    | `GET /`, `POST /`, `PATCH /{id}`, `DELETE /{id}`                                                                                                                                        |
| `/api/admin/alerts/mounts`                    | bearer                                                                                                    | `GET /`, `PUT /`                                                                                                                                                                        |
| `/api/admin/alerts/settings`                  | bearer                                                                                                    | `GET /`, `PUT /`                                                                                                                                                                        |
| `/api/admin/alerts/channels`                  | bearer                                                                                                    | `GET /`, `POST /`, `GET /{id}`, `PUT /{id}`, `PUT /{id}/enabled`, `POST /{id}/test`, `DELETE /{id}`                                                                                     |
| `/api/admin/alerts/channels/telegram/mtproto` | bearer                                                                                                    | `POST /code`, `POST /verify`, `POST /password`, `POST /ping`                                                                                                                            |
| `/api/admin/system/dash-update`               | bearer                                                                                                    | `GET /status`, `GET /check`, `POST /run`, `GET /release-notes`                                                                                                                          |
| `/api/admin/system/settings`                  | bearer                                                                                                    | `GET /`, `PUT /`, `PATCH /`                                                                                                                                                             |
| `/api/admin/system/themes`                    | bearer                                                                                                    | `GET /`, `POST /upload`, `POST /{id}/apply`, `DELETE /{id}`                                                                                                                             |

## Anonymous Reads

- `/api/front/brand` is public.
- `/api/front/metrics` and `/api/front/groups` allow anonymous reads, but anonymous results include only guest-visible nodes. Anonymous `/api/front/groups` omits groups that have no guest-visible nodes.
- `/api/metrics/online` allows anonymous reads for guest-visible nodes.
- `/api/metrics/history` requires bearer by default. If `history_guest_access_mode` is `by_node`, anonymous reads are limited to guest-visible nodes.
- `/api/statistics/access` is public.
- Anonymous reads under `/api/statistics/traffic/*` are controlled by traffic settings and still respect node guest visibility.
- `GET /api/front/metrics` node metadata includes `node.tags` as a string array when tags are configured for the node.

## Auth Sessions

- In the default Redis mode, authentication sessions are stored in Redis and remain valid across Dash restarts and in-place upgrades until they expire or are revoked. With `--no-redis`, sessions are process-local and are invalidated whenever Dash restarts.
- `GET /api/auth/sessions/` returns `{ "sessions": [...] }` for the bearer token user. Each item includes `id`, `expires_at`, `session_only`, and `current`.
- `DELETE /api/auth/sessions/current`, `DELETE /api/auth/sessions/`, and `DELETE /api/auth/sessions/{sid}` return `204` on success.

## Admin Nodes

- `GET /api/admin/nodes/` includes `traffic_p95_enabled`, `traffic_cycle_mode`, `traffic_billing_start_day`, `traffic_billing_anchor_date`, `traffic_billing_timezone`, `traffic_direction_mode`, `tags`, and `version`. `tags` is always a string array.
- `version.version` is the last reported agent version. `version.is_outdated` is true when it is missing, invalid, or below the supported node version floor. `version.supports_auto_update` is true when the reported agent version supports the automatic update protocol; platform support and bundled update asset availability are still checked when an upgrade is requested.
- `PATCH /api/admin/nodes/{id}` accepts `traffic_p95_enabled`, `tags`, and node traffic fields. Omitted non-cycle fields are unchanged. Node cycle fields are atomic: if any of `traffic_cycle_mode`, `traffic_billing_start_day`, `traffic_billing_anchor_date`, or `traffic_billing_timezone` is submitted, the request must include `traffic_cycle_mode` and every field used by that mode, otherwise it returns `400 invalid_traffic_cycle_settings`. Cycle and direction changes take effect immediately. A cycle change invalidates affected monthly derived rows for that node and queues a node-local replay of retained raw data from the earlier start of the old and new active cycles. While that replay catches up, only the affected node is excluded from live Lite accumulation; the global progress and unrelated nodes continue, and the affected node may temporarily have no current-cycle statistic or only partial coverage. `calendar_month` uses `traffic_billing_timezone`; `clamp_to_month_end` uses `traffic_billing_start_day` and `traffic_billing_timezone`; `whmcs_compatible` uses `traffic_billing_anchor_date` and `traffic_billing_timezone`, with `traffic_billing_start_day` derived from the anchor date. The legacy input alias `default` remains accepted without billing fields and is stored as an explicit `calendar_month` cycle starting on day 1. `tags` accepts a string array; values are trimmed, empty values and duplicates are removed, and `[]` clears tags. `traffic_cycle_mode` responses contain `calendar_month`, `whmcs_compatible`, or `clamp_to_month_end`; `traffic_direction_mode` allows `default`, `out`, `both`, and `max`.
- `PATCH /api/admin/nodes/{id}` trims `secret` and returns `400 invalid_secret` unless it contains 8–128 Unicode characters. It returns `409 duplicate_secret` when the submitted `secret` already belongs to another node.
- A submitted node `name` is trimmed and must contain 1 to 64 Unicode characters with no control characters; invalid values return `400 invalid_name`.
- `GET /api/admin/nodes/deploy` returns each platform under `scripts` with `url` and `command_prefix`. Append the node secret to `command_prefix` to obtain the complete one-line install command.
- `PATCH /api/admin/nodes/traffic-p95` accepts `ids` and `enabled`. `enabled` is required. `ids` must be a non-empty positive integer array, cannot contain duplicates, and is capped at 10000 entries. The command validates every node ID first, then updates all selected nodes in one transaction. Success returns `204`; missing or deleted nodes return `404 not_found` and no node is updated.
- `GET /api/admin/nodes/traffic/rebuild` returns the process-local singleton rebuild state. Before a task exists it returns `status=idle`; running responses include `server_id`, `running=true`, and `started_at`; finished responses remain `completed` or `failed` until replaced. Restarting Dash stops any running task and resets this state to `idle`. Failed states expose stable `code` and `error` values, not internal error strings.
- `POST /api/admin/nodes/{id}/traffic/rebuild` starts a Billing-only rebuild of that node's 5-minute traffic facts from retained raw NIC metrics. It returns `202` with the same status body. Missing or deleted nodes return `404 not_found`; Lite mode returns `409 traffic_rebuild_requires_billing`; any running rebuild returns `409 traffic_rebuild_running`. The rebuild range is the intersection of raw metric retention and `database.traffic_retention_days`. The task rewrites facts in serial 6-hour chunks and invalidates overlapping monthly snapshots in the same transaction, releasing the traffic write gate between chunks. Switching to Lite lets an already-started chunk finish, then stops later chunks and fails the task with `traffic_rebuild_requires_billing`. Data outside retention is not recovered.
- Invalid `tags` returns `400 invalid_tags`.
- Every node stores an explicit billing cycle. New nodes default to `calendar_month` with `traffic_billing_start_day=1`. `calendar_month` stores day 1; non-`whmcs_compatible` modes store an empty `traffic_billing_anchor_date`; an empty `traffic_billing_timezone` uses the application timezone at read time.
- Node direction normalization is stable: `default` inherits the global traffic direction; `out`, `both`, and `max` override it for that node.
- Invalid node traffic fields return `400 invalid_traffic_cycle_mode`, `invalid_traffic_cycle_settings`, `invalid_traffic_billing_start_day`, `invalid_traffic_billing_anchor_date`, `invalid_traffic_billing_timezone`, or `invalid_traffic_direction_mode`.
- `POST /api/admin/nodes/{id}/upgrade` returns `204`. It returns `409 node_upgrade_unsupported` when the node cannot receive automatic update delivery, `409` when the bundled version, platform, or asset is unavailable, or `503 node_upgrade_grant_error` when Dash cannot prepare the temporary legacy download grant.

## Admin Groups

- `GET /api/admin/groups/` returns groups, and `GET /api/admin/groups/map` returns the group lookup used by node-management clients.
- `POST /api/admin/groups/` creates a group; `PATCH /api/admin/groups/{id}` updates submitted fields; `DELETE /api/admin/groups/{id}` deletes it.
- Group names are trimmed, required, limited to 64 Unicode characters, and cannot contain control characters. Remarks are trimmed, limited to 255 Unicode characters, and cannot contain control characters. Create returns `400 invalid_name` or `invalid_remark`; update returns `400 invalid_fields` for either invalid value and `400 no_fields` when neither field is supplied.

## Admin Alert Events

- `GET /api/admin/alerts/events` returns alert events for active nodes as `{ "items": [...], "next_cursor": "...", "has_more": true }`. Deleted nodes are not included. When `has_more=false`, `next_cursor` is `null`. Each item includes `id`, `rule_id`, `rule_generation`, `server_id`, `server_name`, `server_hostname`, `status`, `metric`, `rule_name`, `first_trigger_at`, and `last_trigger_at`. `server_ip`, `closed_at`, `current_value`, `effective_threshold`, `close_reason`, `title`, and `message` are present when available.
- Query parameters: `server_id` filters by node; `status` allows `open`, `closed`, and `all`, defaulting to `open`; `metric` filters by alert metric name; `from` and `to` are optional RFC3339 timestamps and filter on `last_trigger_at` when supplied; `limit` defaults to 200 and is capped at 500; `cursor` uses the previous response `next_cursor` to continue the `last_trigger_at DESC, id DESC` order.
- `GET /api/admin/alerts/events/summary` returns current open alert summaries as `{ "items": [...] }`. Each item includes `server_id`, `open_count`, `last_trigger_at`, `metric`, `rule_name`, and `metrics` for the node overview. `metric` and `rule_name` describe the latest open event; `metrics` lists open event metrics in latest-first order. The summary has no time filter.
- `GET /api/admin/alerts/events/servers` returns active server filter options as `{ "items": [{ "id": 1, "name": "..." }] }`.

## Admin Alert Mounts

- `GET /api/admin/alerts/mounts/` returns rules, nodes, and each node's mount state.
- `PUT /api/admin/alerts/mounts/` accepts non-empty `rule_ids`, non-empty positive `server_ids`, and required boolean `mounted`. Duplicate IDs are coalesced; unknown rules or nodes return `400 invalid_fields`. A successful database update returns `204`, after which evaluation is queued in process. Persistence or validation-query failures return `503 db_error`; there is no cache- or alert-runtime-specific API error.

## Admin Alert Channels

- `GET /api/admin/alerts/channels` and `GET /api/admin/alerts/channels/{id}` return sanitized channel configuration plus delivery health. When a stored configuration cannot be decoded by the current schema, the affected channel remains in the successful response with `config: null`; one invalid channel does not fail the list. The admin UI keeps that channel visible for removal but does not allow it to be edited, enabled, selected as a new notification target, or tested. `delivery_status` is `unknown` before the first successful delivery, `healthy` after a success with no current blocked work, `degraded` while delivery failures or blocked notifications remain, and `disabled` while the channel is disabled.
- Delivery health fields are `last_success_at`, `last_failure_at`, `consecutive_failures`, `last_error_code`, `last_error`, `next_retry_at`, `next_probe_at`, `pending_count`, and `blocked_count`. Optional timestamps and errors are `null` when unavailable. `next_retry_at` is the earliest scheduled transient retry and is displayed in the admin UI as local `YYYYMMDD HH:mm:ss`; `next_probe_at` is the earliest blocked recovery probe. `pending_count` includes pending, sending, retrying, blocked, and paused notifications; `blocked_count` is the subset waiting for low-frequency recovery probes.
- `updated_at` is the last channel configuration or enabled-state change. Background health updates do not change its API value.
- Channel names are trimmed, required, limited to 64 Unicode characters, and cannot contain control characters. Invalid create or replacement requests return `400 invalid_fields`.
- Passwords, tokens, hashes, sessions, and webhook secrets are opaque credential material. Omitted or exactly empty placeholders retain the stored value on compatible updates; JSON `null` is invalid for typed configuration fields. Accepted non-empty values are not trimmed or otherwise normalized, so leading and trailing whitespace are preserved.
- A webhook `url` must be an absolute HTTP or HTTPS URL with a non-empty hostname; user information and fragments are rejected.
- `PUT /api/admin/alerts/channels/{id}` replaces the channel configuration. Saving a compatible channel immediately wakes retrying, blocked, and paused notifications and resets their retry-attempt budget; changing the channel type discards notifications created for the previous type. If the channel changes after the request reads the current secret/session but before it commits, the request returns `409 channel_changed` instead of overwriting the newer revision.
- `PUT /api/admin/alerts/channels/{id}/enabled` accepts `{ "enabled": true|false }`. Disabling pauses unsent notifications. Re-enabling wakes them immediately and preserves any previous degraded state until a real delivery succeeds; re-enabling alone does not mark the channel healthy.
- `POST /api/admin/alerts/channels/{id}/test` gives remote delivery at most ten seconds. A successful test marks the tested configuration healthy and immediately wakes blocked notifications. A failed test records the same structured delivery error used by the background worker. If persisting either the successful recovery or the failed-test error fails, the API returns `503 db_error` instead of reporting a result whose health state was not stored. A concurrent configuration replacement wins: a result from the older tested revision does not change the new configuration's health.
- Telegram Bot HTTP `429` responses use the longer of the HTTP `Retry-After` header and the Bot API JSON `parameters.retry_after` value, subject to the worker's retry bound.
- Notification HTTP clients follow at most five redirects. Every hop must preserve the original hostname and request method; same-scheme redirects must keep the effective port, HTTP may upgrade to HTTPS, and HTTPS downgrade, cross-host, user-info, and same-scheme port changes are rejected. Notification requests are POSTs, so only body-preserving `307`/`308` responses are followed; `301`/`302`/`303` responses that rewrite POST to GET are rejected. Webhook signing and channel credentials are sent only after each hop passes this check. Redirect-policy rejection is a deterministic delivery failure: it moves channel work to `blocked` with low-frequency recovery probes instead of transient-network retry cadence.
- MTProto login state failures from `/telegram/mtproto/code`, `/verify`, and `/password` return `503 login_state_error`. Finishing login patches only the session of the channel revision that started the flow; if the channel was replaced concurrently, `/verify` or `/password` returns `409 channel_changed` and the login must be restarted.
- Deleting a channel removes it from alert settings and discards its unsent notifications.

## Admin System Settings

- `GET /api/admin/system/settings` returns `history_guest_access_mode`, `dash_update_channel`, `dash_update_mode`, `logo_url`, `page_title`, and `topbar_text`. `dash_update_channel` is `release` or `prerelease`; `dash_update_mode` is `manual`, `notify`, or `auto`.
- `PATCH /api/admin/system/settings` validates and updates only the submitted fields, so concurrent updates to different fields do not overwrite one another and an unchanged legacy HTTP logo does not block unrelated edits. An empty update returns `400 no_fields`; invalid submitted values return `400 invalid_fields`.
- `PUT /api/admin/system/settings` replaces the full settings document and requires `history_guest_access_mode`, `dash_update_channel`, `dash_update_mode`, `logo_url`, `page_title`, and `topbar_text`.
- `logo_url` may be the built-in path, a same-origin absolute path, a base64 SVG, PNG, JPEG, GIF, WebP, or ICO data URL, or an external HTTPS URL. External HTTP URLs are rejected.
- Stored external HTTP logos from an older release remain readable for compatibility. Browsers may still block them as mixed content on an HTTPS page; logo and favicon rendering then fall back to the built-in logo.

## Admin Themes

- `GET /api/admin/system/themes/` returns readable built-in and custom packages. Each item includes `id`, `name`, `version`, `author`, `description`, `skin`, `format_version`, `deprecated`, `built_in`, `active`, `deletable`, `missing`, `broken`, `has_preview`, `created_at`, and `updated_at`. Current packages report `format_version=1` and `deprecated=true`.
- Theme package format v1 is frozen and deprecated without a removal date. Existing v1 packages remain uploadable, applicable, and runnable. V1 accepts no new CSS syntax, file types, or skin capabilities; future capabilities require a new format.
- Theme manifests require `skin.admin.shell`, `skin.admin.frame`, `skin.dashboard.summary`, and `skin.dashboard.density`. Missing or unknown values reject the package.
- Theme CSS accepts custom-property declarations only. A package is rejected when CSS is not valid UTF-8, exceeds 1 MiB per file, contains more than 1024 declarations, uses a custom-property name longer than 128 bytes, uses a value longer than 4096 Unicode characters, or contains `!important` or resource-capable functions such as `url()`, `image-set()`, `src()`, or `expression()`. Validation parses CSS strings, escapes, and function tokens; those words remain legal inside quoted text, while escaped function names cannot bypass the restriction.
- `POST /api/admin/system/themes/upload` accepts one `file` part whose ZIP payload is at most 20 MiB. The complete multipart request is capped at 21 MiB, leaving 1 MiB for boundaries, headers, and other framing overhead. Dash gives this upload route a maximum five-minute request-read and response-write window.
- The configured active theme ID remains selected even when its package is missing or broken. Runtime falls back to the frontend's built-in default skin; the list exposes a synthetic `missing` or `broken` item and returns `X-Dash-Warning: theme_active_missing` or `X-Dash-Warning: theme_active_broken` so the package can be uploaded again or another skin can be selected.

## Admin Dash Update

- `GET /api/admin/system/dash-update/status` returns the latest Dash update task state. `status` is `idle`, `running`, `completed`, or `failed`. Responses include `available`, and may include `id`, `action`, `channel`, `target_version`, `phase`, `failure_code`, `recovery_path`, `started_at`, `finished_at`, `exit_code`, `log_tail`, and `unavailable_reason`. A failed transaction that must be resumed uses `failure_code=recovery_required`; `recovery_path` identifies preserved local recovery files. Successful forward recovery changes the job to `completed`; successful pre-migration rollback leaves it `failed` with `failure_code=rolled_back` and clears the recovery path.
- `GET /api/admin/system/dash-update/check?channel=release|prerelease` returns `current_version`, `current_channel`, `target_channel`, `latest_version`, `install_revision`, `version_status`, and `bundled_node_version`. `install_revision` is an opaque SHA-256 installation identity used to pin a subsequent run. `version_status` is `available`, `current`, `ahead`, or `unknown`. Omitted `channel` defaults to `release`; `prerelease` checks prerelease releases only. Local availability, installed-state inspection, and GitHub lookup run concurrently under one bounded response budget. Invalid `channel` returns `400 invalid_fields`; a missing or recovery-blocked local updater returns `503 dash_update_unavailable`; GitHub Releases lookup failure or timeout returns `502 dash_update_check_failed`.
- `POST /api/admin/system/dash-update/run` requires `action=update|reinstall`, `channel=release|prerelease`, and `lang=zh|en`. Current clients also echo the preceding check as `target_version`, `expected_current_version`, and `expected_install_revision`; these three fields must be supplied together. For compatibility, omitting all three makes the server perform a bounded, side-effect-free fresh check before queuing. Task submission has a separate bounded window. Once a job is durably reserved, its status resource is authoritative even if transient-unit submission is initially uncertain. The immutable plan is persisted before the executor starts, and the executor verifies current version and installation revision under its cross-process lock. A stale plan is accepted as a job but ends with `failure_code=install_changed`; it never chooses a different target. Ordinary `update` requires a newer target; an equal target returns `409 dash_update_current` before task persistence, and callers must use `reinstall` to apply the same release again. Success returns `202` with the status body. A running task returns `409`; invalid fields return `400 invalid_fields`; an unavailable updater returns `503 dash_update_unavailable`.
- `dash_update_mode=notify` makes Dash periodically check the configured update channel and enqueue an update-available message for every enabled notification channel. `dash_update_mode=auto` checks periodically and starts an update when a newer version is available. If the updater cannot be started, Dash enqueues a failure notification; after a started task reaches a terminal state, it enqueues the result. A successful start has no separate notification. Each channel is delivered and retried independently through the durable notification outbox.
- The admin controller is Linux/systemd based and launches `dash update execute` from the packaged Dash binary in a transient unit; `update_dash_linux.sh` is only a manual compatibility wrapper. Release archives must use format v1, contain matching `release.env`, `bin/dash`, `dist/index.html`, all seven bundled node/runner assets for the five supported platform/architecture targets under `deploy`, `configs/config.example.yaml`, and the Linux install/update scripts, and contain only regular files/directories under `Ithiltir-dash/`; required files must be non-empty. `release.env` binds each bundled asset by SHA-256, and the candidate Dash binary must report both the manifest Dash version and bundled-node version. Compressed size, expanded size, and entry count are bounded. The task may restart Dash, so callers must tolerate a brief connection loss after `202`. If migration or service start requires recovery, run `DASH_HOME/bin/dash update recover` as root.
- `GET /api/admin/system/dash-update/release-notes?lang=zh|en` returns `{ "source_url": "...", "html": "..." }` from the documentation site. Invalid `lang` returns `400 invalid_fields`; fetch failure returns `502 release_notes_fetch_failed`.

## Agent Updates

- Successful `POST /api/node/metrics` responses include `update`.
- `update` is `null` when no upgrade is pending.
- A pending update contains `id`, `version`, `url`, `sha256`, and `size`.
- `url` may include a short-lived `upgrade_token` so legacy agents can download the exact update asset without sending `X-Node-Secret`. Clients must use the URL as returned.
- Pending updates are volatile and clear when the agent reports the exact target version or a higher SemVer precedence. Different build metadata at the same SemVer precedence is treated as a distinct node binary and can still be delivered.

## Node Metrics Runtime Fields

- `POST /api/node/metrics` accepts optional `metrics.disk.smart`, `metrics.thermal`, and `metrics.pressure`. Older agents may omit these fields.
- Persisted byte, capacity, counter, and uptime values are non-negative JSON integers within signed 64-bit range. Process and connection counts use signed 32-bit range; `/api/node/static` additionally uses signed 32-bit report intervals and signed 16-bit CPU topology counts. Existing agents remain wire-compatible because ordinary positive JSON integers have the same encoding. An integer outside the receiving type returns `400 invalid_request`; a negative value, invalid ratio, or invalid rate returns `422 invalid_metrics` or `422 invalid_static_payload`.
- Text written to bounded PostgreSQL identifiers is checked before persistence: Node version 64 characters, hostname and disk name 255, disk reference 320, disk kind/role and RAID health 16, interface name 64, and filesystem type plus logical-disk health/level 32. Static OS/platform/architecture values are limited to 32 characters and platform/kernel versions to 255. Paths, mountpoints, and hardware descriptions use unbounded text storage. Oversized values return `422 invalid_metrics` or `422 invalid_static_payload`; values are never silently truncated.
- Concurrent reports are ordered by server receive time. An older receive-time sample that finishes after a newer one is still written to metrics history, but it does not overwrite current metrics or the frontend hot snapshot and does not schedule a new alert evaluation. The request `timestamp` is retained as the agent-reported time and does not choose the current projection.
- `metrics.disk.smart` is disk SMART runtime state. It is kept in a separate hot cache and is not written to PostgreSQL metrics snapshots. SMART temperature for confirmed physical disks may be reduced into per-device `disk.temp_c` history. `metrics.thermal` stores hardware temperature sensors at the metrics root; thermal data is written to PostgreSQL metrics snapshots but kept as a separate field cache in the frontend cache.
- `metrics.pressure` is Linux PSI (Pressure Stall Information). It may contain `cpu`, `memory`, and `io`, each with optional `some` and `full` numeric groups. Each group has `avg10`, `avg60`, `avg300` percentages and cumulative `total` microseconds. Dashboard stores these values as fixed numeric time-series columns; collection status/reason strings are not persisted. Missing groups remain `NULL` and are treated as unavailable, not as zero pressure.
- `disk.smart.devices` and `thermal.sensors` are arrays. Empty results are `[]`, not `null`, when the field is present.
- Optional numeric fields such as `temp_c`, `power_on_hours`, `lifetime_used_percent`, `critical_warning`, `media_errors`, `high_c`, and `critical_c` are omitted when unavailable. Missing values are not converted to `0`.
- `disk.smart.devices[].critical_warning` is the raw NVMe critical warning bitset. `disk.smart.devices[].media_errors` is the NVMe SMART `media_errors` counter when available; alert text labels it with the SMART UI item number `0E`. `disk.smart.devices[].failing_attrs[]` contains ATA SMART attributes currently reported as `FAILING_NOW`.
- SMART and thermal `status` values are open strings. Known values include `ok`, `partial`, `unsupported`, `not_found`, `no_permission`, `timeout`, `error`, `no_cache`, `stale`, `no_tool`, and `standby`.
- `disk.smart.status` is collection state. `disk.smart.devices[].health` is disk health. `status=ok` with `health=failed` means collection succeeded and the disk health check failed.
- `status=no_cache`, `no_tool`, or `unsupported` is not a disk failure. `status=stale` preserves the last `devices[]` while marking the cache expired.
- `GET /api/front/metrics` combines the latest hot node snapshot with the SMART and thermal field caches and returns `disk.smart`, `disk.temperature_devices`, top-level `thermal`, and top-level `pressure` in each node view when present. `disk.temperature_devices` is the backend-derived list of physical disk names that can be used as `device` for `disk.temp_c` history.
- `/api/metrics/history` supports `cpu.temp_c`, `disk.temp_c`, and PSI average metrics: `pressure.cpu.some_avg10|avg60|avg300`, `pressure.memory.some_avg10|avg60|avg300`, `pressure.memory.full_avg10|avg60|avg300`, `pressure.io.some_avg10|avg60|avg300`, and `pressure.io.full_avg10|avg60|avg300`. CPU temperature comes from thermal CPU sensors. Disk temperature comes from SMART devices that are confirmed physical disks; virtual disks and RAID devices are not persisted. Passing `device` scopes `disk.temp_c` to one physical disk from `disk.temperature_devices`; omitting it aggregates the persisted physical disk rows.

## Alert Metrics

- Built-in SMART health failure and NVMe critical warning rules are mounted by default, like the built-in RAID failure rule.
- Optional user rules may use `disk.smart.failed`, `disk.smart.nvme.critical_warning`, `disk.smart.attribute_failing`, `disk.smart.max_temp_c`, and `thermal.max_temp_c`.
- `disk.smart.failed` counts devices whose SMART health is `failed`. It does not count `no_cache`, `no_tool`, `unsupported`, or other collection states as disk failures.
- `disk.smart.nvme.critical_warning` counts devices whose `critical_warning` bitset is non-zero. `disk.smart.attribute_failing` counts current `FAILING_NOW` SMART attributes.
- Missing `disk.smart` data is not a SMART failure. Built-in SMART rules do not trigger when a node has no SMART report.
- PSI pressure data is currently stored and exposed for history queries only; it is not an alert metric yet and no built-in PSI alerts are enabled.

## Traffic Statistics

- `GET /api/statistics/traffic/settings` returns `guest_access_mode`, `usage_mode`, `cycle_mode`, `billing_start_day`, `billing_anchor_date`, `billing_timezone`, and `direction_mode`. The cycle fields are compatibility fields: they always describe a calendar month starting on day 1 in the application timezone and are not a mutable global default.
- `PATCH /api/statistics/traffic/settings` accepts partial updates to `guest_access_mode`, `usage_mode`, and `direction_mode`. Submitting any cycle field returns `400 billing_cycle_is_per_node`; billing cycles must be changed through the node API. Switching from Lite to Billing starts the background Facts materializer from the most recent 30 minutes; the `204` response does not wait for materialization. Older per-node facts from the Lite interval can be restored on demand while raw data remains retained. Switching from Billing to Lite does not wait for a running rebuild; that task stops at its next chunk check. Success returns `204`.
- Allowed mutable values: `guest_access_mode`: `disabled`, `by_node`; `usage_mode`: `lite`, `billing`; `direction_mode`: `out`, `both`, `max`.
- Traffic reads use each node's explicit cycle fields. Only `traffic_direction_mode=default` is inherited from the global direction mode; `out`, `both`, and `max` remain node-specific overrides.
- Both `lite` and `billing` modes use the effective billing cycle for monthly boundaries. `billing` additionally enables daily statistics, P95, coverage, 5-minute facts, and monthly billing snapshots.
- `GET /daily` requires `usage_mode=billing`; otherwise it returns `409 traffic_daily_requires_billing`. Optional `period` accepts `current` and `previous`; omitted means `current`.
- `GET /monthly` supports `months` and `period`. `months` must be between 1 and 24; invalid values return `400 invalid_request`. `period=current` starts from the current cycle, `period=previous` starts from the previous cycle, and omitted means `current`. The response field `includes_current` is `true` for `period=current` and `false` for `period=previous`.
- Direction mode selects the billing view: outbound, inbound plus outbound, or the larger inbound/outbound value per metric.
- Traffic summary, daily, and monthly responses keep raw `in_*` and `out_*` fields and expose the configured billing view through `selected_bytes`, `selected_p95_bytes_per_sec`, `selected_peak_bytes_per_sec`, and their direction fields.
- Clients use `coverage_ratio`, `data_complete`, `gap_count`, and `reset_count` to display sample coverage and accuracy warnings.
- P95 fields are `null` unless `p95_status` is `available`.

## Non-API HTTP Paths

| Path                          | Role                                                                                                       |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `/theme/active.css`           | resolved theme CSS; missing or broken configured themes return empty override CSS for the frontend default |
| `/theme/active.json`          | resolved theme manifest; the default and fallback default return 404                                       |
| `/theme/preview/{id}.png`     | theme preview image                                                                                        |
| `/deploy/linux/install.sh`    | Linux agent install script                                                                                 |
| `/deploy/macos/install.sh`    | macOS agent install script                                                                                 |
| `/deploy/windows/install.ps1` | Windows agent install script                                                                               |
| `/deploy/*`                   | packaged node release assets; requires `X-Node-Secret` or a temporary `upgrade_token`                      |
| `/`                           | SPA                                                                                                        |

## Contract Rules

- Unknown or malformed values are rejected at the boundary rather than silently normalized into another valid request.
- JSON requests that exceed the route body limit return `413 body_too_large`; malformed JSON returns `400 invalid_request`.
- Core durable-storage and required-dependency failures are returned as errors. Documented optional boundaries retain their degraded behavior: optional bearer reads become anonymous, and an unavailable active theme uses the frontend default while exposing `missing` or `broken` state.
