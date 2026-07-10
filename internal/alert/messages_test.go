package alert

import (
	"strings"
	"testing"
	"time"

	"dash/internal/metrics"
)

func TestBuildOpenMessageSmartCriticalWarningDetail(t *testing.T) {
	critical := uint64(0x0e)
	mediaErrors := uint64(3)
	triggeredAt := time.Date(2026, 7, 5, 8, 35, 53, 0, time.UTC)
	transition := OpenTransition{
		Rule: CompiledRule{
			RuleID:        -4,
			Builtin:       true,
			Name:          "smart_nvme_critical_warning",
			Metric:        "disk.smart.nvme.critical_warning",
			Threshold:     1,
			DurationSec:   0,
			ThresholdMode: "static",
		},
		ObjectID:           42,
		TriggeredAt:        triggeredAt,
		CurrentValue:       1,
		EffectiveThreshold: 1,
		Snapshot: &metrics.NodeView{
			Node: metrics.NodeMeta{Title: "9950X"},
			Disk: metrics.Disk{Smart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{{
				Name:            "nvme0n1",
				Model:           "Samsung SSD 990 PRO",
				Source:          "smartctl",
				Status:          "ok",
				CriticalWarning: &critical,
				MediaErrors:     &mediaErrors,
			}}}},
		},
	}

	msg := buildOpenMessage(transition, MessageConfig{Language: messageLanguageZH, Location: time.UTC})
	if strings.Contains(msg.Title, "smart_nvme_critical_warning") || strings.Contains(msg.Body, "smart_nvme_critical_warning") {
		t.Fatalf("message leaks builtin rule key: title=%q body=%q", msg.Title, msg.Body)
	}
	for _, want := range []string{
		"NVMe 关键警告：nvme0n1",
		"规则: NVMe 关键警告",
		"指标: NVMe 关键警告设备数",
		"原因: nvme0n1 (Samsung SSD 990 PRO): 关键警告 0x0E",
		"温度超限",
		"可靠性降低",
		"只读",
		"0E 媒体/数据完整性错误: 3",
	} {
		if !strings.Contains(alertMessageText(msg), want) {
			t.Fatalf("message missing %q:\n%s", want, alertMessageText(msg))
		}
	}
}

func TestBuildOpenMessageSmartFailedIncludesFailingAttrs(t *testing.T) {
	health := "failed"
	transition := OpenTransition{
		Rule: CompiledRule{
			RuleID:        -3,
			Builtin:       true,
			Name:          "smart_failed",
			Metric:        "disk.smart.failed",
			Threshold:     1,
			DurationSec:   0,
			ThresholdMode: "static",
		},
		ObjectID:           42,
		TriggeredAt:        time.Date(2026, 7, 5, 8, 35, 53, 0, time.UTC),
		CurrentValue:       1,
		EffectiveThreshold: 1,
		Snapshot: &metrics.NodeView{
			Node: metrics.NodeMeta{Title: "9950X"},
			Disk: metrics.Disk{Smart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{{
				Name:   "sda",
				Model:  "WDC WD40EFRX",
				Source: "smartctl",
				Status: "ok",
				Health: &health,
				FailingAttrs: []metrics.DiskSmartAttr{{
					ID:         5,
					Name:       "Reallocated_Sector_Ct",
					WhenFailed: "FAILING_NOW",
				}},
			}}}},
		},
	}

	msg := buildOpenMessage(transition, MessageConfig{Language: messageLanguageZH, Location: time.UTC})
	if strings.Contains(alertMessageText(msg), "smart_failed") {
		t.Fatalf("message leaks builtin rule key:\n%s", alertMessageText(msg))
	}
	for _, want := range []string{
		"SMART 健康失败：sda",
		"原因: sda (WDC WD40EFRX): 健康检查失败",
		"失败属性: Reallocated_Sector_Ct (ID 5)",
	} {
		if !strings.Contains(alertMessageText(msg), want) {
			t.Fatalf("message missing %q:\n%s", want, alertMessageText(msg))
		}
	}
}

func TestBuildOpenMessageSmartCriticalWarningDetailEnglish(t *testing.T) {
	critical := uint64(0x04)
	mediaErrors := uint64(3)
	transition := OpenTransition{
		Rule: CompiledRule{
			RuleID:        -4,
			Builtin:       true,
			Name:          "smart_nvme_critical_warning",
			Metric:        "disk.smart.nvme.critical_warning",
			Threshold:     1,
			DurationSec:   0,
			ThresholdMode: "static",
		},
		ObjectID:           42,
		TriggeredAt:        time.Date(2026, 7, 5, 8, 35, 53, 0, time.UTC),
		CurrentValue:       1,
		EffectiveThreshold: 1,
		Snapshot: &metrics.NodeView{
			Node: metrics.NodeMeta{Title: "9950X"},
			Disk: metrics.Disk{Smart: &metrics.DiskSmart{Devices: []metrics.DiskSmartDevice{{
				Name:            "nvme0n1",
				Source:          "smartctl",
				Status:          "ok",
				CriticalWarning: &critical,
				MediaErrors:     &mediaErrors,
			}}}},
		},
	}

	msg := buildOpenMessage(transition, MessageConfig{Language: messageLanguageEN, Location: time.UTC})
	if strings.Contains(alertMessageText(msg), "smart_nvme_critical_warning") {
		t.Fatalf("message leaks builtin rule key:\n%s", alertMessageText(msg))
	}
	for _, want := range []string{
		"Alert triggered: NVMe critical warning: nvme0n1 @ 9950X",
		"Rule: NVMe critical warning",
		"Metric: NVMe critical warning devices",
		"Reason: nvme0n1: critical warning 0x04 (reliability degraded); 0E media/data integrity errors: 3",
	} {
		if !strings.Contains(alertMessageText(msg), want) {
			t.Fatalf("message missing %q:\n%s", want, alertMessageText(msg))
		}
	}
}

func alertMessageText(msg alertMessage) string {
	return msg.Title + "\n" + msg.Body
}
