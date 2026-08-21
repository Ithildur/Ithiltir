package migrate_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"dash/internal/migrate"
	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationNotificationMigrationRemovesDeletedChannelRefs(t *testing.T) {
	ctx := context.Background()
	db, keyPath := pgtest.NewDBAt(t, 10)

	var activeID int64
	if err := db.Raw(`
		INSERT INTO notify_channels (name, type, config, enabled, is_deleted)
		VALUES ('active', 'webhook', '{}'::jsonb, TRUE, FALSE)
		RETURNING id
	`).Scan(&activeID).Error; err != nil {
		t.Fatalf("create active notification channel: %v", err)
	}
	var deletedID int64
	if err := db.Raw(`
		INSERT INTO notify_channels (name, type, config, enabled, is_deleted)
		VALUES ('deleted', 'webhook', '{}'::jsonb, TRUE, TRUE)
		RETURNING id
	`).Scan(&deletedID).Error; err != nil {
		t.Fatalf("create deleted notification channel: %v", err)
	}
	missingID := deletedID + 1_000_000
	if err := db.Exec(`
		INSERT INTO alert_settings (scope, enabled, channel_ids)
		VALUES ('global', TRUE, jsonb_build_array(?::bigint, ?::bigint, ?::bigint))
		ON CONFLICT (scope) DO UPDATE SET channel_ids = EXCLUDED.channel_ids
	`, activeID, deletedID, missingID).Error; err != nil {
		t.Fatalf("create legacy alert settings: %v", err)
	}
	var before string
	if err := db.Raw("SELECT channel_ids::text FROM alert_settings WHERE scope = 'global'").Scan(&before).Error; err != nil {
		t.Fatalf("load legacy alert settings: %v", err)
	}
	var activeRefs int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM alert_settings AS settings
		CROSS JOIN LATERAL jsonb_array_elements(settings.channel_ids) AS ref(value)
		JOIN notify_channels AS channel
		  ON to_jsonb(channel.id) = ref.value
		 AND channel.is_deleted = FALSE
		WHERE settings.scope = 'global'
	`).Scan(&activeRefs).Error; err != nil {
		t.Fatalf("count legacy active channel refs: %v", err)
	}
	if activeRefs != 1 {
		t.Fatalf("legacy channel ids = %s with %d active refs, want one", before, activeRefs)
	}
	result, err := migrate.Run(ctx, db, keyPath)
	if err != nil {
		t.Fatalf("rerun notification migration: %v", err)
	}
	if result.Applied != 2 {
		t.Fatalf("applied migrations = %d, want 2", result.Applied)
	}

	var raw string
	if err := db.Raw("SELECT channel_ids::text FROM alert_settings WHERE scope = 'global'").Scan(&raw).Error; err != nil {
		t.Fatalf("load migrated alert settings: %v", err)
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		t.Fatalf("decode migrated channel ids: %v", err)
	}
	if len(ids) != 1 || ids[0] != activeID {
		t.Fatalf("migrated channel ids = %v, want [%d]", ids, activeID)
	}
}

func TestIntegrationTrafficCycleMigrationPreservesEffectiveNodeCycles(t *testing.T) {
	ctx := context.Background()
	db, keyPath := pgtest.NewDBAt(t, 10)

	if err := db.Exec(`
		ALTER TABLE traffic_settings
		    DROP CONSTRAINT IF EXISTS chk_traffic_settings_fixed_cycle;
		ALTER TABLE servers
		    DROP CONSTRAINT IF EXISTS chk_servers_traffic_cycle_mode,
		    ALTER COLUMN traffic_cycle_mode SET DEFAULT 'default';
		ALTER TABLE servers
		    ADD CONSTRAINT chk_servers_traffic_cycle_mode
		    CHECK (traffic_cycle_mode IN ('default', 'calendar_month', 'whmcs_compatible', 'clamp_to_month_end'));

		UPDATE traffic_settings
		SET cycle_mode = 'clamp_to_month_end',
		    billing_start_day = 20,
		    billing_anchor_date = '',
		    billing_timezone = 'UTC'
		WHERE id = 1;

		INSERT INTO servers (
		    name,
		    hostname,
		    secret,
		    traffic_cycle_mode,
		    traffic_billing_start_day,
		    traffic_billing_anchor_date,
		    traffic_billing_timezone
		)
		VALUES
		    ('legacy-default', 'legacy-default', 'legacy-default-secret', 'default', 1, '', ''),
		    ('explicit', 'explicit', 'explicit-secret', 'whmcs_compatible', 30, '2026-01-30', 'Asia/Hong_Kong');
	`).Error; err != nil {
		t.Fatalf("seed legacy traffic cycles: %v", err)
	}
	result, err := migrate.Run(ctx, db, keyPath)
	if err != nil {
		t.Fatalf("rerun traffic migration: %v", err)
	}
	if result.Applied != 2 {
		t.Fatalf("applied migrations = %d, want 2", result.Applied)
	}

	type cycleRow struct {
		Mode     string `gorm:"column:traffic_cycle_mode"`
		Day      int    `gorm:"column:traffic_billing_start_day"`
		Anchor   string `gorm:"column:traffic_billing_anchor_date"`
		Timezone string `gorm:"column:traffic_billing_timezone"`
	}
	var legacy cycleRow
	if err := db.Raw(`
		SELECT traffic_cycle_mode,
		       traffic_billing_start_day,
		       traffic_billing_anchor_date,
		       traffic_billing_timezone
		FROM servers
		WHERE secret = 'legacy-default-secret'
	`).Scan(&legacy).Error; err != nil {
		t.Fatalf("load migrated legacy cycle: %v", err)
	}
	if legacy.Mode != "clamp_to_month_end" || legacy.Day != 20 || legacy.Anchor != "" || legacy.Timezone != "UTC" {
		t.Fatalf("legacy cycle = %#v, want inherited clamp day 20 UTC", legacy)
	}

	var explicit cycleRow
	if err := db.Raw(`
		SELECT traffic_cycle_mode,
		       traffic_billing_start_day,
		       traffic_billing_anchor_date,
		       traffic_billing_timezone
		FROM servers
		WHERE secret = 'explicit-secret'
	`).Scan(&explicit).Error; err != nil {
		t.Fatalf("load explicit cycle: %v", err)
	}
	if explicit.Mode != "whmcs_compatible" || explicit.Day != 30 ||
		explicit.Anchor != "2026-01-30" || explicit.Timezone != "Asia/Hong_Kong" {
		t.Fatalf("explicit cycle = %#v, want unchanged WHMCS cycle", explicit)
	}

	var global struct {
		Mode     string `gorm:"column:cycle_mode"`
		Day      int    `gorm:"column:billing_start_day"`
		Anchor   string `gorm:"column:billing_anchor_date"`
		Timezone string `gorm:"column:billing_timezone"`
	}
	if err := db.Raw(`
		SELECT cycle_mode, billing_start_day, billing_anchor_date, billing_timezone
		FROM traffic_settings
		WHERE id = 1
	`).Scan(&global).Error; err != nil {
		t.Fatalf("load migrated global cycle: %v", err)
	}
	if global.Mode != "calendar_month" || global.Day != 1 || global.Anchor != "" || global.Timezone != "" {
		t.Fatalf("global compatibility cycle = %#v, want fixed calendar month", global)
	}

	var created cycleRow
	if err := db.Raw(`
		INSERT INTO servers (name, hostname, secret)
		VALUES ('new-default', 'new-default', 'new-default-secret')
		RETURNING traffic_cycle_mode,
		          traffic_billing_start_day,
		          traffic_billing_anchor_date,
		          traffic_billing_timezone
	`).Scan(&created).Error; err != nil {
		t.Fatalf("create node with new default cycle: %v", err)
	}
	if created.Mode != "calendar_month" || created.Day != 1 || created.Anchor != "" || created.Timezone != "" {
		t.Fatalf("new node cycle = %#v, want explicit calendar month", created)
	}

	var repairCount int64
	if err := db.Raw("SELECT COUNT(*) FROM traffic_usage_repairs").Scan(&repairCount).Error; err != nil {
		t.Fatalf("count migration repairs: %v", err)
	}
	if repairCount != 0 {
		t.Fatalf("migration repair rows = %d, want 0", repairCount)
	}
}

func TestIntegrationReleaseMigrationWidensNaturalObservationFields(t *testing.T) {
	ctx := context.Background()
	db, keyPath := pgtest.NewDBAt(t, 10)

	if err := db.Exec(`
		INSERT INTO servers (name, hostname, secret)
		VALUES ('legacy-storage', 'legacy-storage', 'legacy-storage-secret');

		INSERT INTO disk_metrics (server_id, name, ref, path, collected_at)
		SELECT id, 'disk', 'disk:disk', '/dev/disk', now() - INTERVAL '10 days'
		FROM servers
		WHERE secret = 'legacy-storage-secret';

		INSERT INTO disk_physical_metrics (server_id, name, ref, path, collected_at, temp_c)
		SELECT id, 'disk', 'disk:disk', '/dev/disk', now() - INTERVAL '10 days', 40
		FROM servers
		WHERE secret = 'legacy-storage-secret';

		INSERT INTO disk_usage_metrics (server_id, name, ref, mountpoint, path, collected_at)
		SELECT id, 'disk', 'disk:disk', '/data', '/dev/disk', now() - INTERVAL '10 days'
		FROM servers
		WHERE secret = 'legacy-storage-secret';

		SELECT compress_chunk(chunk, true)
		FROM show_chunks('disk_metrics') AS chunk;
		SELECT compress_chunk(chunk, true)
		FROM show_chunks('disk_physical_metrics') AS chunk;
		SELECT compress_chunk(chunk, true)
		FROM show_chunks('disk_usage_metrics') AS chunk;
	`).Error; err != nil {
		t.Fatalf("seed legacy observation rows: %v", err)
	}
	result, err := migrate.Run(ctx, db, keyPath)
	if err != nil {
		t.Fatalf("rerun release migration: %v", err)
	}
	if result.Applied != 2 {
		t.Fatalf("applied migrations = %d, want 2", result.Applied)
	}

	type columnType struct {
		Name string `gorm:"column:name"`
		Kind string `gorm:"column:kind"`
	}
	var columns []columnType
	if err := db.Raw(`
		SELECT table_name || '.' || column_name AS name,
		       CASE
		           WHEN character_maximum_length IS NULL THEN data_type
		           ELSE data_type || '(' || character_maximum_length::text || ')'
		       END AS kind
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND (table_name, column_name) IN (
		      ('servers', 'name'),
		      ('servers', 'hostname'),
		      ('servers', 'platform_version'),
		      ('servers', 'kernel_version'),
		      ('servers', 'cpu_model'),
		      ('servers', 'cpu_vendor'),
		      ('servers', 'root_path'),
		      ('disk_metrics', 'name'),
		      ('disk_metrics', 'ref'),
		      ('disk_metrics', 'path'),
		      ('disk_metrics_15m', 'name'),
		      ('disk_metrics_15m', 'ref'),
		      ('disk_metrics_1h', 'name'),
		      ('disk_metrics_1h', 'ref'),
		      ('disk_physical_metrics', 'name'),
		      ('disk_physical_metrics', 'ref'),
		      ('disk_physical_metrics', 'path'),
		      ('disk_usage_metrics', 'name'),
		      ('disk_usage_metrics', 'ref'),
		      ('disk_usage_metrics', 'mountpoint'),
		      ('disk_usage_metrics', 'path'),
		      ('disk_usage_metrics_15m', 'name'),
		      ('disk_usage_metrics_15m', 'ref'),
		      ('disk_usage_metrics_15m', 'mountpoint'),
		      ('disk_usage_metrics_1h', 'name'),
		      ('disk_usage_metrics_1h', 'ref'),
		      ('disk_usage_metrics_1h', 'mountpoint'),
		      ('server_current_disk_metrics', 'name'),
		      ('server_current_disk_metrics', 'ref'),
		      ('server_current_disk_metrics', 'path'),
		      ('server_current_disk_usage_metrics', 'name'),
		      ('server_current_disk_usage_metrics', 'ref'),
		      ('server_current_disk_usage_metrics', 'mountpoint'),
		      ('server_current_disk_usage_metrics', 'path')
		  )
	`).Scan(&columns).Error; err != nil {
		t.Fatalf("load observation field types: %v", err)
	}
	got := make(map[string]string, len(columns))
	for _, column := range columns {
		got[column.Name] = column.Kind
	}
	want := map[string]string{
		"servers.name":                                 "character varying(255)",
		"servers.hostname":                             "character varying(255)",
		"servers.platform_version":                     "character varying(255)",
		"servers.kernel_version":                       "character varying(255)",
		"servers.cpu_model":                            "text",
		"servers.cpu_vendor":                           "text",
		"servers.root_path":                            "text",
		"disk_metrics.name":                            "character varying(255)",
		"disk_metrics.ref":                             "character varying(320)",
		"disk_metrics.path":                            "text",
		"disk_metrics_15m.name":                        "character varying(255)",
		"disk_metrics_15m.ref":                         "character varying(320)",
		"disk_metrics_1h.name":                         "character varying(255)",
		"disk_metrics_1h.ref":                          "character varying(320)",
		"disk_physical_metrics.name":                   "character varying(255)",
		"disk_physical_metrics.ref":                    "character varying(320)",
		"disk_physical_metrics.path":                   "text",
		"disk_usage_metrics.name":                      "character varying(255)",
		"disk_usage_metrics.ref":                       "character varying(320)",
		"disk_usage_metrics.mountpoint":                "text",
		"disk_usage_metrics.path":                      "text",
		"disk_usage_metrics_15m.name":                  "character varying(255)",
		"disk_usage_metrics_15m.ref":                   "character varying(320)",
		"disk_usage_metrics_15m.mountpoint":            "text",
		"disk_usage_metrics_1h.name":                   "character varying(255)",
		"disk_usage_metrics_1h.ref":                    "character varying(320)",
		"disk_usage_metrics_1h.mountpoint":             "text",
		"server_current_disk_metrics.name":             "character varying(255)",
		"server_current_disk_metrics.ref":              "character varying(320)",
		"server_current_disk_metrics.path":             "text",
		"server_current_disk_usage_metrics.name":       "character varying(255)",
		"server_current_disk_usage_metrics.ref":        "character varying(320)",
		"server_current_disk_usage_metrics.mountpoint": "text",
		"server_current_disk_usage_metrics.path":       "text",
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("%s type = %q, want %q", name, got[name], kind)
		}
	}

	var realtimeCount int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM timescaledb_information.continuous_aggregates
		WHERE view_name IN (
		    'disk_metrics_15m',
		    'disk_metrics_1h',
		    'disk_usage_metrics_15m',
		    'disk_usage_metrics_1h'
		)
		  AND materialized_only = FALSE
	`).Scan(&realtimeCount).Error; err != nil {
		t.Fatalf("count real-time disk aggregates: %v", err)
	}
	if realtimeCount != 4 {
		t.Fatalf("real-time disk aggregates = %d, want 4", realtimeCount)
	}

	var preserved int64
	if err := db.Raw(`
		SELECT
		    (SELECT COUNT(*) FROM disk_metrics WHERE name = 'disk')
		  + (SELECT COUNT(*) FROM disk_physical_metrics WHERE name = 'disk')
		  + (SELECT COUNT(*) FROM disk_usage_metrics WHERE name = 'disk')
	`).Scan(&preserved).Error; err != nil {
		t.Fatalf("count preserved disk rows: %v", err)
	}
	if preserved != 3 {
		t.Fatalf("preserved disk rows = %d, want 3", preserved)
	}
}

func TestIntegrationSchemaVersionGuard(t *testing.T) {
	ctx := context.Background()
	db, keyPath := pgtest.NewDBAt(t, 12)
	if err := migrate.CheckVersion(ctx, db); err != nil {
		t.Fatalf("CheckVersion(current) error = %v", err)
	}

	var target int64
	if err := db.Raw("SELECT max(version_id) FROM goose_db_version").Scan(&target).Error; err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if err := db.Exec("DELETE FROM goose_db_version WHERE version_id = ?", target).Error; err != nil {
		t.Fatalf("simulate older schema: %v", err)
	}
	if err := migrate.CheckVersion(ctx, db); !errors.Is(err, migrate.ErrSchemaBehind) {
		t.Fatalf("CheckVersion(behind) error = %v, want ErrSchemaBehind", err)
	}

	if err := db.Exec(
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, true)",
		target+1,
	).Error; err != nil {
		t.Fatalf("simulate newer schema: %v", err)
	}
	if err := migrate.CheckVersion(ctx, db); !errors.Is(err, migrate.ErrSchemaAhead) {
		t.Fatalf("CheckVersion(ahead) error = %v, want ErrSchemaAhead", err)
	}
	if _, err := migrate.Run(ctx, db, keyPath); !errors.Is(err, migrate.ErrSchemaAhead) {
		t.Fatalf("Run(ahead) error = %v, want ErrSchemaAhead", err)
	}
}
