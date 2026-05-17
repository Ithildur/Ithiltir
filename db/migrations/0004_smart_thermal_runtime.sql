-- +goose Up

-- Store SMART and thermal runtime details with historical and current metrics.

ALTER TABLE server_metrics
    ADD COLUMN IF NOT EXISTS disk_smart JSONB,
    ADD COLUMN IF NOT EXISTS thermal JSONB;

ALTER TABLE server_current_metrics
    ADD COLUMN IF NOT EXISTS disk_smart JSONB,
    ADD COLUMN IF NOT EXISTS thermal JSONB;

COMMENT ON COLUMN server_metrics.disk_smart IS 'SMART 运行时详情（JSON）';
COMMENT ON COLUMN server_metrics.thermal IS '温度传感器运行时详情（JSON）';
COMMENT ON COLUMN server_current_metrics.disk_smart IS '当前 SMART 运行时详情（JSON）';
COMMENT ON COLUMN server_current_metrics.thermal IS '当前温度传感器运行时详情（JSON）';
