DROP MATERIALIZED VIEW server_metrics_15m;
DROP MATERIALIZED VIEW server_metrics_1h;
DROP MATERIALIZED VIEW disk_metrics_15m;
DROP MATERIALIZED VIEW disk_usage_metrics_15m;

CREATE MATERIALIZED VIEW server_metrics_15m
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false,
    timescaledb.create_group_indexes = false
) AS
SELECT
    time_bucket('15 minutes', collected_at) AS bucket,
    server_id,
    avg(cpu_usage_ratio) AS cpu_usage_ratio_avg,
    min(cpu_usage_ratio) AS cpu_usage_ratio_min,
    max(cpu_usage_ratio) AS cpu_usage_ratio_max,
    last(cpu_usage_ratio, collected_at) AS cpu_usage_ratio_last,
    count(cpu_usage_ratio) AS cpu_usage_ratio_count,
    avg(load1) AS load1_avg,
    min(load1) AS load1_min,
    max(load1) AS load1_max,
    last(load1, collected_at) AS load1_last,
    count(load1) AS load1_count,
    avg(load5) AS load5_avg,
    min(load5) AS load5_min,
    max(load5) AS load5_max,
    last(load5, collected_at) AS load5_last,
    count(load5) AS load5_count,
    avg(load15) AS load15_avg,
    min(load15) AS load15_min,
    max(load15) AS load15_max,
    last(load15, collected_at) AS load15_last,
    count(load15) AS load15_count,
    avg(cpu_temp_c) AS cpu_temp_c_avg,
    min(cpu_temp_c) AS cpu_temp_c_min,
    max(cpu_temp_c) AS cpu_temp_c_max,
    last(cpu_temp_c, collected_at) AS cpu_temp_c_last,
    count(cpu_temp_c) AS cpu_temp_c_count,
    avg(mem_used) AS mem_used_avg,
    min(mem_used) AS mem_used_min,
    max(mem_used) AS mem_used_max,
    last(mem_used, collected_at) AS mem_used_last,
    count(mem_used) AS mem_used_count,
    avg(mem_used_ratio) AS mem_used_ratio_avg,
    min(mem_used_ratio) AS mem_used_ratio_min,
    max(mem_used_ratio) AS mem_used_ratio_max,
    last(mem_used_ratio, collected_at) AS mem_used_ratio_last,
    count(mem_used_ratio) AS mem_used_ratio_count,
    avg(process_count) AS process_count_avg,
    min(process_count) AS process_count_min,
    max(process_count) AS process_count_max,
    last(process_count, collected_at) AS process_count_last,
    count(process_count) AS process_count_count,
    avg(net_in_bps) AS net_in_bps_avg,
    min(net_in_bps) AS net_in_bps_min,
    max(net_in_bps) AS net_in_bps_max,
    last(net_in_bps, collected_at) AS net_in_bps_last,
    count(net_in_bps) AS net_in_bps_count,
    avg(net_out_bps) AS net_out_bps_avg,
    min(net_out_bps) AS net_out_bps_min,
    max(net_out_bps) AS net_out_bps_max,
    last(net_out_bps, collected_at) AS net_out_bps_last,
    count(net_out_bps) AS net_out_bps_count,
    avg(tcp_conn) AS tcp_conn_avg,
    min(tcp_conn) AS tcp_conn_min,
    max(tcp_conn) AS tcp_conn_max,
    last(tcp_conn, collected_at) AS tcp_conn_last,
    count(tcp_conn) AS tcp_conn_count,
    avg(udp_conn) AS udp_conn_avg,
    min(udp_conn) AS udp_conn_min,
    max(udp_conn) AS udp_conn_max,
    last(udp_conn, collected_at) AS udp_conn_last,
    count(udp_conn) AS udp_conn_count,
    avg(psi_cpu_some_avg10) AS psi_cpu_some_avg10_avg,
    min(psi_cpu_some_avg10) AS psi_cpu_some_avg10_min,
    max(psi_cpu_some_avg10) AS psi_cpu_some_avg10_max,
    last(psi_cpu_some_avg10, collected_at) AS psi_cpu_some_avg10_last,
    count(psi_cpu_some_avg10) AS psi_cpu_some_avg10_count,
    avg(psi_cpu_some_avg60) AS psi_cpu_some_avg60_avg,
    min(psi_cpu_some_avg60) AS psi_cpu_some_avg60_min,
    max(psi_cpu_some_avg60) AS psi_cpu_some_avg60_max,
    last(psi_cpu_some_avg60, collected_at) AS psi_cpu_some_avg60_last,
    count(psi_cpu_some_avg60) AS psi_cpu_some_avg60_count,
    avg(psi_cpu_some_avg300) AS psi_cpu_some_avg300_avg,
    min(psi_cpu_some_avg300) AS psi_cpu_some_avg300_min,
    max(psi_cpu_some_avg300) AS psi_cpu_some_avg300_max,
    last(psi_cpu_some_avg300, collected_at) AS psi_cpu_some_avg300_last,
    count(psi_cpu_some_avg300) AS psi_cpu_some_avg300_count,
    avg(psi_memory_some_avg10) AS psi_memory_some_avg10_avg,
    min(psi_memory_some_avg10) AS psi_memory_some_avg10_min,
    max(psi_memory_some_avg10) AS psi_memory_some_avg10_max,
    last(psi_memory_some_avg10, collected_at) AS psi_memory_some_avg10_last,
    count(psi_memory_some_avg10) AS psi_memory_some_avg10_count,
    avg(psi_memory_some_avg60) AS psi_memory_some_avg60_avg,
    min(psi_memory_some_avg60) AS psi_memory_some_avg60_min,
    max(psi_memory_some_avg60) AS psi_memory_some_avg60_max,
    last(psi_memory_some_avg60, collected_at) AS psi_memory_some_avg60_last,
    count(psi_memory_some_avg60) AS psi_memory_some_avg60_count,
    avg(psi_memory_some_avg300) AS psi_memory_some_avg300_avg,
    min(psi_memory_some_avg300) AS psi_memory_some_avg300_min,
    max(psi_memory_some_avg300) AS psi_memory_some_avg300_max,
    last(psi_memory_some_avg300, collected_at) AS psi_memory_some_avg300_last,
    count(psi_memory_some_avg300) AS psi_memory_some_avg300_count,
    avg(psi_memory_full_avg10) AS psi_memory_full_avg10_avg,
    min(psi_memory_full_avg10) AS psi_memory_full_avg10_min,
    max(psi_memory_full_avg10) AS psi_memory_full_avg10_max,
    last(psi_memory_full_avg10, collected_at) AS psi_memory_full_avg10_last,
    count(psi_memory_full_avg10) AS psi_memory_full_avg10_count,
    avg(psi_memory_full_avg60) AS psi_memory_full_avg60_avg,
    min(psi_memory_full_avg60) AS psi_memory_full_avg60_min,
    max(psi_memory_full_avg60) AS psi_memory_full_avg60_max,
    last(psi_memory_full_avg60, collected_at) AS psi_memory_full_avg60_last,
    count(psi_memory_full_avg60) AS psi_memory_full_avg60_count,
    avg(psi_memory_full_avg300) AS psi_memory_full_avg300_avg,
    min(psi_memory_full_avg300) AS psi_memory_full_avg300_min,
    max(psi_memory_full_avg300) AS psi_memory_full_avg300_max,
    last(psi_memory_full_avg300, collected_at) AS psi_memory_full_avg300_last,
    count(psi_memory_full_avg300) AS psi_memory_full_avg300_count,
    avg(psi_io_some_avg10) AS psi_io_some_avg10_avg,
    min(psi_io_some_avg10) AS psi_io_some_avg10_min,
    max(psi_io_some_avg10) AS psi_io_some_avg10_max,
    last(psi_io_some_avg10, collected_at) AS psi_io_some_avg10_last,
    count(psi_io_some_avg10) AS psi_io_some_avg10_count,
    avg(psi_io_some_avg60) AS psi_io_some_avg60_avg,
    min(psi_io_some_avg60) AS psi_io_some_avg60_min,
    max(psi_io_some_avg60) AS psi_io_some_avg60_max,
    last(psi_io_some_avg60, collected_at) AS psi_io_some_avg60_last,
    count(psi_io_some_avg60) AS psi_io_some_avg60_count,
    avg(psi_io_some_avg300) AS psi_io_some_avg300_avg,
    min(psi_io_some_avg300) AS psi_io_some_avg300_min,
    max(psi_io_some_avg300) AS psi_io_some_avg300_max,
    last(psi_io_some_avg300, collected_at) AS psi_io_some_avg300_last,
    count(psi_io_some_avg300) AS psi_io_some_avg300_count,
    avg(psi_io_full_avg10) AS psi_io_full_avg10_avg,
    min(psi_io_full_avg10) AS psi_io_full_avg10_min,
    max(psi_io_full_avg10) AS psi_io_full_avg10_max,
    last(psi_io_full_avg10, collected_at) AS psi_io_full_avg10_last,
    count(psi_io_full_avg10) AS psi_io_full_avg10_count,
    avg(psi_io_full_avg60) AS psi_io_full_avg60_avg,
    min(psi_io_full_avg60) AS psi_io_full_avg60_min,
    max(psi_io_full_avg60) AS psi_io_full_avg60_max,
    last(psi_io_full_avg60, collected_at) AS psi_io_full_avg60_last,
    count(psi_io_full_avg60) AS psi_io_full_avg60_count,
    avg(psi_io_full_avg300) AS psi_io_full_avg300_avg,
    min(psi_io_full_avg300) AS psi_io_full_avg300_min,
    max(psi_io_full_avg300) AS psi_io_full_avg300_max,
    last(psi_io_full_avg300, collected_at) AS psi_io_full_avg300_last,
    count(psi_io_full_avg300) AS psi_io_full_avg300_count
FROM server_metrics
GROUP BY bucket, server_id
WITH NO DATA;

CREATE MATERIALIZED VIEW server_metrics_1h
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false,
    timescaledb.create_group_indexes = false
) AS
SELECT
    time_bucket('1 hour', collected_at) AS bucket,
    server_id,
    avg(cpu_usage_ratio) AS cpu_usage_ratio_avg,
    min(cpu_usage_ratio) AS cpu_usage_ratio_min,
    max(cpu_usage_ratio) AS cpu_usage_ratio_max,
    last(cpu_usage_ratio, collected_at) AS cpu_usage_ratio_last,
    avg(load1) AS load1_avg,
    min(load1) AS load1_min,
    max(load1) AS load1_max,
    last(load1, collected_at) AS load1_last,
    avg(load5) AS load5_avg,
    min(load5) AS load5_min,
    max(load5) AS load5_max,
    last(load5, collected_at) AS load5_last,
    avg(load15) AS load15_avg,
    min(load15) AS load15_min,
    max(load15) AS load15_max,
    last(load15, collected_at) AS load15_last,
    avg(cpu_temp_c) AS cpu_temp_c_avg,
    min(cpu_temp_c) AS cpu_temp_c_min,
    max(cpu_temp_c) AS cpu_temp_c_max,
    last(cpu_temp_c, collected_at) AS cpu_temp_c_last,
    avg(mem_used) AS mem_used_avg,
    min(mem_used) AS mem_used_min,
    max(mem_used) AS mem_used_max,
    last(mem_used, collected_at) AS mem_used_last,
    avg(mem_used_ratio) AS mem_used_ratio_avg,
    min(mem_used_ratio) AS mem_used_ratio_min,
    max(mem_used_ratio) AS mem_used_ratio_max,
    last(mem_used_ratio, collected_at) AS mem_used_ratio_last,
    avg(process_count) AS process_count_avg,
    min(process_count) AS process_count_min,
    max(process_count) AS process_count_max,
    last(process_count, collected_at) AS process_count_last,
    avg(net_in_bps) AS net_in_bps_avg,
    min(net_in_bps) AS net_in_bps_min,
    max(net_in_bps) AS net_in_bps_max,
    last(net_in_bps, collected_at) AS net_in_bps_last,
    avg(net_out_bps) AS net_out_bps_avg,
    min(net_out_bps) AS net_out_bps_min,
    max(net_out_bps) AS net_out_bps_max,
    last(net_out_bps, collected_at) AS net_out_bps_last,
    avg(tcp_conn) AS tcp_conn_avg,
    min(tcp_conn) AS tcp_conn_min,
    max(tcp_conn) AS tcp_conn_max,
    last(tcp_conn, collected_at) AS tcp_conn_last,
    avg(udp_conn) AS udp_conn_avg,
    min(udp_conn) AS udp_conn_min,
    max(udp_conn) AS udp_conn_max,
    last(udp_conn, collected_at) AS udp_conn_last,
    avg(psi_cpu_some_avg10) AS psi_cpu_some_avg10_avg,
    min(psi_cpu_some_avg10) AS psi_cpu_some_avg10_min,
    max(psi_cpu_some_avg10) AS psi_cpu_some_avg10_max,
    last(psi_cpu_some_avg10, collected_at) AS psi_cpu_some_avg10_last,
    avg(psi_cpu_some_avg60) AS psi_cpu_some_avg60_avg,
    min(psi_cpu_some_avg60) AS psi_cpu_some_avg60_min,
    max(psi_cpu_some_avg60) AS psi_cpu_some_avg60_max,
    last(psi_cpu_some_avg60, collected_at) AS psi_cpu_some_avg60_last,
    avg(psi_cpu_some_avg300) AS psi_cpu_some_avg300_avg,
    min(psi_cpu_some_avg300) AS psi_cpu_some_avg300_min,
    max(psi_cpu_some_avg300) AS psi_cpu_some_avg300_max,
    last(psi_cpu_some_avg300, collected_at) AS psi_cpu_some_avg300_last,
    avg(psi_memory_some_avg10) AS psi_memory_some_avg10_avg,
    min(psi_memory_some_avg10) AS psi_memory_some_avg10_min,
    max(psi_memory_some_avg10) AS psi_memory_some_avg10_max,
    last(psi_memory_some_avg10, collected_at) AS psi_memory_some_avg10_last,
    avg(psi_memory_some_avg60) AS psi_memory_some_avg60_avg,
    min(psi_memory_some_avg60) AS psi_memory_some_avg60_min,
    max(psi_memory_some_avg60) AS psi_memory_some_avg60_max,
    last(psi_memory_some_avg60, collected_at) AS psi_memory_some_avg60_last,
    avg(psi_memory_some_avg300) AS psi_memory_some_avg300_avg,
    min(psi_memory_some_avg300) AS psi_memory_some_avg300_min,
    max(psi_memory_some_avg300) AS psi_memory_some_avg300_max,
    last(psi_memory_some_avg300, collected_at) AS psi_memory_some_avg300_last,
    avg(psi_memory_full_avg10) AS psi_memory_full_avg10_avg,
    min(psi_memory_full_avg10) AS psi_memory_full_avg10_min,
    max(psi_memory_full_avg10) AS psi_memory_full_avg10_max,
    last(psi_memory_full_avg10, collected_at) AS psi_memory_full_avg10_last,
    avg(psi_memory_full_avg60) AS psi_memory_full_avg60_avg,
    min(psi_memory_full_avg60) AS psi_memory_full_avg60_min,
    max(psi_memory_full_avg60) AS psi_memory_full_avg60_max,
    last(psi_memory_full_avg60, collected_at) AS psi_memory_full_avg60_last,
    avg(psi_memory_full_avg300) AS psi_memory_full_avg300_avg,
    min(psi_memory_full_avg300) AS psi_memory_full_avg300_min,
    max(psi_memory_full_avg300) AS psi_memory_full_avg300_max,
    last(psi_memory_full_avg300, collected_at) AS psi_memory_full_avg300_last,
    avg(psi_io_some_avg10) AS psi_io_some_avg10_avg,
    min(psi_io_some_avg10) AS psi_io_some_avg10_min,
    max(psi_io_some_avg10) AS psi_io_some_avg10_max,
    last(psi_io_some_avg10, collected_at) AS psi_io_some_avg10_last,
    avg(psi_io_some_avg60) AS psi_io_some_avg60_avg,
    min(psi_io_some_avg60) AS psi_io_some_avg60_min,
    max(psi_io_some_avg60) AS psi_io_some_avg60_max,
    last(psi_io_some_avg60, collected_at) AS psi_io_some_avg60_last,
    avg(psi_io_some_avg300) AS psi_io_some_avg300_avg,
    min(psi_io_some_avg300) AS psi_io_some_avg300_min,
    max(psi_io_some_avg300) AS psi_io_some_avg300_max,
    last(psi_io_some_avg300, collected_at) AS psi_io_some_avg300_last,
    avg(psi_io_full_avg10) AS psi_io_full_avg10_avg,
    min(psi_io_full_avg10) AS psi_io_full_avg10_min,
    max(psi_io_full_avg10) AS psi_io_full_avg10_max,
    last(psi_io_full_avg10, collected_at) AS psi_io_full_avg10_last,
    avg(psi_io_full_avg60) AS psi_io_full_avg60_avg,
    min(psi_io_full_avg60) AS psi_io_full_avg60_min,
    max(psi_io_full_avg60) AS psi_io_full_avg60_max,
    last(psi_io_full_avg60, collected_at) AS psi_io_full_avg60_last,
    avg(psi_io_full_avg300) AS psi_io_full_avg300_avg,
    min(psi_io_full_avg300) AS psi_io_full_avg300_min,
    max(psi_io_full_avg300) AS psi_io_full_avg300_max,
    last(psi_io_full_avg300, collected_at) AS psi_io_full_avg300_last
FROM server_metrics
GROUP BY bucket, server_id
WITH NO DATA;

CREATE MATERIALIZED VIEW disk_metrics_15m
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false,
    timescaledb.create_group_indexes = false
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
    count(read_rate_bytes_per_sec) AS read_bps_count,
    avg(write_rate_bytes_per_sec) AS write_bps_avg,
    min(write_rate_bytes_per_sec) AS write_bps_min,
    max(write_rate_bytes_per_sec) AS write_bps_max,
    last(write_rate_bytes_per_sec, collected_at) AS write_bps_last,
    count(write_rate_bytes_per_sec) AS write_bps_count,
    avg(read_iops) AS read_iops_avg,
    min(read_iops) AS read_iops_min,
    max(read_iops) AS read_iops_max,
    last(read_iops, collected_at) AS read_iops_last,
    count(read_iops) AS read_iops_count,
    avg(write_iops) AS write_iops_avg,
    min(write_iops) AS write_iops_min,
    max(write_iops) AS write_iops_max,
    last(write_iops, collected_at) AS write_iops_last,
    count(write_iops) AS write_iops_count,
    avg(iops) AS iops_avg,
    min(iops) AS iops_min,
    max(iops) AS iops_max,
    last(iops, collected_at) AS iops_last,
    count(iops) AS iops_count
FROM disk_metrics
GROUP BY bucket, server_id, name, ref
WITH NO DATA;

CREATE MATERIALIZED VIEW disk_usage_metrics_15m
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false,
    timescaledb.create_group_indexes = false
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
    count(used) AS used_bytes_count,
    avg(used_ratio) AS used_ratio_avg,
    min(used_ratio) AS used_ratio_min,
    max(used_ratio) AS used_ratio_max,
    last(used_ratio, collected_at) AS used_ratio_last,
    count(used_ratio) AS used_ratio_count
FROM disk_usage_metrics
GROUP BY bucket, server_id, name, ref, mountpoint
WITH NO DATA;

CREATE MATERIALIZED VIEW disk_physical_metrics_15m
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false,
    timescaledb.create_group_indexes = false
) AS
SELECT
    time_bucket('15 minutes', collected_at) AS bucket,
    server_id,
    name,
    ref,
    path,
    avg(temp_c) AS temp_c_avg,
    min(temp_c) AS temp_c_min,
    max(temp_c) AS temp_c_max,
    last(temp_c, collected_at) AS temp_c_last,
    count(temp_c) AS temp_c_count
FROM disk_physical_metrics
GROUP BY bucket, server_id, name, ref, path
WITH NO DATA;

CREATE MATERIALIZED VIEW disk_physical_metrics_1h
WITH (
    timescaledb.continuous,
    timescaledb.materialized_only = false,
    timescaledb.create_group_indexes = false
) AS
SELECT
    time_bucket('1 hour', collected_at) AS bucket,
    server_id,
    name,
    ref,
    path,
    avg(temp_c) AS temp_c_avg,
    min(temp_c) AS temp_c_min,
    max(temp_c) AS temp_c_max,
    last(temp_c, collected_at) AS temp_c_last
FROM disk_physical_metrics
GROUP BY bucket, server_id, name, ref, path
WITH NO DATA;

ALTER MATERIALIZED VIEW disk_metrics_1h
    SET (timescaledb.materialized_only = false);
ALTER MATERIALIZED VIEW disk_usage_metrics_1h
    SET (timescaledb.materialized_only = false);
ALTER MATERIALIZED VIEW server_online_30m
    SET (timescaledb.materialized_only = true);

CREATE INDEX idx_server_metrics_15m_server_bucket
    ON server_metrics_15m (server_id, bucket DESC);
CREATE INDEX idx_server_metrics_1h_server_bucket
    ON server_metrics_1h (server_id, bucket DESC);
CREATE INDEX idx_disk_metrics_15m_server_bucket
    ON disk_metrics_15m (server_id, bucket DESC);
CREATE INDEX idx_disk_usage_metrics_15m_server_bucket
    ON disk_usage_metrics_15m (server_id, bucket DESC);
CREATE INDEX idx_disk_physical_metrics_15m_server_bucket
    ON disk_physical_metrics_15m (server_id, bucket DESC);
CREATE INDEX idx_disk_physical_metrics_1h_server_bucket
    ON disk_physical_metrics_1h (server_id, bucket DESC);

-- The retained 1-hour disk aggregates were created with one index per text
-- grouping column. History queries narrow by server and bucket first, so those
-- indexes only add write and storage cost.
DO $migration$
DECLARE
    target RECORD;
BEGIN
    FOR target IN
        SELECT index_ns.nspname AS index_schema,
               index_rel.relname AS index_name
        FROM timescaledb_information.continuous_aggregates aggregate_info
        JOIN pg_namespace table_ns
          ON table_ns.nspname = aggregate_info.materialization_hypertable_schema
        JOIN pg_class table_rel
          ON table_rel.relnamespace = table_ns.oid
         AND table_rel.relname = aggregate_info.materialization_hypertable_name
        JOIN pg_index index_info
          ON index_info.indrelid = table_rel.oid
        JOIN pg_class index_rel
          ON index_rel.oid = index_info.indexrelid
        JOIN pg_namespace index_ns
          ON index_ns.oid = index_rel.relnamespace
        JOIN pg_attribute first_column
          ON first_column.attrelid = table_rel.oid
         AND first_column.attnum = index_info.indkey[0]
        WHERE aggregate_info.view_schema = current_schema()
          AND aggregate_info.view_name IN ('disk_metrics_1h', 'disk_usage_metrics_1h')
          AND first_column.attname IN ('name', 'ref', 'mountpoint', 'path')
    LOOP
        EXECUTE format('DROP INDEX %I.%I', target.index_schema, target.index_name);
    END LOOP;
END
$migration$;

DROP INDEX IF EXISTS idx_sm_server_collected_at;

SELECT set_chunk_time_interval('server_metrics', INTERVAL '1 hour');
SELECT set_chunk_time_interval('disk_metrics', INTERVAL '1 hour');
SELECT set_chunk_time_interval('disk_usage_metrics', INTERVAL '1 hour');
SELECT set_chunk_time_interval('disk_physical_metrics', INTERVAL '1 hour');
