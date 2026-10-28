-- +goose Up
ALTER TABLE system_settings
    ADD COLUMN pve_guest_visible BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN system_settings.pve_guest_visible IS 'Allow guests to read VM snapshots and history for guest-visible hosts';

ALTER TABLE servers
    ADD COLUMN vm_memory_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN vm_cpu_threshold SMALLINT NOT NULL DEFAULT 80 CHECK (vm_cpu_threshold BETWEEN 1 AND 100),
    ADD COLUMN vm_memory_threshold SMALLINT NOT NULL DEFAULT 90 CHECK (vm_memory_threshold BETWEEN 1 AND 100);

CREATE INDEX idx_alert_notification_outbox_open_predecessor
    ON alert_notification_outbox (event_id, channel_id)
    WHERE transition = 'opened'
      AND status IN ('pending', 'sending', 'retry', 'blocked', 'paused');

INSERT INTO alert_settings (scope, enabled, channel_ids)
VALUES ('global', TRUE, '[]'::jsonb)
ON CONFLICT (scope) DO NOTHING;

ALTER TABLE alert_notification_outbox DROP COLUMN leased_until;
ALTER TABLE alert_control_tasks DROP COLUMN leased_until;

ALTER TABLE notify_channels
    DROP CONSTRAINT chk_notify_channels_config_storage,
    DROP COLUMN config;

DROP INDEX uniq_alert_control_tasks_dedupe;
CREATE UNIQUE INDEX uniq_alert_control_tasks_dedupe
    ON alert_control_tasks (dedupe_key)
    WHERE status = 'pending';

ALTER TABLE traffic_settings
    DROP CONSTRAINT chk_traffic_settings_fixed_cycle,
    DROP CONSTRAINT chk_traffic_settings_cycle,
    DROP CONSTRAINT chk_traffic_settings_billing_start_day,
    DROP COLUMN cycle_mode,
    DROP COLUMN billing_start_day,
    DROP COLUMN billing_anchor_date,
    DROP COLUMN billing_timezone;

-- +goose Down
ALTER TABLE notify_channels
    ADD COLUMN config JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD CONSTRAINT chk_notify_channels_config_storage
        CHECK (config = '{}'::jsonb AND config_sealed IS NOT NULL);
ALTER TABLE notify_channels ALTER COLUMN config DROP DEFAULT;
COMMENT ON COLUMN notify_channels.config IS '兼容占位；通知渠道配置明文不得持久化';

-- Keep the singleton row: it may contain settings saved after this upgrade.
ALTER TABLE traffic_settings
    ADD COLUMN cycle_mode VARCHAR(32) NOT NULL DEFAULT 'calendar_month',
    ADD COLUMN billing_start_day SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN billing_anchor_date VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN billing_timezone VARCHAR(64) NOT NULL DEFAULT '',
    ADD CONSTRAINT chk_traffic_settings_cycle
        CHECK (cycle_mode IN ('calendar_month', 'whmcs_compatible', 'clamp_to_month_end')),
    ADD CONSTRAINT chk_traffic_settings_billing_start_day
        CHECK (billing_start_day BETWEEN 1 AND 31),
    ADD CONSTRAINT chk_traffic_settings_fixed_cycle
        CHECK (cycle_mode = 'calendar_month' AND billing_start_day = 1
               AND billing_anchor_date = '' AND billing_timezone = '');

COMMENT ON COLUMN traffic_settings.cycle_mode IS '兼容只读字段；固定为 calendar_month';
COMMENT ON COLUMN traffic_settings.billing_start_day IS '兼容只读字段；固定为 1';
COMMENT ON COLUMN traffic_settings.billing_anchor_date IS '兼容只读字段；固定为空';
COMMENT ON COLUMN traffic_settings.billing_timezone IS '兼容只读字段；固定为空并使用应用时区';

ALTER TABLE alert_control_tasks ADD COLUMN leased_until TIMESTAMPTZ;
ALTER TABLE alert_notification_outbox ADD COLUMN leased_until TIMESTAMPTZ;
COMMENT ON COLUMN alert_control_tasks.leased_until IS '历史 schema 兼容字段；当前单实例 worker 不使用租约';
COMMENT ON COLUMN alert_notification_outbox.leased_until IS '历史 schema 兼容字段；当前 worker 不按租约截止时间取件';

DROP INDEX uniq_alert_control_tasks_dedupe;
CREATE UNIQUE INDEX uniq_alert_control_tasks_dedupe
    ON alert_control_tasks (dedupe_key)
    WHERE status IN ('pending', 'leased');

DROP INDEX idx_alert_notification_outbox_open_predecessor;
ALTER TABLE servers DROP COLUMN vm_cpu_threshold, DROP COLUMN vm_memory_threshold, DROP COLUMN vm_memory_enabled;
ALTER TABLE system_settings DROP COLUMN pve_guest_visible;
