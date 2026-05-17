-- +goose Up

-- Keep bulky SMART runtime details out of metric history while storing the
-- stable temperature series needed by charts and alert evaluation.

ALTER TABLE server_metrics
    DROP COLUMN IF EXISTS disk_smart,
    ADD COLUMN IF NOT EXISTS cpu_temp_c DOUBLE PRECISION;

ALTER TABLE server_current_metrics
    DROP COLUMN IF EXISTS disk_smart,
    ADD COLUMN IF NOT EXISTS cpu_temp_c DOUBLE PRECISION;

CREATE TABLE IF NOT EXISTS disk_physical_metrics (
    server_id                 BIGINT       NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name                      VARCHAR(128) NOT NULL,
    ref                       VARCHAR(128) NOT NULL DEFAULT '',
    path                      VARCHAR(256) NOT NULL DEFAULT '',
    collected_at              TIMESTAMPTZ  NOT NULL,
    temp_c                    DOUBLE PRECISION NOT NULL,

    PRIMARY KEY (server_id, name, collected_at)
);

SELECT create_hypertable(
    'disk_physical_metrics',
    'collected_at',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists       => TRUE
);

ALTER TABLE disk_physical_metrics
SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'collected_at DESC',
    timescaledb.compress_segmentby = 'server_id, name'
);

SELECT add_compression_policy('disk_physical_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('disk_physical_metrics', INTERVAL '45 days', if_not_exists => TRUE);

COMMENT ON COLUMN server_metrics.cpu_temp_c IS 'Maximum CPU temperature in Celsius';
COMMENT ON COLUMN server_current_metrics.cpu_temp_c IS 'Current maximum CPU temperature in Celsius';
COMMENT ON TABLE disk_physical_metrics IS 'Physical disk temperature time series';
COMMENT ON COLUMN disk_physical_metrics.server_id IS 'Server ID';
COMMENT ON COLUMN disk_physical_metrics.name IS 'Physical disk name';
COMMENT ON COLUMN disk_physical_metrics.ref IS 'Physical disk reference';
COMMENT ON COLUMN disk_physical_metrics.path IS 'Physical disk device path';
COMMENT ON COLUMN disk_physical_metrics.collected_at IS 'Collection time';
COMMENT ON COLUMN disk_physical_metrics.temp_c IS 'Physical disk temperature in Celsius';

DELETE FROM alert_rule_mounts
WHERE rule_id = -5;
