package alert

import (
	"testing"
	"time"

	"dash/internal/alertspec"
	"dash/internal/metrics"
	"dash/internal/model"
)

func TestCompileRulesRejectsInvalidCorePlusMetric(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	compiled := CompileRules([]model.AlertRule{{
		ID:            7,
		Name:          "invalid",
		Enabled:       true,
		Generation:    3,
		Metric:        "mem.used_ratio",
		Operator:      ">=",
		Threshold:     0.9,
		DurationSec:   60,
		ThresholdMode: "core_plus",
		UpdatedAt:     now,
	}}, now)

	if len(compiled.Invalid) != 1 || compiled.Invalid[0].RuleID != 7 {
		t.Fatalf("expected rule 7 to be invalid, got %+v", compiled.Invalid)
	}
	if hasRule(compiled.Rules, 7) {
		t.Fatalf("expected invalid rule to be absent from compiled rules")
	}
}

func TestCompiledRulesForMountsUsesBuiltinDefault(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	compiled := CompileRules([]model.AlertRule{{
		ID:            9,
		Name:          "cpu_high",
		Enabled:       true,
		Generation:    1,
		Metric:        "cpu.load1",
		Operator:      ">=",
		Threshold:     1,
		DurationSec:   60,
		ThresholdMode: "static",
		UpdatedAt:     now,
	}}, now)

	mounted := compiled.ForMounts(map[int64]bool{
		alertspec.BuiltinOfflineID: false,
		9:                          true,
	})
	if hasRule(mounted.Rules, alertspec.BuiltinOfflineID) {
		t.Fatalf("expected offline builtin to be unmounted")
	}
	if !hasRule(mounted.Rules, alertspec.BuiltinRaidID) ||
		!hasRule(mounted.Rules, alertspec.BuiltinSmartFailedID) ||
		!hasRule(mounted.Rules, alertspec.BuiltinSmartCriticalID) ||
		!hasRule(mounted.Rules, 9) {
		t.Fatalf("expected default builtin and explicit user rule to be mounted")
	}
}

func TestNormalizeMetricRejectsHistoricalAlias(t *testing.T) {
	if _, err := alertspec.NormalizeMetric("disk.total_used_ratio"); err == nil {
		t.Fatalf("expected historical disk metric alias to be rejected")
	}
}

func TestNormalizeRuleModelRejectsStaticOffset(t *testing.T) {
	_, err := alertspec.NormalizeRuleModel(model.AlertRule{
		Name:            "cpu_high",
		Metric:          "cpu.usage_ratio",
		Operator:        ">=",
		Threshold:       0.9,
		DurationSec:     60,
		ThresholdMode:   "static",
		ThresholdOffset: 1,
		Generation:      1,
	})
	if err == nil {
		t.Fatalf("expected static threshold_mode with non-zero offset to be rejected")
	}
}

func TestExtractMetricValueDiskFallsBackToFirstMount(t *testing.T) {
	node := metrics.NodeView{Disk: metrics.Disk{Mounts: []metrics.DiskMount{
		{Mountpoint: "/data", UsedRatio: 0.7},
	}}}
	got, ok := alertspec.ExtractMetricValue("disk.usage.used_ratio", node)
	if !ok || got != 0.7 {
		t.Fatalf("expected fallback mount used ratio 0.7, got %v, %v", got, ok)
	}
}

func TestExtractMetricValueRaidFailed(t *testing.T) {
	node := metrics.NodeView{Raid: &metrics.RAID{
		Available: true,
		Arrays: []metrics.RAIDArray{
			{Failed: 0, Health: "degraded"},
			{Failed: 2},
		},
	}}
	got, ok := alertspec.ExtractMetricValue("raid.failed", node)
	if !ok || got != 3 {
		t.Fatalf("expected RAID failed score 3, got %v, %v", got, ok)
	}
}

func TestExtractMetricValueSmartAndThermal(t *testing.T) {
	failed := "failed"
	passed := "passed"
	smartTemp := 55.5
	criticalWarning := uint64(0x0e)
	thermalTemp := 64.25
	node := metrics.NodeView{
		Disk: metrics.Disk{Smart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{
			{
				Status:          "ok",
				Health:          &failed,
				TempC:           &smartTemp,
				CriticalWarning: &criticalWarning,
				FailingAttrs: []metrics.DiskSmartAttr{{
					ID:         184,
					Name:       "End-to-End_Error",
					WhenFailed: "FAILING_NOW",
				}, {
					ID:         5,
					Name:       "Reallocated_Sector_Ct",
					WhenFailed: "",
				}},
			},
			{Status: "no_tool"},
			{Status: "ok", Health: &passed},
		}}},
		Thermal: &metrics.Thermal{Sensors: []metrics.ThermalSensor{
			{Status: "ok", TempC: &thermalTemp},
			{Status: "not_found"},
		}},
	}

	got, ok := alertspec.ExtractMetricValue("disk.smart.failed", node)
	if !ok || got != 1 {
		t.Fatalf("expected SMART failed score 1, got %v, %v", got, ok)
	}

	got, ok = alertspec.ExtractMetricValue("disk.smart.nvme.critical_warning", node)
	if !ok || got != 1 {
		t.Fatalf("expected SMART critical warning score 1, got %v, %v", got, ok)
	}

	got, ok = alertspec.ExtractMetricValue("disk.smart.attribute_failing", node)
	if !ok || got != 1 {
		t.Fatalf("expected SMART failing attribute score 1, got %v, %v", got, ok)
	}

	got, ok = alertspec.ExtractMetricValue("disk.smart.max_temp_c", node)
	if !ok || got != smartTemp {
		t.Fatalf("expected SMART max temp %.1f, got %v, %v", smartTemp, got, ok)
	}

	got, ok = alertspec.ExtractMetricValue("thermal.max_temp_c", node)
	if !ok || got != thermalTemp {
		t.Fatalf("expected thermal max temp %.2f, got %v, %v", thermalTemp, got, ok)
	}

	_, ok = alertspec.ExtractMetricValue("disk.smart.failed", metrics.NodeView{})
	if ok {
		t.Fatalf("expected missing SMART data to be not evaluated")
	}
}

func hasRule(rules []CompiledRule, id int64) bool {
	for _, rule := range rules {
		if rule.RuleID == id {
			return true
		}
	}
	return false
}
