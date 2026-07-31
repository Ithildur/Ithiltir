package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"dash/internal/model"
	"dash/internal/notify"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const notifyConfigConstraint = "chk_notify_channels_config_storage"

type notifyConfigRow struct {
	ID           int64            `gorm:"column:id"`
	Type         model.NotifyType `gorm:"column:type"`
	Config       datatypes.JSON   `gorm:"column:config"`
	ConfigSealed []byte           `gorm:"column:config_sealed"`
}

func (notifyConfigRow) TableName() string {
	return "notify_channels"
}

type legacyNotifyConfig struct {
	id     int64
	typ    model.NotifyType
	config []byte
	sealed []byte
}

// sealNotifyConfigs is the transactional body of the Go migration that turns
// legacy plaintext JSON into encrypted channel configuration. Goose records
// the migration version in the same transaction only after this returns.
func sealNotifyConfigs(ctx context.Context, tx *sql.Tx, keyPath string) error {
	if tx == nil {
		return errors.New("seal notification configs: tx is nil")
	}

	var sealedCount int64
	if err := tx.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM notify_channels WHERE config_sealed IS NOT NULL",
	).Scan(&sealedCount); err != nil {
		return fmt.Errorf("count encrypted notification configs: %w", err)
	}
	configCipher, err := loadOrCreateNotifyConfigCipher(keyPath, sealedCount)
	if err != nil {
		return err
	}

	queryRows, err := tx.QueryContext(ctx, `
		SELECT id, type::text, config::text, config_sealed
		FROM notify_channels
		ORDER BY id ASC
		FOR UPDATE
	`)
	if err != nil {
		return fmt.Errorf("load notification configs for migration: %w", err)
	}
	var rows []legacyNotifyConfig
	for queryRows.Next() {
		var row legacyNotifyConfig
		var typ string
		var config string
		if err := queryRows.Scan(&row.id, &typ, &config, &row.sealed); err != nil {
			_ = queryRows.Close()
			return fmt.Errorf("scan notification config for migration: %w", err)
		}
		row.typ = model.NotifyType(typ)
		row.config = []byte(config)
		rows = append(rows, row)
	}
	if err := queryRows.Err(); err != nil {
		_ = queryRows.Close()
		return fmt.Errorf("read notification configs for migration: %w", err)
	}
	if err := queryRows.Close(); err != nil {
		return fmt.Errorf("close notification config rows: %w", err)
	}

	for i := range rows {
		row := &rows[i]
		if len(row.sealed) != 0 {
			plain, err := configCipher.Open(row.id, row.typ, row.sealed)
			if err != nil {
				return fmt.Errorf("verify encrypted notification config %d: %w", row.id, err)
			}
			clear(plain)
			continue
		}
		payload, err := configCipher.Seal(row.id, row.typ, row.config)
		if err != nil {
			return fmt.Errorf("encrypt notification config %d: %w", row.id, err)
		}
		row.sealed = payload
	}

	if len(rows) != 0 {
		if _, err := tx.ExecContext(
			ctx,
			"ALTER TABLE notify_channels DISABLE TRIGGER notify_channels_updated_at",
		); err != nil {
			return fmt.Errorf("disable notification channel update trigger: %w", err)
		}
		for i := range rows {
			row := &rows[i]
			if _, err := tx.ExecContext(
				ctx,
				"UPDATE notify_channels SET config = '{}'::jsonb, config_sealed = $1 WHERE id = $2",
				row.sealed,
				row.id,
			); err != nil {
				return fmt.Errorf("store encrypted notification config %d: %w", row.id, err)
			}
			clear(row.config)
		}
		if _, err := tx.ExecContext(
			ctx,
			"ALTER TABLE notify_channels ENABLE TRIGGER notify_channels_updated_at",
		); err != nil {
			return fmt.Errorf("enable notification channel update trigger: %w", err)
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		"ALTER TABLE notify_channels VALIDATE CONSTRAINT "+notifyConfigConstraint,
	); err != nil {
		return fmt.Errorf("validate encrypted notification config constraint: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		ALTER TABLE notify_channels
		ALTER COLUMN config_sealed SET NOT NULL
	`); err != nil {
		return fmt.Errorf("require encrypted notification configs: %w", err)
	}
	return nil
}

// CheckNotifyConfigs enforces the post-migration storage invariant and verifies
// that the installed key can decrypt every channel before the server starts.
func CheckNotifyConfigs(ctx context.Context, db *gorm.DB, configCipher *notify.ConfigCipher) error {
	if db == nil {
		return errors.New("check notification configs: db is nil")
	}
	var nullable string
	if err := db.WithContext(ctx).Raw(`
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'notify_channels'
		  AND column_name = 'config_sealed'
	`).Scan(&nullable).Error; err != nil {
		return fmt.Errorf("inspect notification config column: %w", err)
	}
	if nullable != "NO" {
		return errors.New("notification config encryption requires dash migrate")
	}
	var constraintValidated bool
	if err := db.WithContext(ctx).Raw(`
		SELECT convalidated
		FROM pg_constraint
		WHERE conrelid = 'notify_channels'::regclass
		  AND conname = ?
	`, notifyConfigConstraint).Scan(&constraintValidated).Error; err != nil {
		return fmt.Errorf("inspect notification config constraint: %w", err)
	}
	if !constraintValidated {
		return errors.New("notification config storage constraint requires dash migrate")
	}
	var unsafeCount int64
	if err := db.WithContext(ctx).
		Model(&notifyConfigRow{}).
		Where("config_sealed IS NULL OR config <> '{}'::jsonb").
		Count(&unsafeCount).Error; err != nil {
		return fmt.Errorf("inspect notification config storage: %w", err)
	}
	if unsafeCount != 0 {
		return fmt.Errorf("%d notification channel configs require dash migrate", unsafeCount)
	}

	var rows []notifyConfigRow
	if err := db.WithContext(ctx).
		Select("id", "type", "config_sealed").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("load encrypted notification configs: %w", err)
	}
	for _, row := range rows {
		plain, err := configCipher.Open(row.ID, row.Type, row.ConfigSealed)
		if err != nil {
			return fmt.Errorf("decrypt notification config %d: %w", row.ID, err)
		}
		clear(plain)
	}
	return nil
}

func loadOrCreateNotifyConfigCipher(keyPath string, sealedCount int64) (*notify.ConfigCipher, error) {
	configCipher, err := notify.LoadConfigCipher(keyPath)
	if err == nil {
		return configCipher, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if sealedCount != 0 {
		return nil, fmt.Errorf(
			"notification config key is missing while %d encrypted configs exist; restore the key from backup",
			sealedCount,
		)
	}
	if err := notify.CreateConfigKey(keyPath); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	return notify.LoadConfigCipher(keyPath)
}
