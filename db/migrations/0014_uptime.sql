-- +goose Up

CREATE TABLE IF NOT EXISTS node_online (
    server_id BIGINT      NOT NULL REFERENCES servers (id),
    minute    TIMESTAMPTZ NOT NULL,
    online    BOOLEAN     NOT NULL,

    PRIMARY KEY (server_id, minute),
    CONSTRAINT node_online_minute_aligned
        CHECK (minute = date_trunc('minute', minute))
);

SELECT create_hypertable(
    'node_online',
    'minute',
    chunk_time_interval => INTERVAL '1 day',
    create_default_indexes => FALSE,
    if_not_exists => TRUE
);

COMMENT ON TABLE node_online IS '节点分钟级在线状态；缺少记录表示未知';
COMMENT ON COLUMN node_online.server_id IS '节点 ID';
COMMENT ON COLUMN node_online.minute IS '采样分钟，时间对齐到整分钟';
COMMENT ON COLUMN node_online.online IS '该采样分钟的在线状态';

SELECT add_retention_policy('node_online', INTERVAL '46 days', if_not_exists => TRUE);

ALTER TABLE system_settings
    ADD COLUMN IF NOT EXISTS uptime_guest_visible BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS uptime_warning_sla DOUBLE PRECISION NOT NULL DEFAULT 99,
    ADD COLUMN IF NOT EXISTS uptime_error_sla DOUBLE PRECISION NOT NULL DEFAULT 95;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'system_settings'::regclass
          AND conname = 'system_settings_uptime_sla'
    ) THEN
        ALTER TABLE system_settings ADD CONSTRAINT system_settings_uptime_sla CHECK (
            uptime_error_sla >= 0
            AND uptime_error_sla < uptime_warning_sla
            AND uptime_warning_sla <= 100
        );
    END IF;
END;
$$;
-- +goose StatementEnd

COMMENT ON COLUMN system_settings.uptime_guest_visible IS '是否向游客展示节点 uptime';
COMMENT ON COLUMN system_settings.uptime_warning_sla IS '每日 uptime 黄色警告阈值，单位为百分数';
COMMENT ON COLUMN system_settings.uptime_error_sla IS '每日 uptime 红色错误阈值，单位为百分数';

-- +goose StatementBegin
CREATE OR REPLACE PROCEDURE sample_node_online(job_id INTEGER, config JSONB)
LANGUAGE plpgsql
SET search_path FROM CURRENT
AS $$
DECLARE
    sampled_at TIMESTAMPTZ := now();
    offline_after DOUBLE PRECISION := (config->>'offline_after_seconds')::DOUBLE PRECISION;
BEGIN
    IF offline_after IS NULL OR NOT (offline_after > 0 AND offline_after < 'Infinity'::DOUBLE PRECISION) THEN
        RAISE EXCEPTION 'offline_after_seconds must be finite and positive';
    END IF;

    -- Snapshot the current minute only. Missing minutes stay unknown; retries
    -- preserve the first observation instead of rewriting history.
    INSERT INTO node_online (server_id, minute, online)
    SELECT current.server_id,
           date_trunc('minute', sampled_at),
           current.collected_at >= sampled_at - make_interval(secs => offline_after)
    FROM server_current_metrics AS current
    JOIN servers AS node ON node.id = current.server_id
    WHERE NOT node.is_deleted
    ON CONFLICT (server_id, minute) DO NOTHING;
END;
$$;
-- +goose StatementEnd

SELECT add_job(
    'sample_node_online',
    INTERVAL '1 minute',
    config => '{"offline_after_seconds":17}'::JSONB,
    initial_start => date_trunc('minute', now()) + INTERVAL '1 minute',
    fixed_schedule => TRUE
)
WHERE NOT EXISTS (
    SELECT 1 FROM timescaledb_information.jobs
    WHERE proc_schema = current_schema() AND proc_name = 'sample_node_online'
);

-- +goose StatementBegin
CREATE OR REPLACE PROCEDURE configure_node_online_1h(zone TEXT)
LANGUAGE plpgsql
SET search_path FROM CURRENT
AS $$
DECLARE
    sampler INTEGER;
    previous_zone TEXT;
BEGIN
    -- Validate before replacing any derived data. Minute observations stay intact.
    IF zone IS NULL THEN
        RAISE EXCEPTION 'uptime timezone must not be null';
    END IF;
    PERFORM now() AT TIME ZONE zone;
    SELECT job_id, config->>'timezone' INTO STRICT sampler, previous_zone
    FROM timescaledb_information.jobs
    WHERE proc_schema = current_schema() AND proc_name = 'sample_node_online';
    IF previous_zone = zone AND to_regclass('node_online_1h') IS NOT NULL THEN
        RETURN;
    END IF;

    DROP MATERIALIZED VIEW IF EXISTS node_online_1h;
    EXECUTE format($view$
        CREATE MATERIALIZED VIEW node_online_1h
        WITH (timescaledb.continuous, timescaledb.materialized_only = FALSE,
              timescaledb.create_group_indexes = FALSE) AS
        SELECT server_id, time_bucket(INTERVAL '1 hour', minute, %L::TEXT) AS bucket,
               count(*) AS samples, count(*) FILTER (WHERE online) AS online
        FROM node_online GROUP BY server_id, bucket WITH NO DATA
    $view$, zone);
    CREATE INDEX idx_node_online_1h_server_bucket ON node_online_1h (server_id, bucket);
    PERFORM add_continuous_aggregate_policy('node_online_1h',
        start_offset => INTERVAL '46 days', end_offset => INTERVAL '0 seconds',
        schedule_interval => INTERVAL '1 hour', timezone => zone,
        initial_start => (date_trunc('hour', now() AT TIME ZONE zone) AT TIME ZONE zone)
                         + INTERVAL '1 hour 5 seconds');
    PERFORM add_retention_policy('node_online_1h', INTERVAL '46 days');
    PERFORM alter_job(sampler, config => jsonb_set(config, '{timezone}', to_jsonb(zone)))
    FROM timescaledb_information.jobs WHERE job_id = sampler;
END;
$$;
-- +goose StatementEnd

-- Reapplication preserves the timezone selected by application bootstrap.
-- +goose StatementBegin
DO $$
DECLARE
    zone TEXT;
BEGIN
    SELECT COALESCE(config->>'timezone', 'UTC') INTO STRICT zone
    FROM timescaledb_information.jobs
    WHERE proc_schema = current_schema() AND proc_name = 'sample_node_online';
    CALL configure_node_online_1h(zone);
END;
$$;
-- +goose StatementEnd
