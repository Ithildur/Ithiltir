SELECT remove_continuous_aggregate_policy('server_metrics_15m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_metrics_15m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_usage_metrics_15m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_physical_metrics_15m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('server_metrics_1h', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_metrics_1h', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_usage_metrics_1h', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_physical_metrics_1h', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('server_online_30m', if_exists => TRUE);

SELECT add_continuous_aggregate_policy('server_metrics_15m',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');
SELECT add_continuous_aggregate_policy('disk_metrics_15m',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');
SELECT add_continuous_aggregate_policy('disk_usage_metrics_15m',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');
SELECT add_continuous_aggregate_policy('disk_physical_metrics_15m',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');

SELECT add_continuous_aggregate_policy('server_metrics_1h',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes');
SELECT add_continuous_aggregate_policy('disk_metrics_1h',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes');
SELECT add_continuous_aggregate_policy('disk_usage_metrics_1h',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes');
SELECT add_continuous_aggregate_policy('disk_physical_metrics_1h',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes');

SELECT add_continuous_aggregate_policy('server_online_30m',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes');

SELECT remove_retention_policy('server_metrics_15m', if_exists => TRUE);
SELECT remove_retention_policy('disk_metrics_15m', if_exists => TRUE);
SELECT remove_retention_policy('disk_usage_metrics_15m', if_exists => TRUE);
SELECT remove_retention_policy('disk_physical_metrics_15m', if_exists => TRUE);
SELECT remove_retention_policy('server_metrics_1h', if_exists => TRUE);
SELECT remove_retention_policy('disk_metrics_1h', if_exists => TRUE);
SELECT remove_retention_policy('disk_usage_metrics_1h', if_exists => TRUE);
SELECT remove_retention_policy('disk_physical_metrics_1h', if_exists => TRUE);
SELECT remove_retention_policy('server_online_30m', if_exists => TRUE);

SELECT add_retention_policy('server_metrics_15m', INTERVAL '16 days');
SELECT add_retention_policy('disk_metrics_15m', INTERVAL '16 days');
SELECT add_retention_policy('disk_usage_metrics_15m', INTERVAL '16 days');
SELECT add_retention_policy('disk_physical_metrics_15m', INTERVAL '16 days');
SELECT add_retention_policy('server_metrics_1h', INTERVAL '32 days');
SELECT add_retention_policy('disk_metrics_1h', INTERVAL '32 days');
SELECT add_retention_policy('disk_usage_metrics_1h', INTERVAL '32 days');
SELECT add_retention_policy('disk_physical_metrics_1h', INTERVAL '32 days');
SELECT add_retention_policy('server_online_30m', INTERVAL '8 days');

SELECT remove_compression_policy('server_metrics', if_exists => TRUE);
SELECT remove_compression_policy('disk_metrics', if_exists => TRUE);
SELECT remove_compression_policy('disk_usage_metrics', if_exists => TRUE);
SELECT remove_compression_policy('disk_physical_metrics', if_exists => TRUE);
SELECT add_compression_policy('server_metrics', INTERVAL '1 day');
SELECT add_compression_policy('disk_metrics', INTERVAL '1 day');
SELECT add_compression_policy('disk_usage_metrics', INTERVAL '1 day');
SELECT add_compression_policy('disk_physical_metrics', INTERVAL '1 day');

SELECT remove_retention_policy('server_metrics', if_exists => TRUE);
SELECT remove_retention_policy('disk_metrics', if_exists => TRUE);
SELECT remove_retention_policy('disk_usage_metrics', if_exists => TRUE);
SELECT remove_retention_policy('disk_physical_metrics', if_exists => TRUE);
SELECT add_retention_policy('server_metrics', INTERVAL '8 days');
SELECT add_retention_policy('disk_metrics', INTERVAL '8 days');
SELECT add_retention_policy('disk_usage_metrics', INTERVAL '8 days');
SELECT add_retention_policy('disk_physical_metrics', INTERVAL '8 days');
