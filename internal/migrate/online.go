package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SyncOnlineJob shares the configured offline threshold with the database sampler.
// It runs at startup/bootstrap; TimescaleDB owns the sampling schedule.
func SyncOnlineJob(ctx context.Context, db *gorm.DB, offlineAfter time.Duration, loc *time.Location) error {
	if offlineAfter <= 0 {
		return fmt.Errorf("sync online job: offline threshold must be positive")
	}
	zone, err := onlineTimezone(loc)
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", advisoryLockID).Error; err != nil {
			return err
		}
		var jobID int
		if err := tx.Raw(`
		SELECT job_id FROM timescaledb_information.jobs
		WHERE proc_schema = current_schema() AND proc_name = 'sample_node_online'
		  AND to_regprocedure('configure_node_online_1h(text)') IS NOT NULL
		  AND EXISTS (SELECT 1 FROM information_schema.columns
		      WHERE table_schema = current_schema() AND table_name = 'node_online' AND column_name = 'observed_ms')
	`).Scan(&jobID).Error; err != nil {
			return fmt.Errorf("sync online job: %w", err)
		}
		if jobID == 0 {
			return fmt.Errorf("sync online job: uptime duration schema is missing; initialize uptime storage using db/migrations/0014_uptime.sql")
		}
		if err := tx.Exec("CALL configure_node_online_1h(?)", zone).Error; err != nil {
			return fmt.Errorf("configure hourly uptime: %w", err)
		}
		if err := tx.Exec(`
		SELECT alter_job(job_id, config => config || jsonb_build_object('offline_after_seconds', ?::double precision))
		FROM timescaledb_information.jobs
		WHERE job_id = ?
		  AND (config->>'offline_after_seconds')::double precision IS DISTINCT FROM ?::double precision
	`, offlineAfter.Seconds(), jobID, offlineAfter.Seconds()).Error; err != nil {
			return fmt.Errorf("sync online job: configure threshold: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	// Prime a new/rebuilt aggregate once, so the first API request does not scan
	// the entire minute history while waiting for the next hourly refresh.
	var refreshID int
	if err := db.WithContext(ctx).Raw(`
		SELECT j.job_id FROM timescaledb_information.jobs j
		LEFT JOIN timescaledb_information.job_stats s ON s.job_id = j.job_id
		WHERE j.hypertable_schema = current_schema() AND j.hypertable_name = 'node_online_1h'
		 AND j.proc_name = 'policy_refresh_continuous_aggregate'
		 AND COALESCE(s.total_successes, 0) = 0
	`).Scan(&refreshID).Error; err != nil {
		return fmt.Errorf("load hourly refresh job: %w", err)
	}
	if refreshID != 0 {
		if err := db.WithContext(ctx).Exec("CALL run_job(?)", refreshID).Error; err != nil {
			return fmt.Errorf("initialize hourly uptime: %w", err)
		}
	}
	return nil
}

// PostgreSQL needs an actual timezone name rather than Go's symbolic "Local".
func onlineTimezone(loc *time.Location) (string, error) {
	if loc.String() != "Local" {
		return loc.String(), nil
	}
	name, set := os.LookupEnv("TZ")
	if set && name == "" {
		return "UTC", nil
	}
	if set {
		name = strings.TrimPrefix(name, ":")
		if !filepath.IsAbs(name) {
			return name, nil
		}
	} else {
		name = "/etc/localtime"
	}
	if resolved, err := filepath.EvalSymlinks(name); err == nil {
		if _, zone, ok := strings.Cut(resolved, "/zoneinfo/"); ok {
			return zone, nil
		}
	}
	if !set {
		if name, err := os.ReadFile("/etc/timezone"); err == nil && strings.TrimSpace(string(name)) != "" {
			return strings.TrimSpace(string(name)), nil
		}
	}
	return "", fmt.Errorf("resolve uptime timezone: set app.timezone to an IANA timezone name")
}
