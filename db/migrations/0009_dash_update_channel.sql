-- +goose Up

ALTER TABLE system_settings
    ADD COLUMN IF NOT EXISTS dash_update_channel VARCHAR(16) NOT NULL DEFAULT 'release',
    ADD COLUMN IF NOT EXISTS dash_update_mode VARCHAR(16) NOT NULL DEFAULT 'manual';

UPDATE system_settings
SET dash_update_channel = 'release'
WHERE dash_update_channel IS NULL
   OR dash_update_channel NOT IN ('release', 'prerelease');

UPDATE system_settings
SET dash_update_mode = 'manual'
WHERE dash_update_mode IS NULL
   OR dash_update_mode NOT IN ('manual', 'notify', 'auto');

ALTER TABLE system_settings
    ALTER COLUMN dash_update_channel SET DEFAULT 'release',
    ALTER COLUMN dash_update_channel SET NOT NULL,
    ALTER COLUMN dash_update_mode SET DEFAULT 'manual',
    ALTER COLUMN dash_update_mode SET NOT NULL;

ALTER TABLE system_settings
    DROP CONSTRAINT IF EXISTS chk_system_settings_dash_update_channel,
    DROP CONSTRAINT IF EXISTS chk_system_settings_dash_update_mode;

ALTER TABLE system_settings
    ADD CONSTRAINT chk_system_settings_dash_update_channel
    CHECK (dash_update_channel IN ('release', 'prerelease')),
    ADD CONSTRAINT chk_system_settings_dash_update_mode
    CHECK (dash_update_mode IN ('manual', 'notify', 'auto'));

COMMENT ON COLUMN system_settings.dash_update_channel IS 'Dash update channel: release or prerelease';
COMMENT ON COLUMN system_settings.dash_update_mode IS 'Dash update automation mode: manual, notify, or auto';
