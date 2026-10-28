package migrate

import (
	"context"
	"errors"
	"fmt"

	"dash/internal/model"
	"dash/internal/notify"

	"gorm.io/gorm"
)

type notifyConfigRow struct {
	ID           int64            `gorm:"column:id"`
	Type         model.NotifyType `gorm:"column:type"`
	ConfigSealed []byte           `gorm:"column:config_sealed"`
}

func (notifyConfigRow) TableName() string {
	return "notify_channels"
}

// CheckNotifyConfigs verifies that the installed key can decrypt every channel
// after the application has checked the schema version.
func CheckNotifyConfigs(ctx context.Context, db *gorm.DB, configCipher *notify.ConfigCipher) error {
	if db == nil {
		return errors.New("check notification configs: db is nil")
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
