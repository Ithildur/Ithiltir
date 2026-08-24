package migrate

import (
	"context"
	"errors"
	"fmt"

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
