-- +goose Up

-- Per-server billing cycle mode override. Existing servers keep following the global default.

ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS traffic_cycle_mode VARCHAR(32) NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS traffic_billing_start_day SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS traffic_billing_anchor_date VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS traffic_billing_timezone VARCHAR(64) NOT NULL DEFAULT '';

UPDATE servers
SET traffic_cycle_mode = 'default'
WHERE traffic_cycle_mode IS NULL
   OR traffic_cycle_mode NOT IN ('default', 'calendar_month', 'whmcs_compatible', 'clamp_to_month_end');

UPDATE servers
SET traffic_billing_start_day = 1
WHERE traffic_billing_start_day IS NULL
   OR traffic_billing_start_day < 1
   OR traffic_billing_start_day > 31;

UPDATE servers
SET traffic_billing_anchor_date = ''
WHERE traffic_billing_anchor_date IS NULL;

UPDATE servers
SET traffic_billing_timezone = ''
WHERE traffic_billing_timezone IS NULL;

ALTER TABLE servers
    DROP CONSTRAINT IF EXISTS chk_servers_traffic_cycle_mode,
    DROP CONSTRAINT IF EXISTS chk_servers_traffic_billing_start_day;

ALTER TABLE servers
    ADD CONSTRAINT chk_servers_traffic_cycle_mode
    CHECK (traffic_cycle_mode IN ('default', 'calendar_month', 'whmcs_compatible', 'clamp_to_month_end')),
    ADD CONSTRAINT chk_servers_traffic_billing_start_day
    CHECK (traffic_billing_start_day BETWEEN 1 AND 31);

COMMENT ON COLUMN servers.traffic_cycle_mode IS '账期模式覆盖；default 表示继承全局设置';
COMMENT ON COLUMN servers.traffic_billing_start_day IS '节点账期月度起始日';
COMMENT ON COLUMN servers.traffic_billing_anchor_date IS '节点 WHMCS 兼容账期锚点';
COMMENT ON COLUMN servers.traffic_billing_timezone IS '节点账期时区；空值表示使用应用时区';
