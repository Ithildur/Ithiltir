package migrate

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

var retentionTables = []string{
	"server_metrics",
	"nic_metrics",
	"disk_metrics",
	"disk_physical_metrics",
	"disk_usage_metrics",
	"service_checks",
}

// SyncRetentionPolicies aligns Timescale retention policies with the configured day window.
// It runs at startup/bootstrap only; business paths must not touch database policies.
func SyncRetentionPolicies(ctx context.Context, db *gorm.DB, days, trafficDays int) error {
	if db == nil {
		return fmt.Errorf("sync retention policies: db is nil")
	}
	if days <= 0 {
		return fmt.Errorf("sync retention policies: days must be positive")
	}
	if trafficDays <= 0 {
		return fmt.Errorf("sync retention policies: traffic days must be positive")
	}

	for _, table := range retentionTables {
		if err := syncRetentionPolicy(ctx, db, table, days); err != nil {
			return err
		}
	}
	if err := syncRetentionPolicy(ctx, db, "traffic_5m", trafficDays); err != nil {
		return err
	}

	return nil
}

func syncRetentionPolicy(ctx context.Context, db *gorm.DB, table string, days int) error {
	if err := db.WithContext(ctx).
		Exec("SELECT remove_retention_policy(?::regclass, if_exists => TRUE)", table).
		Error; err != nil {
		return fmt.Errorf("sync retention policies: remove %s: %w", table, err)
	}

	if err := db.WithContext(ctx).
		Exec("SELECT add_retention_policy(?::regclass, (?::int * INTERVAL '1 day'), if_not_exists => TRUE)", table, days).
		Error; err != nil {
		return fmt.Errorf("sync retention policies: add %s: %w", table, err)
	}
	return nil
}
