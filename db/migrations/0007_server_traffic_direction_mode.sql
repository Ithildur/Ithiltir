-- +goose Up

-- Per-server traffic direction override. Existing servers keep following the global default.

ALTER TABLE servers
    ADD COLUMN traffic_direction_mode VARCHAR(16) NOT NULL DEFAULT 'default';

UPDATE servers
SET traffic_direction_mode = 'default'
WHERE traffic_direction_mode IS NULL
   OR traffic_direction_mode NOT IN ('default', 'out', 'both', 'max');

ALTER TABLE servers
    DROP CONSTRAINT IF EXISTS chk_servers_traffic_direction_mode;

ALTER TABLE servers
    ADD CONSTRAINT chk_servers_traffic_direction_mode
    CHECK (traffic_direction_mode IN ('default', 'out', 'both', 'max'));

COMMENT ON COLUMN servers.traffic_direction_mode IS '统计方向覆盖；default 表示继承全局设置';
