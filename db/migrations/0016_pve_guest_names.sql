-- +goose Up
ALTER TABLE system_settings
    ADD COLUMN pve_guest_names_visible BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN system_settings.pve_guest_names_visible IS 'Expose VM names in guest-visible PVE snapshots';

-- +goose Down
ALTER TABLE system_settings DROP COLUMN pve_guest_names_visible;
