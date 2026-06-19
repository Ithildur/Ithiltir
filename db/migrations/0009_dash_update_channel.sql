-- +goose Up

ALTER TABLE system_settings
    ADD COLUMN IF NOT EXISTS dash_update_channel VARCHAR(16) NOT NULL DEFAULT 'release';

UPDATE system_settings
SET dash_update_channel = 'release'
WHERE dash_update_channel IS NULL
   OR dash_update_channel NOT IN ('release', 'prerelease');

ALTER TABLE system_settings
    DROP CONSTRAINT IF EXISTS chk_system_settings_dash_update_channel;

ALTER TABLE system_settings
    ADD CONSTRAINT chk_system_settings_dash_update_channel
    CHECK (dash_update_channel IN ('release', 'prerelease'));

COMMENT ON COLUMN system_settings.dash_update_channel IS 'Dash update channel: release or prerelease';
