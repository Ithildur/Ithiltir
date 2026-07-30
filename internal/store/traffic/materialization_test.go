package traffic

import (
	"context"
	"math"
	"testing"
	"time"

	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/gorm"
)

func TestNextMaterializationRangeClampsAndOverlaps(t *testing.T) {
	floor := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	target := floor.Add(3 * time.Hour)

	start, end, advanced := nextMaterializationRange(
		model.TrafficMaterializationProgress{ScannedUntil: floor.Add(-24 * time.Hour)},
		target,
		floor,
		time.Minute,
	)
	if !advanced || !start.Equal(floor) || !end.Equal(floor.Add(time.Hour)) {
		t.Fatalf("clamped range = %s..%s/%v", start, end, advanced)
	}

	cursor := floor.Add(time.Hour)
	start, end, advanced = nextMaterializationRange(
		model.TrafficMaterializationProgress{ScannedUntil: cursor},
		target,
		floor,
		time.Minute,
	)
	if !advanced || !start.Equal(cursor) || !end.Equal(cursor.Add(time.Hour)) {
		t.Fatalf("catch-up range = %s..%s/%v", start, end, advanced)
	}

	cursor = target.Add(-time.Hour)
	start, end, advanced = nextMaterializationRange(
		model.TrafficMaterializationProgress{ScannedUntil: cursor},
		target,
		floor,
		time.Minute,
	)
	if !advanced || !start.Equal(cursor.Add(-time.Minute)) || !end.Equal(target) {
		t.Fatalf("head overlap range = %s..%s/%v", start, end, advanced)
	}

	start, end, advanced = nextMaterializationRange(
		model.TrafficMaterializationProgress{ScannedUntil: target},
		target,
		floor,
		time.Minute,
	)
	if advanced || !start.Equal(target) || !end.Equal(target) {
		t.Fatalf("complete range = %s..%s/%v", start, end, advanced)
	}
}

func TestIntegrationBillingActivationStartsWithRecentFacts(t *testing.T) {
	db := pgtest.NewDB(t)
	st := New(db)
	ctx := context.Background()
	ref := recentTrafficTestRef()
	oldCursor := ref.AddDate(-1, 0, 0)
	if err := db.Model(&model.TrafficMaterializationProgress{}).
		Where("kind = ?", string(materializationFacts)).
		Update("scanned_until", oldCursor).Error; err != nil {
		t.Fatalf("seed facts cursor: %v", err)
	}

	billing := UsageBilling
	if _, err := st.PatchSettingsAt(
		ctx,
		SettingsPatch{UsageMode: &billing},
		ref,
	); err != nil {
		t.Fatalf("enable billing: %v", err)
	}

	progress := loadProgress(t, db, materializationFacts)
	want := trafficBucketStart(ref.Add(-trafficBackfillWindow))
	if !progress.ScannedUntil.Equal(want) {
		t.Fatalf("facts cursor = %s, want %s", progress.ScannedUntil, want)
	}
}

func TestIntegrationLitePreventsFactWrites(t *testing.T) {
	db := pgtest.NewDB(t)
	st := New(db)
	ctx := context.Background()
	before := loadProgress(t, db, materializationFacts)

	hasMore, err := st.MaterializeTraffic5m(
		ctx,
		before.ScannedUntil.Add(time.Hour),
		before.ScannedUntil.Add(-24*time.Hour),
	)
	if err != nil {
		t.Fatalf("materialize facts in Lite: %v", err)
	}
	after := loadProgress(t, db, materializationFacts)
	if hasMore || !after.ScannedUntil.Equal(before.ScannedUntil) {
		t.Fatalf("Lite facts hasMore = %v, cursor %s -> %s", hasMore, before.ScannedUntil, after.ScannedUntil)
	}
}

func TestIntegrationMaterializersBridgeEmptyChunk(t *testing.T) {
	db := pgtest.NewDB(t)
	st := New(db)
	ctx := context.Background()

	server := model.Server{Name: "gap", Hostname: "gap", Secret: "gap-secret"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}
	start := recentTrafficTestRef().Add(-4 * time.Hour).Truncate(time.Hour)
	endSample := start.Add(2*time.Hour + 5*time.Minute)
	samples := []model.NICMetric{
		{ServerID: server.ID, Iface: "eth0", CollectedAt: start},
		{ServerID: server.ID, Iface: "eth0", CollectedAt: endSample, BytesRecv: 7500, BytesSent: 15000},
	}
	if err := db.Create(&samples).Error; err != nil {
		t.Fatalf("create nic samples: %v", err)
	}
	current := model.ServerCurrentNICMetric{
		ServerID:    server.ID,
		Iface:       "eth0",
		CollectedAt: endSample,
		BytesRecv:   7500,
		BytesSent:   15000,
	}
	if err := db.Create(&current).Error; err != nil {
		t.Fatalf("create current nic: %v", err)
	}
	if err := db.Model(&model.TrafficSetting{}).
		Where("id = ?", trafficSettingsID).
		Update("usage_mode", string(UsageBilling)).Error; err != nil {
		t.Fatalf("enable billing: %v", err)
	}
	for _, kind := range []materializationKind{materializationUsage, materializationFacts} {
		if err := db.Model(&model.TrafficMaterializationProgress{}).
			Where("kind = ?", string(kind)).
			Update("scanned_until", start).Error; err != nil {
			t.Fatalf("seed %s cursor: %v", kind, err)
		}
	}

	target := start.Add(4 * time.Hour)
	for range 3 {
		if _, err := st.MaterializeTrafficMonthUsage(ctx, time.UTC, target, start); err != nil {
			t.Fatalf("materialize usage: %v", err)
		}
		if _, err := st.MaterializeTraffic5m(ctx, target, start); err != nil {
			t.Fatalf("materialize facts: %v", err)
		}
	}

	var usage model.TrafficMonthUsage
	if err := db.Where("server_id = ? AND iface = ?", server.ID, "eth0").Take(&usage).Error; err != nil {
		t.Fatalf("load usage: %v", err)
	}
	if usage.InBytes != 7500 || usage.OutBytes != 15000 || usage.GapCount != 1 {
		t.Fatalf("usage bytes/gaps = %d/%d/%d, want 7500/15000/1", usage.InBytes, usage.OutBytes, usage.GapCount)
	}

	var facts struct {
		Rows     int64 `gorm:"column:rows"`
		InBytes  int64 `gorm:"column:in_bytes"`
		OutBytes int64 `gorm:"column:out_bytes"`
		GapCount int64 `gorm:"column:gap_count"`
	}
	if err := db.Model(&model.Traffic5m{}).
		Select("COUNT(*) AS rows, COALESCE(SUM(in_bytes), 0) AS in_bytes, COALESCE(SUM(out_bytes), 0) AS out_bytes, COALESCE(SUM(gap_count), 0) AS gap_count").
		Where("server_id = ? AND iface = ?", server.ID, "eth0").
		Scan(&facts).Error; err != nil {
		t.Fatalf("load facts: %v", err)
	}
	if facts.Rows != 25 || facts.InBytes != 7500 || facts.OutBytes != 15000 || facts.GapCount != 1 {
		t.Fatalf("facts rows/bytes/gaps = %d/%d/%d/%d, want 25/7500/15000/1", facts.Rows, facts.InBytes, facts.OutBytes, facts.GapCount)
	}
}

func TestIntegrationUsageOverflowDoesNotBlockMaterialization(t *testing.T) {
	db := pgtest.NewDB(t)
	st := New(db)
	ctx := context.Background()

	ref := time.Now().UTC().Add(-24 * time.Hour)
	start := time.Date(ref.Year(), ref.Month(), ref.Day(), 12, 0, 0, 0, time.UTC)
	target := start.Add(time.Hour)
	settings := defaultSettings()
	cycle := mustCycle(
		t,
		settings.CycleMode,
		settings.BillingStartDay,
		settings.BillingAnchorDate,
		time.UTC,
		target,
	)

	overflow := model.Server{Name: "overflow", Hostname: "overflow", Secret: "overflow-secret"}
	normal := model.Server{Name: "normal", Hostname: "normal", Secret: "normal-secret"}
	for _, server := range []*model.Server{&overflow, &normal} {
		if err := db.Create(server).Error; err != nil {
			t.Fatalf("create server %q: %v", server.Name, err)
		}
	}

	samples := []model.NICMetric{
		{ServerID: overflow.ID, Iface: "eth0", CollectedAt: start},
		{ServerID: overflow.ID, Iface: "eth0", CollectedAt: start.Add(time.Minute), BytesRecv: 10},
		{ServerID: normal.ID, Iface: "eth0", CollectedAt: start},
		{ServerID: normal.ID, Iface: "eth0", CollectedAt: start.Add(time.Minute), BytesRecv: 100},
	}
	if err := db.Create(&samples).Error; err != nil {
		t.Fatalf("create nic samples: %v", err)
	}

	existing := model.TrafficMonthUsage{
		ServerID:        overflow.ID,
		Iface:           "eth0",
		CycleMode:       string(cycle.Mode),
		BillingStartDay: int16(cycle.BillingStartDay),
		Timezone:        cycle.Timezone,
		CycleStart:      cycle.Start,
		CycleEnd:        cycle.End,
		CoveredFrom:     cycle.Start,
		CoveredUntil:    start,
		LastCollectedAt: start,
		InBytes:         math.MaxInt64 - 5,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing usage: %v", err)
	}
	if err := db.Model(&model.TrafficMaterializationProgress{}).
		Where("kind = ?", string(materializationUsage)).
		Update("scanned_until", start).Error; err != nil {
		t.Fatalf("seed usage cursor: %v", err)
	}

	if _, err := st.MaterializeTrafficMonthUsage(ctx, time.UTC, target, start); err != nil {
		t.Fatalf("materialize usage: %v", err)
	}
	progress := loadProgress(t, db, materializationUsage)
	if !progress.ScannedUntil.Equal(target) {
		t.Fatalf("usage cursor = %s, want %s", progress.ScannedUntil, target)
	}

	var overflowUsage model.TrafficMonthUsage
	if err := db.Where("server_id = ? AND iface = ?", overflow.ID, "eth0").Take(&overflowUsage).Error; err != nil {
		t.Fatalf("load overflow usage: %v", err)
	}
	if overflowUsage.InBytes != math.MaxInt64-5 || overflowUsage.GapCount != 1 {
		t.Fatalf("overflow usage = bytes:%d gaps:%d", overflowUsage.InBytes, overflowUsage.GapCount)
	}

	var normalUsage model.TrafficMonthUsage
	if err := db.Where("server_id = ? AND iface = ?", normal.ID, "eth0").Take(&normalUsage).Error; err != nil {
		t.Fatalf("load normal usage: %v", err)
	}
	if normalUsage.InBytes != 100 || normalUsage.GapCount != 0 {
		t.Fatalf("normal usage = bytes:%d gaps:%d", normalUsage.InBytes, normalUsage.GapCount)
	}
}

func TestIntegrationNodeCycleChangeQueuesOnlyNodeRepair(t *testing.T) {
	db := pgtest.NewDB(t)

	target := model.Server{Name: "target", Hostname: "target", Secret: "target-secret"}
	other := model.Server{
		Name:                   "other",
		Hostname:               "other",
		Secret:                 "other-secret",
		TrafficCycleMode:       string(CycleClampMonthEnd),
		TrafficBillingStartDay: 20,
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create target server: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other server: %v", err)
	}

	ref := recentTrafficTestRef()
	cycleStart := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.UTC)
	cycleEnd := cycleStart.AddDate(0, 1, 0)
	if err := db.Model(&model.TrafficMaterializationProgress{}).
		Where("kind = ?", string(materializationUsage)).
		Update("scanned_until", ref).Error; err != nil {
		t.Fatalf("seed usage cursor: %v", err)
	}
	for _, serverID := range []int64{target.ID, other.ID} {
		row := model.TrafficMonthUsage{
			ServerID:        serverID,
			Iface:           "eth0",
			CycleMode:       string(CycleCalendarMonth),
			BillingStartDay: 1,
			Timezone:        "UTC",
			CycleStart:      cycleStart,
			CycleEnd:        cycleEnd,
			CoveredFrom:     cycleStart,
			CoveredUntil:    ref,
			LastCollectedAt: ref,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create usage row for server %d: %v", serverID, err)
		}
	}

	next := ServerCycleSettings{
		Mode:            ServerCycleMode(CycleClampMonthEnd),
		BillingStartDay: 15,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := PrepareServerCycleChange(tx, target.ID, next, time.UTC, ref); err != nil {
			return err
		}
		return tx.Model(&model.Server{}).
			Where("id = ?", target.ID).
			Updates(map[string]any{
				"traffic_cycle_mode":          string(next.Mode),
				"traffic_billing_start_day":   next.BillingStartDay,
				"traffic_billing_anchor_date": "",
				"traffic_billing_timezone":    "",
			}).Error
	}); err != nil {
		t.Fatalf("change target cycle: %v", err)
	}

	var targetRows, otherRows int64
	if err := db.Model(&model.TrafficMonthUsage{}).
		Where("server_id = ?", target.ID).
		Count(&targetRows).Error; err != nil {
		t.Fatalf("count target usage: %v", err)
	}
	if err := db.Model(&model.TrafficMonthUsage{}).
		Where("server_id = ?", other.ID).
		Count(&otherRows).Error; err != nil {
		t.Fatalf("count other usage: %v", err)
	}
	if targetRows != 0 || otherRows != 1 {
		t.Fatalf("usage rows target/other = %d/%d, want 0/1", targetRows, otherRows)
	}

	progress := loadProgress(t, db, materializationUsage)
	if !progress.ScannedUntil.Equal(ref) {
		t.Fatalf("usage cursor = %s, want unchanged %s", progress.ScannedUntil, ref)
	}

	var repairs []model.TrafficUsageRepair
	if err := db.Order("server_id ASC").Find(&repairs).Error; err != nil {
		t.Fatalf("load usage repairs: %v", err)
	}
	wantRepairAt := cycleStart
	if len(repairs) != 1 || repairs[0].ServerID != target.ID || !repairs[0].ScannedUntil.Equal(wantRepairAt) {
		t.Fatalf("usage repairs = %#v, want target server at %s", repairs, wantRepairAt)
	}
}

func TestIntegrationUsageRepairStepProcessesOneNode(t *testing.T) {
	db := pgtest.NewDB(t)
	st := New(db)
	start := recentTrafficTestRef().Add(-time.Hour).Truncate(time.Hour)

	servers := []model.Server{
		{Name: "first", Hostname: "first", Secret: "first-secret"},
		{Name: "second", Hostname: "second", Secret: "second-secret"},
	}
	if err := db.Create(&servers).Error; err != nil {
		t.Fatalf("create servers: %v", err)
	}
	repairs := []model.TrafficUsageRepair{
		{ServerID: servers[0].ID, ScannedUntil: start},
		{ServerID: servers[1].ID, ScannedUntil: start},
	}
	if err := db.Create(&repairs).Error; err != nil {
		t.Fatalf("create usage repairs: %v", err)
	}

	hasMore, err := st.MaterializeTrafficMonthUsageRepair(
		context.Background(),
		time.UTC,
		start,
		start,
	)
	if err != nil {
		t.Fatalf("repair usage: %v", err)
	}
	if !hasMore {
		t.Fatal("repair usage hasMore = false, want true")
	}

	var remaining []model.TrafficUsageRepair
	if err := db.Order("server_id ASC").Find(&remaining).Error; err != nil {
		t.Fatalf("load remaining usage repairs: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ServerID != servers[1].ID {
		t.Fatalf("remaining usage repairs = %#v, want second server", remaining)
	}
}

func TestIntegrationUsageRepairDoesNotMoveLiveCursor(t *testing.T) {
	db := pgtest.NewDB(t)
	st := New(db)
	ctx := context.Background()
	start := recentTrafficTestRef().Add(-time.Hour).Truncate(time.Hour)
	target := start.Add(time.Hour)

	servers := []model.Server{
		{Name: "live", Hostname: "live", Secret: "live-secret"},
		{Name: "repairing", Hostname: "repairing", Secret: "repairing-secret"},
	}
	if err := db.Create(&servers).Error; err != nil {
		t.Fatalf("create servers: %v", err)
	}
	live, repairing := servers[0], servers[1]
	for _, server := range []model.Server{live, repairing} {
		samples := []model.NICMetric{
			{ServerID: server.ID, Iface: "eth0", CollectedAt: start},
			{ServerID: server.ID, Iface: "eth0", CollectedAt: start.Add(30 * time.Minute), BytesRecv: 1800, BytesSent: 3600},
		}
		if err := db.Create(&samples).Error; err != nil {
			t.Fatalf("create server %d samples: %v", server.ID, err)
		}
		current := model.ServerCurrentNICMetric{
			ServerID:    server.ID,
			Iface:       "eth0",
			CollectedAt: start.Add(30 * time.Minute),
			BytesRecv:   1800,
			BytesSent:   3600,
		}
		if err := db.Create(&current).Error; err != nil {
			t.Fatalf("create server %d current nic: %v", server.ID, err)
		}
	}
	if err := db.Model(&model.TrafficMaterializationProgress{}).
		Where("kind = ?", string(materializationUsage)).
		Update("scanned_until", start).Error; err != nil {
		t.Fatalf("seed usage cursor: %v", err)
	}
	if err := db.Create(&model.TrafficUsageRepair{
		ServerID:     repairing.ID,
		ScannedUntil: start,
	}).Error; err != nil {
		t.Fatalf("create usage repair: %v", err)
	}

	if _, err := st.MaterializeTrafficMonthUsage(ctx, time.UTC, target, start); err != nil {
		t.Fatalf("materialize live usage: %v", err)
	}
	var liveRows, repairingRows int64
	if err := db.Model(&model.TrafficMonthUsage{}).Where("server_id = ?", live.ID).Count(&liveRows).Error; err != nil {
		t.Fatalf("count live usage: %v", err)
	}
	if err := db.Model(&model.TrafficMonthUsage{}).Where("server_id = ?", repairing.ID).Count(&repairingRows).Error; err != nil {
		t.Fatalf("count repairing usage before repair: %v", err)
	}
	if liveRows != 1 || repairingRows != 0 {
		t.Fatalf("usage rows live/repairing = %d/%d, want 1/0", liveRows, repairingRows)
	}
	progress := loadProgress(t, db, materializationUsage)
	if !progress.ScannedUntil.Equal(target) {
		t.Fatalf("live usage cursor = %s, want %s", progress.ScannedUntil, target)
	}

	hasMore, err := st.MaterializeTrafficMonthUsageRepair(ctx, time.UTC, target, start)
	if err != nil {
		t.Fatalf("repair usage: %v", err)
	}
	if hasMore {
		t.Fatal("repair usage hasMore = true, want false")
	}
	if err := db.Model(&model.TrafficMonthUsage{}).Where("server_id = ?", repairing.ID).Count(&repairingRows).Error; err != nil {
		t.Fatalf("count repaired usage: %v", err)
	}
	if repairingRows != 1 {
		t.Fatalf("repaired usage rows = %d, want 1", repairingRows)
	}
	progress = loadProgress(t, db, materializationUsage)
	if !progress.ScannedUntil.Equal(target) {
		t.Fatalf("usage cursor after repair = %s, want unchanged %s", progress.ScannedUntil, target)
	}
	var repairCount int64
	if err := db.Model(&model.TrafficUsageRepair{}).Count(&repairCount).Error; err != nil {
		t.Fatalf("count completed repairs: %v", err)
	}
	if repairCount != 0 {
		t.Fatalf("completed repair rows = %d, want 0", repairCount)
	}
}

func loadProgress(t *testing.T, db *gorm.DB, kind materializationKind) model.TrafficMaterializationProgress {
	t.Helper()
	var progress model.TrafficMaterializationProgress
	if err := db.Where("kind = ?", string(kind)).Take(&progress).Error; err != nil {
		t.Fatalf("load %s progress: %v", kind, err)
	}
	return progress
}

func recentTrafficTestRef() time.Time {
	now := time.Now().UTC()
	ref := time.Date(now.Year(), now.Month(), 26, 12, 0, 0, 0, time.UTC)
	if ref.After(now.Add(-time.Hour)) {
		ref = ref.AddDate(0, -1, 0)
	}
	return ref
}
