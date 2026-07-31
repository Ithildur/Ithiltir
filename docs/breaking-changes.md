# Breaking Changes

## Unreleased

### Runtime configuration

- `monitor_dash_pwd` must contain at least 8 visible ASCII characters without whitespace. Existing installations with a shorter admin password must replace it before starting the upgraded Dash binary.
- `app.public_url` now accepts only IP literals or ASCII DNS names with ports from 1 through 65535. Internationalized domains must be configured in IDNA/punycode form.
- Unknown YAML fields and explicitly invalid duration values are rejected during startup instead of being ignored or replaced with defaults. Omitted duration fields still use their documented defaults.

### Notification channel configuration

- `dash migrate` encrypts complete notification-channel configuration documents with AES-256-GCM, replaces the plaintext in the current logical rows with `{}`, and creates `$DASH_HOME/configs/notify-config.key` only when no ciphertext already exists. Back up that key separately from PostgreSQL. Startup fails instead of generating a replacement or falling back to plaintext when the key is missing, invalid, or cannot decrypt every stored channel. This migration is not secure physical erasure: MVCC dead tuples, table free space, retained WAL, replicas, physical backups, and storage snapshots may still contain the former plaintext and must remain protected until explicitly retired under the applicable storage and backup retention policies.
- Stored notification channel configuration is validated with the current strict schema when it is listed, updated, tested, or used to send a notification. List and detail reads keep an invalid channel visible with `config: null` instead of failing the complete response.
- Numeric strings and integer-valued JSON floats are no longer accepted for integer fields such as `api_id` and `smtp_port`.
- Unknown configuration fields are rejected.
- Stored configurations that do not satisfy the current schema must be deleted and recreated. The admin UI keeps them visible for that recovery path while preventing unsafe edit, enable, selection, and test actions.
- Notification delivery no longer has a silent `failed_permanent` terminal state. Existing rows in that state are migrated to `blocked` and resume automatic low-frequency probes after upgrade; a historical notification may therefore be delivered when its endpoint recovers.
- Disabling a channel pauses unsent notifications, and re-enabling or compatibly replacing it wakes them. Deleting a channel or changing its type explicitly discards incompatible unsent notifications.
- Channel list and detail responses add delivery-health fields and the `delivery_status` values `unknown`, `healthy`, `degraded`, and `disabled`.
- Channel list and detail responses also expose `next_retry_at`; waking compatible work resets its attempt counter before retrying.
- Channel names are limited to 64 Unicode characters and cannot contain control characters.
- Webhook URLs are limited to HTTP(S), cannot contain user info or fragments, and are capped at 4096 bytes. Notification HTTP clients follow at most five redirects to the original hostname; same-scheme hops must keep the effective port, only HTTP-to-HTTPS upgrades may change scheme, and unsafe redirects are rejected before credentials are forwarded. POST notifications follow only method- and body-preserving `307`/`308` responses; `301`/`302`/`303` responses that downgrade the request to GET, and all other redirect-policy rejections, are recorded as `blocked` rather than transient send failures. Email addresses are parsed strictly, recipient lists are capped at 100 entries, and all channel types enforce bounded field sizes.
- Password, token, and secret placeholders retain the stored value only when the submitted field is omitted or exactly empty. Non-empty secret material is no longer trimmed, so leading/trailing whitespace is preserved as credential data.
- MTProto login-state failures now use `503 login_state_error` instead of the Redis-specific `redis_error`. Completing a login after the target channel was replaced returns `409 channel_changed` rather than overwriting the newer configuration.

### Node tags

- A node may have at most 32 tags.
- Each tag may contain at most 64 Unicode characters and must not contain control characters.
- Newly written tags must satisfy these limits. When reading stored tags, Dash logs a warning and discards tags that violate the limits or exceed the count cap; malformed stored JSON produces an empty tag set. The node and its other metrics remain in the frontend snapshot.

### Node and group metadata

- Node names are limited to 64 Unicode characters and cannot contain control characters.
- Newly submitted node secrets are trimmed and must contain 8 to 128 Unicode characters. Existing shorter stored secrets remain valid until they are rotated.
- Node report identifiers are now rejected before persistence when they exceed their documented PostgreSQL bounds; oversized metric/static payloads return `422` instead of surfacing as a database failure. Hostnames and disk identities have wider bounds, while operating-system paths, mountpoints, and hardware descriptions use `TEXT`.
- The upgrade migration temporarily decompresses retained disk metric chunks and rebuilds their disposable continuous aggregates before changing these column types. Operators should leave temporary database headroom; existing compression policies are restored and recompress eligible chunks after the migration.
- Group names are required and limited to 64 Unicode characters. Group remarks are limited to 255 Unicode characters. Neither field accepts control characters.
- Empty strings are no longer aliases for node traffic modes or global `usage_mode=lite`. Clients must send an accepted explicit mode.

### Alert rules

- Alert rule names may contain at most 128 Unicode characters and must not contain control characters.
- Thresholds and threshold offsets must be finite, duration is limited to 0, 60, or 300 seconds, and cooldown is limited to 525600 minutes.
- Stored rules that violate the current constraints are marked invalid. Startup reconciliation closes their open alert events with the `rule_invalid` reason.

### Runtime configuration

- `auth.jwt_signing_key` must be at least 32 bytes and must not contain surrounding whitespace. Changing the key invalidates existing login sessions.
- Default Redis mode requires a non-empty `redis.addr`, a server version of at least `8.2.3`, and permission for the configured account to run `PING` and `INFO server`. Dash refuses to start otherwise; `--no-redis` skips these Redis requirements.
- Redis continues to store admin sessions by default and also stores the disposable frontend cache; only `--no-redis` keeps sessions and the frontend cache in process memory. Alert evaluation runtime and MTProto login handshakes move to process-local state and reset on restart; open firing alerts are restored from PostgreSQL, while pending and cooldown phases are not restored.
- Frontend cache keys move to the project-namespaced `ithiltir:dash:front:v2:*` layout. Existing v1, unnamespaced v2, alert-runtime, and MTProto-login keys are ignored without dual-write and are not deleted automatically at startup. `auth:jwt:*` remains the session compatibility prefix, so upgrades do not proactively clear existing login credentials.

### Theme manifests

- Theme manifests reject unknown fields.
- `skin.admin.shell`, `skin.admin.frame`, `skin.dashboard.summary`, and `skin.dashboard.density` are required and no longer receive default values when omitted.
- Existing custom themes that do not satisfy the current manifest schema are reported as broken and the runtime uses the default skin until the package is replaced.
- Theme CSS must be valid UTF-8 and now has explicit file, declaration, name, and value limits. Resource-capable functions and `!important` are rejected after CSS escape decoding; quoted text is parsed structurally and is no longer rejected merely because it contains a forbidden word.

### Site branding

- New `logo_url` values accept only same-origin absolute paths, base64 SVG/PNG/JPEG/GIF/WebP/ICO data URLs, or external HTTPS URLs. External HTTP URLs, URL credentials, malformed data URLs, and other image media types are rejected.
- Stored HTTP logo values remain readable for compatibility, but an HTTPS browser may block them as mixed content; the UI falls back to the built-in logo.

### Traffic statistics API

- Billing cycles are now owned by individual nodes. `PATCH /api/statistics/traffic/settings` rejects `cycle_mode`, `billing_start_day`, `billing_anchor_date`, and `billing_timezone` with `400 billing_cycle_is_per_node`; the global cycle controls have been removed from the admin UI.
- The upgrade migration resolves every stored `traffic_cycle_mode=default` node to the global cycle that was effective immediately before migration, without queuing a replay or changing its existing cycle boundaries. New nodes default to an explicit calendar month starting on day 1. The legacy node PATCH input `traffic_cycle_mode=default` remains accepted but is normalized to that explicit calendar cycle; subsequent reads no longer return `default`.
- Traffic statistic responses no longer include the deprecated `stats.partial` field. Clients must use `data_complete`, `coverage_ratio`, `gap_count`, and `reset_count`.
- `GET /api/statistics/traffic/monthly` rejects `months` values outside 1 through 24 with `400 invalid_request`. Values above 24 are no longer capped automatically.
- Per-node traffic rebuild is available only in Billing mode. It returns `409 traffic_rebuild_requires_billing` in Lite mode. Switching to Lite while a rebuild is running stops it at the next chunk check.
- The upgrade migration initializes `covered_from` on existing `traffic_month_usage` rows to the corresponding `cycle_start` to preserve the legacy full-cycle presentation. This is a compatibility assumption, not coverage proven from historical raw samples.
- New traffic materialization progress starts at the most recent 30 minutes at upgrade time. Older pre-upgrade backlog is not replayed automatically even when the raw metrics remain retained; Billing can rebuild 5-minute facts per node, while existing Lite rows retain the compatibility assumption described above.

### Request errors

- Oversized JSON request bodies return `413 body_too_large` instead of being grouped with malformed JSON as `400 invalid_request`.

### Dash update archives

- The Linux updater is now the native `dash update` subcommand. `update_dash_linux.sh` remains only as a flag-compatible wrapper. New Linux archives must contain a version-matched `bin/dash` and release-format-v1 `release.env` with SHA-256 fields for every bundled node/runner asset; the candidate binary must report the manifest Dash and bundled-node versions. The built-in updater rejects historical archives without this manifest contract.
- Archives are limited to one regular-file/directory root, 1 GiB compressed, 4 GiB expanded, and 20000 entries. Migration failure no longer restores an older binary over a database that may already have forward migrations; Dash remains stopped with recovery files preserved.
- `goose_db_version` is the sole database schema version. Server startup now requires an exact match with the binary's embedded migrations, while `dash migrate` upgrades older schemas and rejects newer schemas. Starting an older binary after a schema upgrade is unsupported.

### Manual Dash installation

- `install_dash_linux.sh` is supported only as a one-time first installation on a fresh host. It is not a reinstall or update command; all later version changes use the packaged `dash update` executor. Existing `update_dash_linux.sh` commands remain compatible through the wrapper.
- Before configuration is collected, manual dependency mode requires PostgreSQL 16+ and a TimescaleDB installation built for that PostgreSQL major version. It does not require a local `redis-server` binary.
- After Redis configuration is collected, the installer validates the configured endpoint with the packaged Dash binary, including connectivity, `PING`, `INFO server`, and the Redis 8.2.3+ version floor. Remote-only Redis deployments are supported.

### Node installer redirects

- Linux, macOS, and Windows node installers no longer follow arbitrary redirects while forwarding `X-Node-Secret`. They accept at most five same-host hops, require the effective port to remain unchanged for same-scheme redirects, allow HTTP-to-HTTPS upgrades, and reject HTTPS downgrades and cross-host redirects.
