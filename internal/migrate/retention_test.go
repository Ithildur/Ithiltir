package migrate_test

import (
	"testing"

	"dash/internal/migrate"
	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationRetentionPolicySyncPreservesJobs(t *testing.T) {
	db := pgtest.NewDB(t)
	type policy struct {
		JobID        int
		Scheduled    bool
		Schedule     string
		HypertableID int
		Days         int
	}
	read := func(table string) policy {
		t.Helper()
		var p policy
		if err := db.Raw(`SELECT job_id, scheduled, schedule_interval::text AS schedule,
			(config->>'hypertable_id')::int AS hypertable_id,
			extract(day FROM (config->>'drop_after')::interval)::int AS days
			FROM timescaledb_information.jobs
			WHERE hypertable_schema = current_schema() AND hypertable_name = ? AND proc_name = 'policy_retention'`, table).Scan(&p).Error; err != nil {
			t.Fatal(err)
		}
		return p
	}
	original := read("nic_metrics")
	if err := db.Exec("SELECT alter_job(?, scheduled => FALSE, schedule_interval => INTERVAL '2 days')", original.JobID).Error; err != nil {
		t.Fatal(err)
	}
	original = read("nic_metrics")
	if err := migrate.SyncRetentionPolicies(t.Context(), db, 8, 45, 45); err != nil {
		t.Fatal(err)
	}
	if got := read("nic_metrics"); got != original {
		t.Fatalf("unchanged policy was replaced: got %+v, want %+v", got, original)
	}
	if err := migrate.SyncRetentionPolicies(t.Context(), db, 6, 30, 20); err != nil {
		t.Fatal(err)
	}
	want := original
	want.Days = 30
	if got := read("nic_metrics"); got != want {
		t.Fatalf("updated policy lost job identity or configuration: got %+v, want %+v", got, want)
	}
	if read("server_metrics").Days != 6 || read("traffic_5m").Days != 20 {
		t.Fatal("raw metrics and traffic policies did not use their configured windows")
	}
	if err := db.Exec("SELECT remove_retention_policy('service_checks')").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrate.SyncRetentionPolicies(t.Context(), db, 6, 30, 20); err != nil {
		t.Fatal(err)
	}
	if got := read("service_checks"); got.JobID == 0 || got.Days != 30 {
		t.Fatalf("missing policy was not recreated: %+v", got)
	}
}
