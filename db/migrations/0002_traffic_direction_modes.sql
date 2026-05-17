-- +goose Up

-- Replace legacy traffic direction modes with billing-oriented modes.

ALTER TABLE traffic_settings
    DROP CONSTRAINT IF EXISTS chk_traffic_settings_direction;

UPDATE traffic_settings
SET direction_mode = CASE direction_mode
    WHEN 'split' THEN 'both'
    WHEN 'in' THEN 'max'
    WHEN 'both' THEN 'both'
    WHEN 'max' THEN 'max'
    ELSE 'out'
END
WHERE direction_mode NOT IN ('out', 'both', 'max');

ALTER TABLE traffic_settings
    ADD CONSTRAINT chk_traffic_settings_direction
    CHECK (direction_mode IN ('out', 'both', 'max'));

ALTER TABLE traffic_month_usage
    ADD COLUMN IF NOT EXISTS both_peak_bytes_per_sec DOUBLE PRECISION NOT NULL DEFAULT 0;

COMMENT ON COLUMN traffic_month_usage.both_peak_bytes_per_sec IS '入出合计估算峰值速率（B/s）';

ALTER TABLE traffic_monthly
    ADD COLUMN IF NOT EXISTS both_p95_bytes_per_sec DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS both_peak_bytes_per_sec DOUBLE PRECISION NOT NULL DEFAULT 0;

COMMENT ON COLUMN traffic_monthly.both_p95_bytes_per_sec IS '入出合计95带宽（B/s）';
COMMENT ON COLUMN traffic_monthly.both_peak_bytes_per_sec IS '入出合计峰值速率（B/s）';
