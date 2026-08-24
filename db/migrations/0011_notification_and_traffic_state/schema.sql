ALTER TABLE notify_channels
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN last_success_at TIMESTAMPTZ,
    ADD COLUMN last_failure_at TIMESTAMPTZ,
    ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN last_error_code VARCHAR(64),
    ADD COLUMN last_error TEXT,
    ADD COLUMN config_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN config_sealed BYTEA;

ALTER TABLE notify_channels
    DISABLE TRIGGER notify_channels_updated_at;

UPDATE notify_channels
SET config_updated_at = updated_at;

ALTER TABLE notify_channels
    ENABLE TRIGGER notify_channels_updated_at;

ALTER TABLE notify_channels
    DROP CONSTRAINT IF EXISTS chk_notify_channels_config_storage;

-- Existing plaintext rows are converted by the following Go migration. NOT
-- VALID permits those rows temporarily while immediately rejecting new writes
-- that do not provide ciphertext and clear the compatibility column.
ALTER TABLE notify_channels
    ADD CONSTRAINT chk_notify_channels_config_storage
    CHECK (config = '{}'::jsonb AND config_sealed IS NOT NULL) NOT VALID;

WITH normalized AS (
    SELECT settings.id,
           COALESCE(
               jsonb_agg(ref.value ORDER BY ref.ordinality)
                   FILTER (WHERE channel.id IS NOT NULL),
               '[]'::jsonb
           ) AS channel_ids
    FROM alert_settings AS settings
    LEFT JOIN LATERAL jsonb_array_elements(settings.channel_ids)
        WITH ORDINALITY AS ref(value, ordinality) ON TRUE
    LEFT JOIN notify_channels AS channel
        ON to_jsonb(channel.id) = ref.value
       AND channel.is_deleted = FALSE
    GROUP BY settings.id
)
UPDATE alert_settings AS settings
SET channel_ids = normalized.channel_ids
FROM normalized
WHERE settings.id = normalized.id
  AND settings.channel_ids IS DISTINCT FROM normalized.channel_ids;

ALTER TABLE alert_notification_outbox
    ADD COLUMN probe_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN is_probe BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN failure_code VARCHAR(64),
    ALTER COLUMN event_id DROP NOT NULL,
    ALTER COLUMN transition TYPE VARCHAR(32);

UPDATE alert_notification_outbox
SET status = 'blocked',
    failure_code = COALESCE(failure_code, 'legacy_failed_permanent'),
    next_attempt_at = now() + ((id % 3600) * INTERVAL '1 second')
WHERE status = 'failed_permanent';

UPDATE alert_notification_outbox AS outbox
SET status = 'paused',
    failure_code = 'channel_disabled',
    probe_count = 0
FROM notify_channels AS channel
WHERE outbox.channel_id = channel.id
  AND channel.enabled = FALSE
  AND outbox.status IN ('pending', 'retry', 'blocked');

ALTER TABLE alert_notification_outbox
    DROP CONSTRAINT IF EXISTS chk_alert_notification_outbox_status;

ALTER TABLE alert_notification_outbox
    ADD CONSTRAINT chk_alert_notification_outbox_status
    CHECK (status IN ('pending', 'sending', 'sent', 'retry', 'blocked', 'paused', 'discarded'));

ALTER TABLE alert_notification_outbox
    DROP CONSTRAINT IF EXISTS chk_alert_notification_outbox_probe_state;

ALTER TABLE alert_notification_outbox
    ADD CONSTRAINT chk_alert_notification_outbox_probe_state
    CHECK (NOT is_probe OR status = 'sending');

CREATE INDEX idx_alert_notification_outbox_channel_active
    ON alert_notification_outbox (channel_id, status, next_attempt_at)
    WHERE status IN ('pending', 'sending', 'retry', 'blocked', 'paused');

DROP INDEX IF EXISTS uniq_alert_control_tasks_dedupe;

CREATE UNIQUE INDEX uniq_alert_control_tasks_dedupe
    ON alert_control_tasks (dedupe_key)
    WHERE status IN ('pending', 'leased');

INSERT INTO metric_settings (
    id,
    history_guest_access_mode,
    created_at,
    updated_at
)
VALUES (1, 'disabled', now(), now())
ON CONFLICT (id) DO NOTHING;

COMMENT ON COLUMN notify_channels.revision IS '渠道配置和启停状态版本';
COMMENT ON COLUMN notify_channels.last_success_at IS '最近一次成功投递时间';
COMMENT ON COLUMN notify_channels.last_failure_at IS '最近一次投递失败时间';
COMMENT ON COLUMN notify_channels.consecutive_failures IS '连续投递失败次数';
COMMENT ON COLUMN notify_channels.last_error_code IS '最近一次投递失败稳定错误码';
COMMENT ON COLUMN notify_channels.last_error IS '最近一次投递失败摘要';
COMMENT ON COLUMN notify_channels.config_updated_at IS '最近一次配置或启停状态更新时间';
COMMENT ON COLUMN notify_channels.config IS '兼容占位；通知渠道配置明文不得持久化';
COMMENT ON COLUMN notify_channels.config_sealed IS 'AES-256-GCM 加密的完整渠道配置';
COMMENT ON TABLE alert_notification_outbox IS '持久化通知 outbox；历史表名保留用于兼容';
COMMENT ON COLUMN alert_notification_outbox.event_id IS '关联告警事件；系统通知为空';
COMMENT ON COLUMN alert_notification_outbox.transition IS '通知事件类型';
COMMENT ON COLUMN alert_notification_outbox.probe_count IS '低频恢复探测失败次数';
COMMENT ON COLUMN alert_notification_outbox.is_probe IS '当前 sending 投递是否为 blocked 渠道恢复探测';
COMMENT ON COLUMN alert_notification_outbox.failure_code IS '最近一次失败或终止原因的稳定错误码';
COMMENT ON INDEX uniq_alert_control_tasks_dedupe IS '活跃告警控制任务幂等键；失败任务保留诊断信息但不阻止重新入队';

UPDATE alert_control_tasks
SET status = CASE WHEN status = 'leased' THEN 'pending' ELSE status END,
    leased_until = NULL
WHERE status = 'leased'
   OR leased_until IS NOT NULL;

UPDATE alert_notification_outbox
SET leased_until = now()
WHERE status = 'sending';

UPDATE alert_notification_outbox
SET leased_until = NULL
WHERE status <> 'sending'
  AND leased_until IS NOT NULL;

COMMENT ON COLUMN alert_control_tasks.leased_until IS '历史 schema 兼容字段；当前单实例 worker 不使用租约';
COMMENT ON COLUMN alert_notification_outbox.leased_until IS '历史 schema 兼容字段；当前 worker 不按租约截止时间取件';

-- Billing cycles are node-owned. Preserve the effective cycle of legacy
-- `default` nodes before freezing the compatibility fields in the singleton
-- traffic settings row.
UPDATE servers AS server
SET traffic_cycle_mode = settings.cycle_mode,
    traffic_billing_start_day = CASE
        WHEN settings.cycle_mode = 'calendar_month' THEN 1
        ELSE settings.billing_start_day
    END,
    traffic_billing_anchor_date = CASE
        WHEN settings.cycle_mode = 'whmcs_compatible' THEN settings.billing_anchor_date
        ELSE ''
    END,
    traffic_billing_timezone = settings.billing_timezone
FROM traffic_settings AS settings
WHERE settings.id = 1
  AND COALESCE(NULLIF(server.traffic_cycle_mode, ''), 'default') = 'default';

ALTER TABLE servers
    ALTER COLUMN traffic_cycle_mode SET DEFAULT 'calendar_month',
    DROP CONSTRAINT IF EXISTS chk_servers_traffic_cycle_mode;

ALTER TABLE servers
    ADD CONSTRAINT chk_servers_traffic_cycle_mode
    CHECK (traffic_cycle_mode IN ('calendar_month', 'whmcs_compatible', 'clamp_to_month_end'));

UPDATE traffic_settings
SET cycle_mode = 'calendar_month',
    billing_start_day = 1,
    billing_anchor_date = '',
    billing_timezone = ''
WHERE id = 1;

ALTER TABLE traffic_settings
    DROP CONSTRAINT IF EXISTS chk_traffic_settings_fixed_cycle;

ALTER TABLE traffic_settings
    ADD CONSTRAINT chk_traffic_settings_fixed_cycle
    CHECK (
        cycle_mode = 'calendar_month'
        AND billing_start_day = 1
        AND billing_anchor_date = ''
        AND billing_timezone = ''
    );

COMMENT ON COLUMN servers.traffic_cycle_mode IS '节点显式账期模式';
COMMENT ON COLUMN traffic_settings.cycle_mode IS '兼容只读字段；固定为 calendar_month';
COMMENT ON COLUMN traffic_settings.billing_start_day IS '兼容只读字段；固定为 1';
COMMENT ON COLUMN traffic_settings.billing_anchor_date IS '兼容只读字段；固定为空';
COMMENT ON COLUMN traffic_settings.billing_timezone IS '兼容只读字段；固定为空并使用应用时区';

-- Server labels and hostnames support the conventional DNS length. System
-- versions and storage identities are bounded external identifiers; hardware
-- descriptions and operating-system paths are not.
ALTER TABLE servers
    ALTER COLUMN name TYPE VARCHAR(255),
    ALTER COLUMN hostname TYPE VARCHAR(255),
    ALTER COLUMN platform_version TYPE VARCHAR(255),
    ALTER COLUMN kernel_version TYPE VARCHAR(255),
    ALTER COLUMN cpu_model TYPE TEXT,
    ALTER COLUMN cpu_vendor TYPE TEXT,
    ALTER COLUMN root_path TYPE TEXT;

-- TimescaleDB cannot alter a hypertable with compressed chunks, and continuous
-- aggregates cannot follow a grouping-column type change. Derived aggregates
-- are disposable; decompress and rebuild them only while upgrading legacy
-- storage types.
DO $migration$
DECLARE
    compressed_chunk REGCLASS;
    change_disk_io BOOLEAN;
    change_disk_physical BOOLEAN;
    change_disk_usage BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'disk_metrics'
          AND (
              (column_name = 'name' AND character_maximum_length IS DISTINCT FROM 255)
              OR (column_name = 'ref' AND character_maximum_length IS DISTINCT FROM 320)
              OR (column_name = 'path' AND data_type <> 'text')
          )
    ) INTO change_disk_io;

    IF change_disk_io THEN
        DROP MATERIALIZED VIEW IF EXISTS disk_metrics_15m;
        DROP MATERIALIZED VIEW IF EXISTS disk_metrics_1h;
        PERFORM remove_compression_policy('disk_metrics', if_exists => TRUE);
        FOR compressed_chunk IN
            SELECT format('%I.%I', chunk_schema, chunk_name)::regclass
            FROM timescaledb_information.chunks
            WHERE hypertable_schema = current_schema()
              AND hypertable_name = 'disk_metrics'
              AND is_compressed
        LOOP
            PERFORM decompress_chunk(compressed_chunk, true);
        END LOOP;
        ALTER TABLE disk_metrics
            ALTER COLUMN name TYPE VARCHAR(255),
            ALTER COLUMN ref TYPE VARCHAR(320),
            ALTER COLUMN path TYPE TEXT;
    END IF;

    SELECT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'disk_physical_metrics'
          AND (
              (column_name = 'name' AND character_maximum_length IS DISTINCT FROM 255)
              OR (column_name = 'ref' AND character_maximum_length IS DISTINCT FROM 320)
              OR (column_name = 'path' AND data_type <> 'text')
          )
    ) INTO change_disk_physical;

    IF change_disk_physical THEN
        PERFORM remove_compression_policy('disk_physical_metrics', if_exists => TRUE);
        FOR compressed_chunk IN
            SELECT format('%I.%I', chunk_schema, chunk_name)::regclass
            FROM timescaledb_information.chunks
            WHERE hypertable_schema = current_schema()
              AND hypertable_name = 'disk_physical_metrics'
              AND is_compressed
        LOOP
            PERFORM decompress_chunk(compressed_chunk, true);
        END LOOP;
        ALTER TABLE disk_physical_metrics
            ALTER COLUMN name TYPE VARCHAR(255),
            ALTER COLUMN ref TYPE VARCHAR(320),
            ALTER COLUMN path TYPE TEXT;
    END IF;

    SELECT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'disk_usage_metrics'
          AND (
              (column_name = 'name' AND character_maximum_length IS DISTINCT FROM 255)
              OR (column_name = 'ref' AND character_maximum_length IS DISTINCT FROM 320)
              OR (column_name = 'mountpoint' AND data_type <> 'text')
              OR (column_name = 'path' AND data_type <> 'text')
          )
    ) INTO change_disk_usage;

    IF change_disk_usage THEN
        DROP MATERIALIZED VIEW IF EXISTS disk_usage_metrics_15m;
        DROP MATERIALIZED VIEW IF EXISTS disk_usage_metrics_1h;
        PERFORM remove_compression_policy('disk_usage_metrics', if_exists => TRUE);
        FOR compressed_chunk IN
            SELECT format('%I.%I', chunk_schema, chunk_name)::regclass
            FROM timescaledb_information.chunks
            WHERE hypertable_schema = current_schema()
              AND hypertable_name = 'disk_usage_metrics'
              AND is_compressed
        LOOP
            PERFORM decompress_chunk(compressed_chunk, true);
        END LOOP;
        ALTER TABLE disk_usage_metrics
            ALTER COLUMN name TYPE VARCHAR(255),
            ALTER COLUMN ref TYPE VARCHAR(320),
            ALTER COLUMN mountpoint TYPE TEXT,
            ALTER COLUMN path TYPE TEXT;
    END IF;
END
$migration$;

ALTER TABLE server_current_disk_metrics
    ALTER COLUMN name TYPE VARCHAR(255),
    ALTER COLUMN ref TYPE VARCHAR(320),
    ALTER COLUMN path TYPE TEXT;

ALTER TABLE server_current_disk_usage_metrics
    ALTER COLUMN name TYPE VARCHAR(255),
    ALTER COLUMN ref TYPE VARCHAR(320),
    ALTER COLUMN mountpoint TYPE TEXT,
    ALTER COLUMN path TYPE TEXT;

SELECT add_compression_policy('disk_metrics', INTERVAL '7 days');
SELECT add_compression_policy('disk_physical_metrics', INTERVAL '7 days');
SELECT add_compression_policy('disk_usage_metrics', INTERVAL '7 days');

CREATE MATERIALIZED VIEW disk_metrics_15m
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false
) AS
SELECT
    time_bucket('15 minutes', collected_at) AS bucket,
    server_id,
    name,
    ref,
    avg(read_rate_bytes_per_sec) AS read_bps_avg,
    min(read_rate_bytes_per_sec) AS read_bps_min,
    max(read_rate_bytes_per_sec) AS read_bps_max,
    last(read_rate_bytes_per_sec, collected_at) AS read_bps_last,
    avg(write_rate_bytes_per_sec) AS write_bps_avg,
    min(write_rate_bytes_per_sec) AS write_bps_min,
    max(write_rate_bytes_per_sec) AS write_bps_max,
    last(write_rate_bytes_per_sec, collected_at) AS write_bps_last,
    avg(read_iops) AS read_iops_avg,
    min(read_iops) AS read_iops_min,
    max(read_iops) AS read_iops_max,
    last(read_iops, collected_at) AS read_iops_last,
    avg(write_iops) AS write_iops_avg,
    min(write_iops) AS write_iops_min,
    max(write_iops) AS write_iops_max,
    last(write_iops, collected_at) AS write_iops_last,
    avg(iops) AS iops_avg,
    min(iops) AS iops_min,
    max(iops) AS iops_max,
    last(iops, collected_at) AS iops_last
FROM disk_metrics
GROUP BY bucket, server_id, name, ref
WITH NO DATA;

CREATE MATERIALIZED VIEW disk_metrics_1h
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false
) AS
SELECT
    time_bucket('1 hour', collected_at) AS bucket,
    server_id,
    name,
    ref,
    avg(read_rate_bytes_per_sec) AS read_bps_avg,
    min(read_rate_bytes_per_sec) AS read_bps_min,
    max(read_rate_bytes_per_sec) AS read_bps_max,
    last(read_rate_bytes_per_sec, collected_at) AS read_bps_last,
    avg(write_rate_bytes_per_sec) AS write_bps_avg,
    min(write_rate_bytes_per_sec) AS write_bps_min,
    max(write_rate_bytes_per_sec) AS write_bps_max,
    last(write_rate_bytes_per_sec, collected_at) AS write_bps_last,
    avg(read_iops) AS read_iops_avg,
    min(read_iops) AS read_iops_min,
    max(read_iops) AS read_iops_max,
    last(read_iops, collected_at) AS read_iops_last,
    avg(write_iops) AS write_iops_avg,
    min(write_iops) AS write_iops_min,
    max(write_iops) AS write_iops_max,
    last(write_iops, collected_at) AS write_iops_last,
    avg(iops) AS iops_avg,
    min(iops) AS iops_min,
    max(iops) AS iops_max,
    last(iops, collected_at) AS iops_last
FROM disk_metrics
GROUP BY bucket, server_id, name, ref
WITH NO DATA;

CREATE MATERIALIZED VIEW disk_usage_metrics_15m
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false
) AS
SELECT
    time_bucket('15 minutes', collected_at) AS bucket,
    server_id,
    name,
    ref,
    mountpoint,
    avg(used) AS used_bytes_avg,
    min(used) AS used_bytes_min,
    max(used) AS used_bytes_max,
    last(used, collected_at) AS used_bytes_last,
    avg(used_ratio) AS used_ratio_avg,
    min(used_ratio) AS used_ratio_min,
    max(used_ratio) AS used_ratio_max,
    last(used_ratio, collected_at) AS used_ratio_last
FROM disk_usage_metrics
GROUP BY bucket, server_id, name, ref, mountpoint
WITH NO DATA;

CREATE MATERIALIZED VIEW disk_usage_metrics_1h
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false
) AS
SELECT
    time_bucket('1 hour', collected_at) AS bucket,
    server_id,
    name,
    ref,
    mountpoint,
    avg(used) AS used_bytes_avg,
    min(used) AS used_bytes_min,
    max(used) AS used_bytes_max,
    last(used, collected_at) AS used_bytes_last,
    avg(used_ratio) AS used_ratio_avg,
    min(used_ratio) AS used_ratio_min,
    max(used_ratio) AS used_ratio_max,
    last(used_ratio, collected_at) AS used_ratio_last
FROM disk_usage_metrics
GROUP BY bucket, server_id, name, ref, mountpoint
WITH NO DATA;

SELECT add_continuous_aggregate_policy('disk_usage_metrics_15m',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');

SELECT add_continuous_aggregate_policy('disk_usage_metrics_1h',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes');

SELECT add_continuous_aggregate_policy('disk_metrics_15m',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');

SELECT add_continuous_aggregate_policy('disk_metrics_1h',
    start_offset => INTERVAL '31 days',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes');

ALTER TABLE traffic_month_usage
    ADD COLUMN covered_from TIMESTAMPTZ;

-- Existing rows predate explicit lower-bound tracking. Preserve their previous
-- completeness semantics; newly materialized rows record the real first
-- consumed source boundary.
UPDATE traffic_month_usage
SET covered_from = cycle_start
WHERE covered_from IS NULL;

ALTER TABLE traffic_month_usage
    ALTER COLUMN covered_from SET NOT NULL;

COMMENT ON COLUMN traffic_month_usage.covered_from IS '统计实际覆盖起点；晚于账期起点时表示部分账期';

CREATE TABLE traffic_materialization_progress (
    kind                    VARCHAR(16)  PRIMARY KEY,
    scanned_until           TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_traffic_materialization_progress_kind
        CHECK (kind IN ('usage', 'facts'))
);

INSERT INTO traffic_materialization_progress (kind, scanned_until)
VALUES
    ('usage', date_trunc('minute', now()) - INTERVAL '30 minutes'),
    ('facts', date_trunc('minute', now()) - INTERVAL '30 minutes')
ON CONFLICT (kind) DO NOTHING;

COMMENT ON TABLE traffic_materialization_progress IS '流量物化扫描高水位；每种固定物化器一行';
COMMENT ON COLUMN traffic_materialization_progress.kind IS '固定物化器：usage / facts';
COMMENT ON COLUMN traffic_materialization_progress.scanned_until IS '该时间之前的原始采样已尝试处理，不代表数据完整';

SELECT ensure_updated_at_trigger('traffic_materialization_progress');

CREATE TABLE traffic_usage_repairs (
    server_id               BIGINT      PRIMARY KEY REFERENCES servers (id) ON DELETE CASCADE,
    scanned_until           TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE traffic_usage_repairs IS '账期变更触发的节点 Lite 累计局部修复；存在记录时实时 Usage 扫描跳过该节点';
COMMENT ON COLUMN traffic_usage_repairs.scanned_until IS '该节点局部修复已扫描到的原始指标时间';

SELECT ensure_updated_at_trigger('traffic_usage_repairs');
