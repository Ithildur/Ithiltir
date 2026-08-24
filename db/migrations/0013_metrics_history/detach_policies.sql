SELECT remove_continuous_aggregate_policy('server_metrics_15m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('server_metrics_1h', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('server_online_30m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_metrics_15m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_metrics_1h', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_usage_metrics_15m', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('disk_usage_metrics_1h', if_exists => TRUE);

SELECT remove_compression_policy('server_metrics', if_exists => TRUE);
SELECT remove_compression_policy('disk_metrics', if_exists => TRUE);
SELECT remove_compression_policy('disk_usage_metrics', if_exists => TRUE);
SELECT remove_compression_policy('disk_physical_metrics', if_exists => TRUE);

SELECT remove_retention_policy('server_metrics', if_exists => TRUE);
SELECT remove_retention_policy('disk_metrics', if_exists => TRUE);
SELECT remove_retention_policy('disk_usage_metrics', if_exists => TRUE);
SELECT remove_retention_policy('disk_physical_metrics', if_exists => TRUE);
