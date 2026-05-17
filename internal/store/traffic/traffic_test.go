package traffic

import (
	"context"
	"reflect"
	"testing"
	"time"

	"dash/internal/model"
)

func TestP95DiscardTop(t *testing.T) {
	values := make([]float64, 100)
	for i := range values {
		values[i] = float64(i + 1)
	}
	if got := p95DiscardTop(values); got != 95 {
		t.Fatalf("p95 = %v, want 95", got)
	}
}

func TestTrafficCycleCalendarMonth(t *testing.T) {
	loc := time.UTC
	ref := time.Date(2026, time.April, 26, 12, 0, 0, 0, loc)

	cycle := currentTrafficCycle(CycleCalendarMonth, 15, loc, ref)

	wantStart := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	if !cycle.Start.Equal(wantStart) || !cycle.End.Equal(wantEnd) {
		t.Fatalf("cycle = %s..%s, want %s..%s", cycle.Start, cycle.End, wantStart, wantEnd)
	}
	if cycle.BillingStartDay != 1 {
		t.Fatalf("billing day = %d, want 1", cycle.BillingStartDay)
	}
}

func TestListTrafficIfaces(t *testing.T) {
	st, db := newSQLiteStore(t)
	ctx := context.Background()
	if err := db.AutoMigrate(&model.NICMetric{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	rows := []model.NICMetric{
		{ServerID: 7, Iface: "zz0", CollectedAt: start},
		{ServerID: 7, Iface: "aa0", CollectedAt: start.Add(time.Minute)},
		{ServerID: 7, Iface: "zz0", CollectedAt: start.Add(2 * time.Minute)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	ifaces, err := st.ListTrafficIfaces(ctx, 7)
	if err != nil {
		t.Fatalf("ListTrafficIfaces() error = %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("ifaces = %d, want 2", len(ifaces))
	}
	if ifaces[0].Name != "aa0" || ifaces[1].Name != "zz0" {
		t.Fatalf("ifaces = %#v, want distinct names in name order", ifaces)
	}
}

func TestTrafficCycleClampMonthEnd(t *testing.T) {
	loc := time.UTC
	ref := time.Date(2026, time.February, 15, 12, 0, 0, 0, loc)

	cycle := currentTrafficCycle(CycleClampMonthEnd, 31, loc, ref)

	wantStart := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)
	if !cycle.Start.Equal(wantStart) || !cycle.End.Equal(wantEnd) {
		t.Fatalf("cycle = %s..%s, want %s..%s", cycle.Start, cycle.End, wantStart, wantEnd)
	}
}

func TestTrafficCycleWHMCSCompatible(t *testing.T) {
	loc := time.UTC
	ref := time.Date(2026, time.February, 15, 12, 0, 0, 0, loc)

	cycle := currentTrafficCycle(CycleWHMCS, 31, loc, ref)

	wantStart := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)
	if !cycle.Start.Equal(wantStart) || !cycle.End.Equal(wantEnd) {
		t.Fatalf("cycle = %s..%s, want %s..%s", cycle.Start, cycle.End, wantStart, wantEnd)
	}

	ref = time.Date(2026, time.March, 15, 12, 0, 0, 0, loc)
	cycle = currentTrafficCycle(CycleWHMCS, 31, loc, ref)
	wantStart = time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)
	wantEnd = time.Date(2026, time.April, 3, 0, 0, 0, 0, time.UTC)
	if !cycle.Start.Equal(wantStart) || !cycle.End.Equal(wantEnd) {
		t.Fatalf("cycle = %s..%s, want %s..%s", cycle.Start, cycle.End, wantStart, wantEnd)
	}
}

func TestTrafficCycleWHMCSAnchorOverridesBillingDay(t *testing.T) {
	loc := time.UTC
	ref := time.Date(2026, time.February, 15, 12, 0, 0, 0, loc)

	cycle := currentTrafficCycleAnchored(CycleWHMCS, 31, "2026-01-30", loc, ref)

	wantStart := time.Date(2026, time.January, 30, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	if !cycle.Start.Equal(wantStart) || !cycle.End.Equal(wantEnd) {
		t.Fatalf("cycle = %s..%s, want %s..%s", cycle.Start, cycle.End, wantStart, wantEnd)
	}
	if cycle.BillingStartDay != 30 {
		t.Fatalf("billing day = %d, want 30", cycle.BillingStartDay)
	}
	if cycle.BillingAnchorDate != "2026-01-30" {
		t.Fatalf("anchor = %q, want 2026-01-30", cycle.BillingAnchorDate)
	}
}

func TestSplitTrafficSamplesAcrossBuckets(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 4, 0, 0, time.UTC)
	end := time.Date(2026, time.April, 1, 0, 9, 0, 0, time.UTC)

	samples := splitTrafficSamples(7, "eth0", start, end, 300, 600)

	if len(samples) != 2 {
		t.Fatalf("samples = %d, want 2", len(samples))
	}
	wantBuckets := []time.Time{
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 1, 0, 5, 0, 0, time.UTC),
	}
	wantIn := []int64{60, 240}
	wantOut := []int64{120, 480}
	var inTotal int64
	var outTotal int64
	for i, sample := range samples {
		if !sample.Bucket.Equal(wantBuckets[i]) {
			t.Fatalf("sample %d bucket = %s, want %s", i, sample.Bucket, wantBuckets[i])
		}
		if sample.InBytes != wantIn[i] || sample.OutBytes != wantOut[i] {
			t.Fatalf("sample %d bytes = %d/%d, want %d/%d", i, sample.InBytes, sample.OutBytes, wantIn[i], wantOut[i])
		}
		inTotal += sample.InBytes
		outTotal += sample.OutBytes
	}
	if inTotal != 300 || outTotal != 600 {
		t.Fatalf("totals = %d/%d, want 300/600", inTotal, outTotal)
	}
	for i, sample := range samples {
		if !sample.Valid {
			t.Fatalf("sample %d should be valid", i)
		}
	}
}

func TestSplitTrafficSamplesMarksLongGapInvalid(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)

	samples := splitTrafficSamples(7, "eth0", start, end, 1800, 3600)

	if len(samples) != 6 {
		t.Fatalf("samples = %d, want 6", len(samples))
	}
	if samples[0].Gap != 1 {
		t.Fatalf("first gap = %d, want 1", samples[0].Gap)
	}
	for i, sample := range samples {
		if sample.Valid {
			t.Fatalf("sample %d should be invalid", i)
		}
		if sample.InRate != 0 || sample.OutRate != 0 {
			t.Fatalf("sample %d rate = %v/%v, want 0/0", i, sample.InRate, sample.OutRate)
		}
	}
}

func TestTrafficCoverageDoesNotTreatGapBucketsAsSamples(t *testing.T) {
	cycle := TrafficCycle{
		Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.April, 1, 0, 30, 0, 0, time.UTC),
	}
	rows := []trafficBucket{
		{InBytes: 300, OutBytes: 600, InRateBytesPerSec: 1, OutRateBytesPerSec: 2, InPeakBytesPerSec: 1, OutPeakBytesPerSec: 2, GapCount: 1},
		{InBytes: 300, OutBytes: 600, InRateBytesPerSec: 1, OutRateBytesPerSec: 2, InPeakBytesPerSec: 1, OutPeakBytesPerSec: 2},
		{InBytes: 300, OutBytes: 600, InRateBytesPerSec: 1, OutRateBytesPerSec: 2, InPeakBytesPerSec: 1, OutPeakBytesPerSec: 2},
		{InBytes: 300, OutBytes: 600, InRateBytesPerSec: 1, OutRateBytesPerSec: 2, InPeakBytesPerSec: 1, OutPeakBytesPerSec: 2},
		{InBytes: 300, OutBytes: 600, InRateBytesPerSec: 1, OutRateBytesPerSec: 2, InPeakBytesPerSec: 1, OutPeakBytesPerSec: 2},
		{InBytes: 300, OutBytes: 600, InRateBytesPerSec: 1, OutRateBytesPerSec: 2, InPeakBytesPerSec: 1, OutPeakBytesPerSec: 2},
	}

	stat := buildTrafficStat(rows, cycle.Start, cycle.End, true, TrafficSnapshotSealed, DirectionBoth, UsageBilling, true)

	if stat.SampleCount != 0 {
		t.Fatalf("sample count = %d, want 0", stat.SampleCount)
	}
	if stat.CoverageRatio != 0 {
		t.Fatalf("coverage = %v, want 0", stat.CoverageRatio)
	}
	if !stat.Partial {
		t.Fatalf("partial = false, want true")
	}
}

func TestTrafficStatEndUsesCompletedBucketForCurrentCycle(t *testing.T) {
	cycle := TrafficCycle{
		Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.April, 1, 1, 0, 0, 0, time.UTC),
	}
	ref := time.Date(2026, time.April, 1, 0, 17, 30, 0, time.UTC)

	end, complete := trafficStatEnd(cycle, ref)
	if complete {
		t.Fatalf("complete = true, want false")
	}
	wantEnd := time.Date(2026, time.April, 1, 0, 15, 0, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Fatalf("stat end = %s, want %s", end, wantEnd)
	}

	stat := emptyTrafficStat(cycle.Start, end, complete, TrafficSnapshotProvisional)
	if stat.ExpectedSampleCount != 3 {
		t.Fatalf("expected samples = %d, want 3", stat.ExpectedSampleCount)
	}
	if stat.CycleComplete {
		t.Fatalf("cycle complete = true, want false")
	}
}

func TestTrafficDailyKeepsCurrentDayIncomplete(t *testing.T) {
	statEnd := time.Date(2026, time.April, 2, 12, 0, 0, 0, time.UTC)
	items := buildTrafficDaily(
		TrafficQuery{
			ServerID:      7,
			Iface:         "eth0",
			UsageMode:     UsageBilling,
			DirectionMode: DirectionOut,
			Location:      time.UTC,
			P95Enabled:    true,
		},
		[]trafficBucket{
			{
				Bucket:             time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
				InBytes:            300,
				OutBytes:           600,
				InRateBytesPerSec:  1,
				OutRateBytesPerSec: 2,
				SampleCount:        1,
			},
			{
				Bucket:             time.Date(2026, time.April, 2, 11, 55, 0, 0, time.UTC),
				InBytes:            30,
				OutBytes:           60,
				InRateBytesPerSec:  1,
				OutRateBytesPerSec: 2,
				SampleCount:        1,
			},
		},
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		statEnd,
		statEnd,
		false,
		TrafficSnapshotProvisional,
	)

	if len(items) != 2 {
		t.Fatalf("daily items = %d, want 2", len(items))
	}
	if !items[0].Stat.CycleComplete {
		t.Fatal("closed day complete = false, want true")
	}
	if items[1].Stat.CycleComplete {
		t.Fatal("current day complete = true, want false")
	}
}

func TestTrafficInvalidBucketIgnoredForP95AndPeak(t *testing.T) {
	cycle := TrafficCycle{
		Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.April, 1, 0, 10, 0, 0, time.UTC),
	}
	rows := []trafficBucket{
		{
			InBytes:            300,
			OutBytes:           600,
			InRateBytesPerSec:  999,
			OutRateBytesPerSec: 999,
			InPeakBytesPerSec:  999,
			OutPeakBytesPerSec: 999,
			SampleCount:        0,
			GapCount:           1,
		},
	}

	stat := buildTrafficStat(rows, cycle.Start, cycle.End, true, TrafficSnapshotSealed, DirectionBoth, UsageBilling, true)

	if stat.InP95BytesPerSec != 0 || stat.OutP95BytesPerSec != 0 {
		t.Fatalf("p95 = %v/%v, want 0/0", stat.InP95BytesPerSec, stat.OutP95BytesPerSec)
	}
	if stat.InPeakBytesPerSec != 0 || stat.OutPeakBytesPerSec != 0 {
		t.Fatalf("peak = %v/%v, want 0/0", stat.InPeakBytesPerSec, stat.OutPeakBytesPerSec)
	}
	if stat.InBytes != 300 || stat.OutBytes != 600 {
		t.Fatalf("bytes = %d/%d, want 300/600", stat.InBytes, stat.OutBytes)
	}
}

func TestTrafficMonthlyPartialSnapshotStaysRecoverable(t *testing.T) {
	summary := TrafficSummary{
		Stat: TrafficStat{
			Status:        TrafficSnapshotSealed,
			CycleComplete: true,
			DataComplete:  false,
		},
	}

	recoverable := trafficMonthlySaveable(summary, true)
	if recoverable.Stat.Status != TrafficSnapshotGrace {
		t.Fatalf("recoverable status = %s, want grace", recoverable.Stat.Status)
	}

	expired := trafficMonthlySaveable(summary, false)
	if expired.Stat.Status != TrafficSnapshotStale {
		t.Fatalf("expired status = %s, want stale", expired.Stat.Status)
	}
}

func TestTrafficMonthlySealedPartialReusableOnlyAfterSourceExpires(t *testing.T) {
	row := model.TrafficMonthly{
		Status:              string(TrafficSnapshotStale),
		SampleCount:         1,
		ExpectedSampleCount: 2,
	}

	reusable, err := trafficMonthlySnapshotReusable(row, true)
	if err != nil {
		t.Fatalf("trafficMonthlySnapshotReusable() error = %v", err)
	}
	if reusable {
		t.Fatalf("partial snapshot should be recomputed while source is available")
	}
	reusable, err = trafficMonthlySnapshotReusable(row, false)
	if err != nil {
		t.Fatalf("trafficMonthlySnapshotReusable() error = %v", err)
	}
	if !reusable {
		t.Fatalf("partial snapshot should be reusable after source expires")
	}
}

func TestTrafficMonthlySnapshotRejectsUnknownStatus(t *testing.T) {
	row := model.TrafficMonthly{
		Status:              "unknown",
		SampleCount:         2,
		ExpectedSampleCount: 2,
	}

	if _, err := trafficMonthlySnapshotReusable(row, true); err == nil {
		t.Fatalf("unknown snapshot status should be rejected")
	}
}

func TestBuildTraffic5mRowsIsIdempotent(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)
	rows := []trafficNICRow{
		{ServerID: 7, Iface: "eth0", CollectedAt: start, BytesRecv: 100, BytesSent: 200},
		{ServerID: 7, Iface: "eth0", CollectedAt: start.Add(5 * time.Minute), BytesRecv: 400, BytesSent: 800},
		{ServerID: 7, Iface: "eth0", CollectedAt: end, BytesRecv: 700, BytesSent: 1400},
	}

	first := buildTraffic5mRows(rows, start, end)
	second := buildTraffic5mRows(rows, start, end)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("backfill rows are not stable")
	}
	if len(first) != 2 {
		t.Fatalf("rows = %d, want 2", len(first))
	}
	for i, row := range first {
		if row.InBytes != 300 || row.OutBytes != 600 {
			t.Fatalf("row %d bytes = %d/%d, want 300/600", i, row.InBytes, row.OutBytes)
		}
		if row.SampleCount != 1 {
			t.Fatalf("row %d sample_count = %d, want 1", i, row.SampleCount)
		}
	}
}

func TestBuildTraffic5mRowsKeepsCoveredZeroTrafficBucket(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	rows := []trafficNICRow{
		{ServerID: 7, Iface: "eth0", CollectedAt: start, BytesRecv: 100, BytesSent: 200},
		{ServerID: 7, Iface: "eth0", CollectedAt: end, BytesRecv: 100, BytesSent: 200},
	}

	items := buildTraffic5mRows(rows, start, end)

	if len(items) != 1 {
		t.Fatalf("rows = %d, want 1", len(items))
	}
	if items[0].InBytes != 0 || items[0].OutBytes != 0 {
		t.Fatalf("bytes = %d/%d, want 0/0", items[0].InBytes, items[0].OutBytes)
	}
	if items[0].SampleCount != 1 {
		t.Fatalf("sample_count = %d, want 1", items[0].SampleCount)
	}
}

func TestBuildTrafficMonthUsageRowsDoesNotWriteAllAggregate(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	settings := Settings{
		CycleMode:       CycleCalendarMonth,
		BillingStartDay: 1,
	}
	rows := []trafficNICRow{
		{ServerID: 7, Iface: "eth0", CollectedAt: start, BytesRecv: 0, BytesSent: 0},
		{ServerID: 7, Iface: "eth0", CollectedAt: end, BytesRecv: 300, BytesSent: 600},
		{ServerID: 7, Iface: "eth1", CollectedAt: start, BytesRecv: 0, BytesSent: 0},
		{ServerID: 7, Iface: "eth1", CollectedAt: end, BytesRecv: 900, BytesSent: 1200},
	}

	items := buildTrafficMonthUsageRows(rows, settings, time.UTC, start, end, map[trafficUsageKey]time.Time{})

	byIface := make(map[string]modelTrafficUsageForTest)
	for _, item := range items {
		byIface[item.Iface] = modelTrafficUsageForTest{
			inBytes:  item.InBytes,
			outBytes: item.OutBytes,
			inPeak:   item.InPeakBytesPerSec,
			outPeak:  item.OutPeakBytesPerSec,
		}
	}
	if _, ok := byIface["all"]; ok {
		t.Fatalf("all aggregate row should not be persisted")
	}
	eth0 := byIface["eth0"]
	if eth0.inBytes != 300 || eth0.outBytes != 600 {
		t.Fatalf("eth0 bytes = %d/%d, want 300/600", eth0.inBytes, eth0.outBytes)
	}
	eth1 := byIface["eth1"]
	if eth1.inBytes != 900 || eth1.outBytes != 1200 {
		t.Fatalf("eth1 bytes = %d/%d, want 900/1200", eth1.inBytes, eth1.outBytes)
	}
}

func TestBuildTrafficMonthUsageRowsKeepsIfaceRowsSeparate(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	settings := Settings{
		CycleMode:       CycleCalendarMonth,
		BillingStartDay: 1,
	}
	rows := []trafficNICRow{
		{ServerID: 7, Iface: "eth0", CollectedAt: start.Add(time.Second), BytesRecv: 0, BytesSent: 0},
		{ServerID: 7, Iface: "eth0", CollectedAt: start.Add(6 * time.Second), BytesRecv: 300, BytesSent: 600},
		{ServerID: 7, Iface: "eth1", CollectedAt: start.Add(2 * time.Second), BytesRecv: 0, BytesSent: 0},
		{ServerID: 7, Iface: "eth1", CollectedAt: start.Add(7 * time.Second), BytesRecv: 900, BytesSent: 1200},
	}

	items := buildTrafficMonthUsageRows(rows, settings, time.UTC, start, start.Add(5*time.Minute), map[trafficUsageKey]time.Time{})

	byIface := make(map[string]modelTrafficUsageForTest)
	for _, item := range items {
		byIface[item.Iface] = modelTrafficUsageForTest{
			inBytes:  item.InBytes,
			outBytes: item.OutBytes,
			inPeak:   item.InPeakBytesPerSec,
			outPeak:  item.OutPeakBytesPerSec,
		}
	}
	if _, ok := byIface["all"]; ok {
		t.Fatalf("all aggregate row should not be persisted")
	}
	if byIface["eth0"].inBytes != 300 || byIface["eth1"].inBytes != 900 {
		t.Fatalf("iface bytes = %d/%d, want 300/900", byIface["eth0"].inBytes, byIface["eth1"].inBytes)
	}
}

func TestBuildTrafficMonthUsageRowsKeepsGapBytesOutOfPeak(t *testing.T) {
	start := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)
	settings := Settings{
		CycleMode:       CycleCalendarMonth,
		BillingStartDay: 1,
	}
	rows := []trafficNICRow{
		{ServerID: 7, Iface: "eth0", CollectedAt: start, BytesRecv: 0, BytesSent: 0},
		{ServerID: 7, Iface: "eth0", CollectedAt: end, BytesRecv: 600, BytesSent: 1200},
	}

	items := buildTrafficMonthUsageRows(rows, settings, time.UTC, start, end, map[trafficUsageKey]time.Time{})

	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.InBytes != 600 || item.OutBytes != 1200 {
		t.Fatalf("bytes = %d/%d, want 600/1200", item.InBytes, item.OutBytes)
	}
	if item.SampleCount != 0 || item.GapCount != 1 {
		t.Fatalf("sample/gap = %d/%d, want 0/1", item.SampleCount, item.GapCount)
	}
	if item.InPeakBytesPerSec != 0 || item.OutPeakBytesPerSec != 0 {
		t.Fatalf("peak = %v/%v, want 0/0", item.InPeakBytesPerSec, item.OutPeakBytesPerSec)
	}
}

func TestBuildTrafficMonthUsageRowsUsesGivenLocation(t *testing.T) {
	loc := time.FixedZone("Asia/Test", 8*60*60)
	start := time.Date(2026, time.March, 31, 16, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	settings := Settings{
		CycleMode:       CycleCalendarMonth,
		BillingStartDay: 1,
	}
	rows := []trafficNICRow{
		{ServerID: 7, Iface: "eth0", CollectedAt: start, BytesRecv: 0, BytesSent: 0},
		{ServerID: 7, Iface: "eth0", CollectedAt: end, BytesRecv: 120, BytesSent: 240},
	}

	items := buildTrafficMonthUsageRows(rows, settings, loc, start, end, map[trafficUsageKey]time.Time{})

	if len(items) == 0 {
		t.Fatalf("usage rows are missing")
	}
	wantStart := time.Date(2026, time.March, 31, 16, 0, 0, 0, time.UTC)
	for _, item := range items {
		if !item.CycleStart.Equal(wantStart) {
			t.Fatalf("cycle start = %s, want %s", item.CycleStart, wantStart)
		}
		if item.Timezone != loc.String() {
			t.Fatalf("timezone = %s, want %s", item.Timezone, loc.String())
		}
	}
}

func TestBuildTrafficMonthUsageRowsUsesServerCycleOverride(t *testing.T) {
	start := time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	settings := Settings{
		CycleMode:       CycleCalendarMonth,
		BillingStartDay: 1,
	}
	rows := []trafficNICRow{
		{
			ServerID:        7,
			Iface:           "eth0",
			ServerCycleMode: string(CycleClampMonthEnd),
			BillingStartDay: 15,
			CollectedAt:     start,
			BytesRecv:       0,
			BytesSent:       0,
		},
		{
			ServerID:        7,
			Iface:           "eth0",
			ServerCycleMode: string(CycleClampMonthEnd),
			BillingStartDay: 15,
			CollectedAt:     end,
			BytesRecv:       120,
			BytesSent:       240,
		},
	}

	items := buildTrafficMonthUsageRows(rows, settings, time.UTC, start, end, map[trafficUsageKey]time.Time{})

	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.CycleMode != string(CycleClampMonthEnd) {
		t.Fatalf("cycle mode = %q, want %q", item.CycleMode, CycleClampMonthEnd)
	}
	if item.BillingStartDay != 15 {
		t.Fatalf("billing day = %d, want 15", item.BillingStartDay)
	}
	wantStart := time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC)
	if !item.CycleStart.Equal(wantStart) {
		t.Fatalf("cycle start = %s, want %s", item.CycleStart, wantStart)
	}
}

type modelTrafficUsageForTest struct {
	inBytes  int64
	outBytes int64
	inPeak   float64
	outPeak  float64
}

func TestTrafficBillingSelectionUsesOutboundP95(t *testing.T) {
	cycle := TrafficCycle{
		Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.April, 1, 8, 20, 0, 0, time.UTC),
	}
	rows := make([]trafficBucket, 100)
	for i := range rows {
		inRate := float64(1001 + i)
		outRate := float64(1 + i)
		rows[i] = trafficBucket{
			InBytes:            int64(inRate),
			OutBytes:           int64(outRate),
			InRateBytesPerSec:  inRate,
			OutRateBytesPerSec: outRate,
			InPeakBytesPerSec:  inRate,
			OutPeakBytesPerSec: outRate,
			SampleCount:        1,
		}
	}

	stat := buildTrafficStat(rows, cycle.Start, cycle.End, true, TrafficSnapshotSealed, DirectionOut, UsageBilling, true)

	if stat.OutP95BytesPerSec != 95 {
		t.Fatalf("out p95 = %v, want 95", stat.OutP95BytesPerSec)
	}
	if stat.InP95BytesPerSec != 1095 {
		t.Fatalf("in p95 = %v, want 1095", stat.InP95BytesPerSec)
	}
	if stat.SelectedP95BytesPerSec != stat.OutP95BytesPerSec || stat.SelectedP95Direction != TrafficDirectionOutKey {
		t.Fatalf("selected p95 = %v/%s, want outbound %v", stat.SelectedP95BytesPerSec, stat.SelectedP95Direction, stat.OutP95BytesPerSec)
	}
	if stat.SelectedPeakBytesPerSec != stat.OutPeakBytesPerSec || stat.SelectedPeakDirection != TrafficDirectionOutKey {
		t.Fatalf("selected peak = %v/%s, want outbound %v", stat.SelectedPeakBytesPerSec, stat.SelectedPeakDirection, stat.OutPeakBytesPerSec)
	}
}

func TestTrafficBillingSelectionUsesBothDirections(t *testing.T) {
	cycle := TrafficCycle{
		Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.April, 1, 8, 20, 0, 0, time.UTC),
	}
	rows := make([]trafficBucket, 100)
	for i := range rows {
		inRate := float64(1001 + i)
		outRate := float64(1 + i)
		rows[i] = trafficBucket{
			InBytes:            int64(inRate),
			OutBytes:           int64(outRate),
			InRateBytesPerSec:  inRate,
			OutRateBytesPerSec: outRate,
			InPeakBytesPerSec:  inRate,
			OutPeakBytesPerSec: outRate,
			SampleCount:        1,
		}
	}

	stat := buildTrafficStat(rows, cycle.Start, cycle.End, true, TrafficSnapshotSealed, DirectionBoth, UsageBilling, true)

	if stat.SelectedBytes != stat.InBytes+stat.OutBytes || stat.SelectedBytesDirection != TrafficDirectionTotal {
		t.Fatalf("selected bytes = %d/%s, want total %d", stat.SelectedBytes, stat.SelectedBytesDirection, stat.InBytes+stat.OutBytes)
	}
	if stat.BothP95BytesPerSec != 1190 {
		t.Fatalf("both p95 = %v, want 1190", stat.BothP95BytesPerSec)
	}
	if stat.SelectedP95BytesPerSec != stat.BothP95BytesPerSec || stat.SelectedP95Direction != TrafficDirectionTotal {
		t.Fatalf("selected p95 = %v/%s, want total %v", stat.SelectedP95BytesPerSec, stat.SelectedP95Direction, stat.BothP95BytesPerSec)
	}
	if stat.SelectedPeakBytesPerSec != stat.BothPeakBytesPerSec || stat.SelectedPeakDirection != TrafficDirectionTotal {
		t.Fatalf("selected peak = %v/%s, want total %v", stat.SelectedPeakBytesPerSec, stat.SelectedPeakDirection, stat.BothPeakBytesPerSec)
	}
}

func TestTrafficBillingSelectionUsesMaxDirection(t *testing.T) {
	cycle := TrafficCycle{
		Start: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.April, 1, 8, 20, 0, 0, time.UTC),
	}
	rows := make([]trafficBucket, 100)
	for i := range rows {
		inRate := float64(1001 + i)
		outRate := float64(1 + i)
		rows[i] = trafficBucket{
			InBytes:            int64(inRate),
			OutBytes:           int64(outRate),
			InRateBytesPerSec:  inRate,
			OutRateBytesPerSec: outRate,
			InPeakBytesPerSec:  inRate,
			OutPeakBytesPerSec: outRate,
			SampleCount:        1,
		}
	}

	stat := buildTrafficStat(rows, cycle.Start, cycle.End, true, TrafficSnapshotSealed, DirectionMax, UsageBilling, true)

	if stat.SelectedBytes != stat.InBytes || stat.SelectedBytesDirection != TrafficDirectionInKey {
		t.Fatalf("selected bytes = %d/%s, want inbound %d", stat.SelectedBytes, stat.SelectedBytesDirection, stat.InBytes)
	}
	if stat.SelectedP95BytesPerSec != stat.InP95BytesPerSec || stat.SelectedP95Direction != TrafficDirectionInKey {
		t.Fatalf("selected p95 = %v/%s, want inbound %v", stat.SelectedP95BytesPerSec, stat.SelectedP95Direction, stat.InP95BytesPerSec)
	}
	if stat.SelectedPeakBytesPerSec != stat.InPeakBytesPerSec || stat.SelectedPeakDirection != TrafficDirectionInKey {
		t.Fatalf("selected peak = %v/%s, want inbound %v", stat.SelectedPeakBytesPerSec, stat.SelectedPeakDirection, stat.InPeakBytesPerSec)
	}
}
