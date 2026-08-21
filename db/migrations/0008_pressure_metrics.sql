-- +goose Up

-- Store Linux PSI pressure metrics as fixed numeric time-series columns.

ALTER TABLE server_metrics
    ADD COLUMN psi_cpu_some_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_some_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_some_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_some_total BIGINT,
    ADD COLUMN psi_cpu_full_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_full_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_full_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_full_total BIGINT,
    ADD COLUMN psi_memory_some_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_memory_some_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_memory_some_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_memory_some_total BIGINT,
    ADD COLUMN psi_memory_full_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_memory_full_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_memory_full_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_memory_full_total BIGINT,
    ADD COLUMN psi_io_some_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_io_some_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_io_some_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_io_some_total BIGINT,
    ADD COLUMN psi_io_full_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_io_full_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_io_full_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_io_full_total BIGINT;

ALTER TABLE server_current_metrics
    ADD COLUMN psi_cpu_some_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_some_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_some_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_some_total BIGINT,
    ADD COLUMN psi_cpu_full_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_full_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_full_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_cpu_full_total BIGINT,
    ADD COLUMN psi_memory_some_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_memory_some_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_memory_some_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_memory_some_total BIGINT,
    ADD COLUMN psi_memory_full_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_memory_full_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_memory_full_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_memory_full_total BIGINT,
    ADD COLUMN psi_io_some_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_io_some_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_io_some_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_io_some_total BIGINT,
    ADD COLUMN psi_io_full_avg10 DOUBLE PRECISION,
    ADD COLUMN psi_io_full_avg60 DOUBLE PRECISION,
    ADD COLUMN psi_io_full_avg300 DOUBLE PRECISION,
    ADD COLUMN psi_io_full_total BIGINT;

COMMENT ON COLUMN server_metrics.psi_cpu_some_avg10 IS 'CPU PSI some avg10 percentage';
COMMENT ON COLUMN server_metrics.psi_cpu_some_avg60 IS 'CPU PSI some avg60 percentage';
COMMENT ON COLUMN server_metrics.psi_cpu_some_avg300 IS 'CPU PSI some avg300 percentage';
COMMENT ON COLUMN server_metrics.psi_cpu_some_total IS 'CPU PSI some cumulative stall time in microseconds';
COMMENT ON COLUMN server_metrics.psi_cpu_full_avg10 IS 'CPU PSI full avg10 percentage';
COMMENT ON COLUMN server_metrics.psi_cpu_full_avg60 IS 'CPU PSI full avg60 percentage';
COMMENT ON COLUMN server_metrics.psi_cpu_full_avg300 IS 'CPU PSI full avg300 percentage';
COMMENT ON COLUMN server_metrics.psi_cpu_full_total IS 'CPU PSI full cumulative stall time in microseconds';
COMMENT ON COLUMN server_metrics.psi_memory_some_avg10 IS 'Memory PSI some avg10 percentage';
COMMENT ON COLUMN server_metrics.psi_memory_some_avg60 IS 'Memory PSI some avg60 percentage';
COMMENT ON COLUMN server_metrics.psi_memory_some_avg300 IS 'Memory PSI some avg300 percentage';
COMMENT ON COLUMN server_metrics.psi_memory_some_total IS 'Memory PSI some cumulative stall time in microseconds';
COMMENT ON COLUMN server_metrics.psi_memory_full_avg10 IS 'Memory PSI full avg10 percentage';
COMMENT ON COLUMN server_metrics.psi_memory_full_avg60 IS 'Memory PSI full avg60 percentage';
COMMENT ON COLUMN server_metrics.psi_memory_full_avg300 IS 'Memory PSI full avg300 percentage';
COMMENT ON COLUMN server_metrics.psi_memory_full_total IS 'Memory PSI full cumulative stall time in microseconds';
COMMENT ON COLUMN server_metrics.psi_io_some_avg10 IS 'IO PSI some avg10 percentage';
COMMENT ON COLUMN server_metrics.psi_io_some_avg60 IS 'IO PSI some avg60 percentage';
COMMENT ON COLUMN server_metrics.psi_io_some_avg300 IS 'IO PSI some avg300 percentage';
COMMENT ON COLUMN server_metrics.psi_io_some_total IS 'IO PSI some cumulative stall time in microseconds';
COMMENT ON COLUMN server_metrics.psi_io_full_avg10 IS 'IO PSI full avg10 percentage';
COMMENT ON COLUMN server_metrics.psi_io_full_avg60 IS 'IO PSI full avg60 percentage';
COMMENT ON COLUMN server_metrics.psi_io_full_avg300 IS 'IO PSI full avg300 percentage';
COMMENT ON COLUMN server_metrics.psi_io_full_total IS 'IO PSI full cumulative stall time in microseconds';

COMMENT ON COLUMN server_current_metrics.psi_cpu_some_avg10 IS 'Current CPU PSI some avg10 percentage';
COMMENT ON COLUMN server_current_metrics.psi_cpu_some_avg60 IS 'Current CPU PSI some avg60 percentage';
COMMENT ON COLUMN server_current_metrics.psi_cpu_some_avg300 IS 'Current CPU PSI some avg300 percentage';
COMMENT ON COLUMN server_current_metrics.psi_cpu_some_total IS 'Current CPU PSI some cumulative stall time in microseconds';
COMMENT ON COLUMN server_current_metrics.psi_cpu_full_avg10 IS 'Current CPU PSI full avg10 percentage';
COMMENT ON COLUMN server_current_metrics.psi_cpu_full_avg60 IS 'Current CPU PSI full avg60 percentage';
COMMENT ON COLUMN server_current_metrics.psi_cpu_full_avg300 IS 'Current CPU PSI full avg300 percentage';
COMMENT ON COLUMN server_current_metrics.psi_cpu_full_total IS 'Current CPU PSI full cumulative stall time in microseconds';
COMMENT ON COLUMN server_current_metrics.psi_memory_some_avg10 IS 'Current memory PSI some avg10 percentage';
COMMENT ON COLUMN server_current_metrics.psi_memory_some_avg60 IS 'Current memory PSI some avg60 percentage';
COMMENT ON COLUMN server_current_metrics.psi_memory_some_avg300 IS 'Current memory PSI some avg300 percentage';
COMMENT ON COLUMN server_current_metrics.psi_memory_some_total IS 'Current memory PSI some cumulative stall time in microseconds';
COMMENT ON COLUMN server_current_metrics.psi_memory_full_avg10 IS 'Current memory PSI full avg10 percentage';
COMMENT ON COLUMN server_current_metrics.psi_memory_full_avg60 IS 'Current memory PSI full avg60 percentage';
COMMENT ON COLUMN server_current_metrics.psi_memory_full_avg300 IS 'Current memory PSI full avg300 percentage';
COMMENT ON COLUMN server_current_metrics.psi_memory_full_total IS 'Current memory PSI full cumulative stall time in microseconds';
COMMENT ON COLUMN server_current_metrics.psi_io_some_avg10 IS 'Current IO PSI some avg10 percentage';
COMMENT ON COLUMN server_current_metrics.psi_io_some_avg60 IS 'Current IO PSI some avg60 percentage';
COMMENT ON COLUMN server_current_metrics.psi_io_some_avg300 IS 'Current IO PSI some avg300 percentage';
COMMENT ON COLUMN server_current_metrics.psi_io_some_total IS 'Current IO PSI some cumulative stall time in microseconds';
COMMENT ON COLUMN server_current_metrics.psi_io_full_avg10 IS 'Current IO PSI full avg10 percentage';
COMMENT ON COLUMN server_current_metrics.psi_io_full_avg60 IS 'Current IO PSI full avg60 percentage';
COMMENT ON COLUMN server_current_metrics.psi_io_full_avg300 IS 'Current IO PSI full avg300 percentage';
COMMENT ON COLUMN server_current_metrics.psi_io_full_total IS 'Current IO PSI full cumulative stall time in microseconds';
