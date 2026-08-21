-- +goose Up

-- SMART and thermal columns are part of the version 1 baseline. Version 4
-- standardized their catalog documentation without changing their shape.

COMMENT ON COLUMN server_metrics.disk_smart IS 'SMART 运行时详情（JSON）';
COMMENT ON COLUMN server_metrics.thermal IS '温度传感器运行时详情（JSON）';
COMMENT ON COLUMN server_current_metrics.disk_smart IS '当前 SMART 运行时详情（JSON）';
COMMENT ON COLUMN server_current_metrics.thermal IS '当前温度传感器运行时详情（JSON）';
