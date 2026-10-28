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

-- +goose Down
DROP INDEX idx_alert_notification_outbox_open_predecessor;
ALTER TABLE servers DROP COLUMN vm_cpu_threshold, DROP COLUMN vm_memory_threshold, DROP COLUMN vm_memory_enabled;
ALTER TABLE system_settings DROP COLUMN pve_guest_visible;
