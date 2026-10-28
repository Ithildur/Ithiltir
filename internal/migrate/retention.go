package migrate

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

var metricsRawTables = []string{
	"server_metrics",
	"disk_metrics",
	"disk_physical_metrics",
	"disk_usage_metrics",
}

var retentionTables = []string{
	"nic_metrics",
	"service_checks",
}

// SyncRetentionPolicies aligns Timescale retention policies with the configured day window.
// It runs at startup/bootstrap only; business paths must not touch database policies.
func SyncRetentionPolicies(ctx context.Context, db *gorm.DB, metricsDays, days, trafficDays int) error {
	if db == nil {
		return fmt.Errorf("sync retention policies: db is nil")
	}
	if metricsDays < 2 {
		return fmt.Errorf("sync retention policies: metrics days must be at least 2")
	}
	if days <= 0 {
		return fmt.Errorf("sync retention policies: days must be positive")
	}
	if trafficDays <= 0 {
		return fmt.Errorf("sync retention policies: traffic days must be positive")
	}

	for _, table := range metricsRawTables {
		if err := syncRetentionPolicy(ctx, db, table, metricsDays); err != nil {
			return err
		}
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
	var policy struct {
		JobID   int
		Matches bool
	}
	if err := db.WithContext(ctx).Raw(`
		SELECT job_id, (config->>'drop_after')::interval = (?::int * INTERVAL '1 day') AS matches
		FROM timescaledb_information.jobs
		WHERE hypertable_schema = current_schema() AND hypertable_name = ?
		  AND proc_name = 'policy_retention'
	`, days, table).Scan(&policy).Error; err != nil {
		return fmt.Errorf("sync retention policies: inspect %s: %w", table, err)
	}
	if policy.Matches {
		return nil
	}
	if policy.JobID == 0 {
		if err := db.WithContext(ctx).
			Exec("SELECT add_retention_policy(?::regclass, (?::int * INTERVAL '1 day'))", table, days).
			Error; err != nil {
			return fmt.Errorf("sync retention policies: add %s: %w", table, err)
		}
		return nil
	}

	if err := db.WithContext(ctx).Exec(`
		SELECT alter_job(job_id, config => config || jsonb_build_object('drop_after', ?::int * INTERVAL '1 day'))
		FROM timescaledb_information.jobs WHERE job_id = ?
	`, days, policy.JobID).Error; err != nil {
		return fmt.Errorf("sync retention policies: update %s: %w", table, err)
	}
	return nil
}
