package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
)

type historySchemaState uint8

const metricsHistoryVersion int64 = 13

const (
	historySchemaMixed historySchemaState = iota
	historySchemaOld
	historySchemaDDLReady
)

var (
	historyDetachPoliciesSQL = readSQL("0013_metrics_history/detach_policies.sql")
	historyDDLSQL            = readSQL("0013_metrics_history/ddl.sql")
	historyPoliciesSQL       = readSQL("0013_metrics_history/policies.sql")
)

func metricsHistory() *goose.Migration {
	return goose.NewGoMigration(
		metricsHistoryVersion,
		&goose.GoFunc{RunDB: migrateHistory},
		nil,
	)
}

func migrateHistory(ctx context.Context, db *sql.DB) error {
	state, err := inspectHistorySchema(ctx, db)
	if err != nil {
		return err
	}
	switch state {
	case historySchemaOld:
		if err := detachHistoryPolicies(ctx, db); err != nil {
			return err
		}
		if err := runHistoryTx(ctx, db, "apply metrics history DDL", historyDDLSQL); err != nil {
			return err
		}
		state, err = inspectHistorySchema(ctx, db)
		if err != nil {
			return err
		}
		if state != historySchemaDDLReady {
			return fmt.Errorf("metrics history migration: DDL committed without producing the expected schema")
		}
	case historySchemaDDLReady:
		// A previous attempt completed the atomic DDL phase and stopped during
		// the serial backfill. Repeating refreshes is safe.
	default:
		return fmt.Errorf("metrics history migration: schema is neither the version 12 layout nor the complete version 13 DDL layout")
	}

	if err := backfillHistory(ctx, db, time.Now().UTC()); err != nil {
		return err
	}
	return runHistoryTx(ctx, db, "apply metrics history policies", historyPoliciesSQL)
}

const historyJobFilter = `
job.hypertable_schema = current_schema()
  AND (
      (
          job.proc_name = 'policy_refresh_continuous_aggregate'
          AND job.hypertable_name IN (
              'server_metrics_15m',
              'server_metrics_1h',
              'server_online_30m',
              'disk_metrics_15m',
              'disk_metrics_1h',
              'disk_usage_metrics_15m',
              'disk_usage_metrics_1h'
          )
      )
      OR (
          job.proc_name IN ('policy_compression', 'policy_retention')
          AND job.hypertable_name IN (
              'server_metrics',
              'disk_metrics',
              'disk_usage_metrics',
              'disk_physical_metrics'
          )
      )
  )`

const pauseHistoryJobsSQL = `
SELECT alter_job(job.job_id, scheduled => FALSE)
FROM timescaledb_information.jobs job
WHERE ` + historyJobFilter

const runningHistoryJobsSQL = `
SELECT count(*)
FROM timescaledb_information.jobs job
JOIN timescaledb_information.job_stats stats USING (job_id)
WHERE stats.job_status = 'Running'
  AND ` + historyJobFilter

func detachHistoryPolicies(ctx context.Context, db *sql.DB) error {
	if err := runHistoryTx(ctx, db, "pause metrics history jobs", pauseHistoryJobsSQL); err != nil {
		return err
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		if err := db.QueryRowContext(ctx, runningHistoryJobsSQL).Scan(&count); err != nil {
			return fmt.Errorf("metrics history migration: inspect running jobs: %w", err)
		}
		if count == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("metrics history migration: wait for running jobs: %w", ctx.Err())
		case <-ticker.C:
		}
	}

	return runHistoryTx(ctx, db, "detach metrics history policies", historyDetachPoliciesSQL)
}

func runHistoryTx(ctx context.Context, db *sql.DB, action, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("metrics history migration: begin %s: %w", action, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("metrics history migration: %s: %w", action, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("metrics history migration: commit %s: %w", action, err)
	}
	return nil
}

func inspectHistorySchema(ctx context.Context, db *sql.DB) (historySchemaState, error) {
	const query = `
WITH base_views(name) AS (
    VALUES
        ('server_metrics_15m'),
        ('server_metrics_1h'),
        ('server_online_30m'),
        ('disk_metrics_15m'),
        ('disk_metrics_1h'),
        ('disk_usage_metrics_15m'),
        ('disk_usage_metrics_1h')
), physical_views(name) AS (
    VALUES
        ('disk_physical_metrics_15m'),
        ('disk_physical_metrics_1h')
), signatures(table_name, column_name) AS (
    VALUES
        ('server_metrics_15m', 'cpu_temp_c_count'),
        ('server_metrics_15m', 'psi_io_full_avg300_count'),
        ('server_metrics_1h', 'cpu_temp_c_avg'),
        ('disk_metrics_15m', 'read_bps_count'),
        ('disk_usage_metrics_15m', 'used_bytes_count'),
        ('disk_physical_metrics_15m', 'temp_c_count'),
        ('disk_physical_metrics_1h', 'temp_c_avg')
)
SELECT
    (SELECT count(*) FROM base_views
     WHERE to_regclass(format('%I.%I', current_schema(), name)) IS NOT NULL),
    (SELECT count(*) FROM physical_views
     WHERE to_regclass(format('%I.%I', current_schema(), name)) IS NOT NULL),
    (SELECT count(*) FROM signatures signature
     WHERE EXISTS (
         SELECT 1
         FROM information_schema.columns column_info
         WHERE column_info.table_schema = current_schema()
           AND column_info.table_name = signature.table_name
           AND column_info.column_name = signature.column_name
     ))`

	var baseViews, physicalViews, signatures int
	if err := db.QueryRowContext(ctx, query).Scan(&baseViews, &physicalViews, &signatures); err != nil {
		return historySchemaMixed, fmt.Errorf("metrics history migration: inspect schema: %w", err)
	}
	switch {
	case baseViews == 7 && physicalViews == 0 && signatures == 0:
		return historySchemaOld, nil
	case baseViews == 7 && physicalViews == 2 && signatures == 7:
		return historySchemaDDLReady, nil
	default:
		return historySchemaMixed, nil
	}
}

func backfillHistory(ctx context.Context, db *sql.DB, now time.Time) error {
	type group struct {
		views    []string
		bucket   time.Duration
		lookback time.Duration
	}
	groups := []group{
		{
			views: []string{
				"server_metrics_15m",
				"disk_metrics_15m",
				"disk_usage_metrics_15m",
				"disk_physical_metrics_15m",
			},
			bucket:   15 * time.Minute,
			lookback: 16 * 24 * time.Hour,
		},
		{
			views: []string{
				"server_metrics_1h",
				"disk_metrics_1h",
				"disk_usage_metrics_1h",
				"disk_physical_metrics_1h",
			},
			bucket:   time.Hour,
			lookback: 31 * 24 * time.Hour,
		},
	}

	now = now.UTC()
	for _, group := range groups {
		end := now.Truncate(group.bucket)
		start := end.Add(-group.lookback)
		for _, view := range group.views {
			if _, err := db.ExecContext(
				ctx,
				"CALL refresh_continuous_aggregate($1::regclass, $2, $3)",
				view,
				start,
				end,
			); err != nil {
				return fmt.Errorf("metrics history migration: backfill %s: %w", view, err)
			}
		}
	}
	return nil
}
