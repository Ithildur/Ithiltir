package alertspec

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"dash/internal/metrics"
	"dash/internal/model"
)

type metricSpec struct {
	extract         func(node metrics.NodeView) (float64, bool)
	corePlusAllowed bool
}

const (
	BuiltinOfflineID       int64 = -1
	BuiltinRaidID          int64 = -2
	BuiltinSmartFailedID   int64 = -3
	BuiltinSmartCriticalID int64 = -4
	// -5 was used by a retired built-in rule; do not reuse it.
	maxRuleNameRunes       = 128
	maxCooldownMin   int32 = 365 * 24 * 60
)

type BuiltinRule struct {
	ID          int64
	Name        string
	Metric      string
	Operator    string
	Threshold   float64
	DurationSec int32
	CooldownMin int32
}

type Threshold struct {
	Metric string
	Mode   string
	Value  float64
	Offset float64
}

func BuiltinRules() []BuiltinRule {
	return []BuiltinRule{
		{
			ID:          BuiltinOfflineID,
			Name:        "node_offline",
			Metric:      "node.offline",
			Operator:    ">=",
			Threshold:   1,
			DurationSec: 0,
			CooldownMin: 0,
		},
		{
			ID:          BuiltinRaidID,
			Name:        "raid_failed",
			Metric:      "raid.failed",
			Operator:    ">=",
			Threshold:   1,
			DurationSec: 0,
			CooldownMin: 30,
		},
		{
			ID:          BuiltinSmartFailedID,
			Name:        "smart_failed",
			Metric:      "disk.smart.failed",
			Operator:    ">=",
			Threshold:   1,
			DurationSec: 0,
			CooldownMin: 30,
		},
		{
			ID:          BuiltinSmartCriticalID,
			Name:        "smart_nvme_critical_warning",
			Metric:      "disk.smart.nvme.critical_warning",
			Operator:    ">=",
			Threshold:   1,
			DurationSec: 0,
			CooldownMin: 30,
		},
	}
}

func BuiltinRuleIDs() []int64 {
	items := BuiltinRules()
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

type ValidationError struct {
	msg string
}

func (e ValidationError) Error() string {
	return e.msg
}

func IsValidationError(err error) bool {
	var target ValidationError
	return errors.As(err, &target)
}

func invalid(msg string) error {
	return ValidationError{msg: msg}
}

func invalidf(format string, args ...any) error {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

var metricRegistry = map[string]metricSpec{
	"node.offline": {
		extract: func(metrics.NodeView) (float64, bool) {
			return 0, false
		},
	},
	"cpu.usage_ratio": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return node.CPU.UsageRatio, true
		},
	},
	"cpu.load1": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return node.CPU.Load.L1, true
		},
		corePlusAllowed: true,
	},
	"cpu.load5": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return node.CPU.Load.L5, true
		},
		corePlusAllowed: true,
	},
	"cpu.load15": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return node.CPU.Load.L15, true
		},
		corePlusAllowed: true,
	},
	"mem.used": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return float64(node.Memory.UsedBytes), true
		},
	},
	"mem.used_ratio": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return node.Memory.UsedRatio, true
		},
	},
	"disk.usage.used_ratio": {
		extract: func(node metrics.NodeView) (float64, bool) {
			mount, ok := pickPrimaryMount(node)
			if !ok {
				return 0, false
			}
			return mount.UsedRatio, true
		},
	},
	"disk.smart.failed": {
		extract: smartFailed,
	},
	"disk.smart.nvme.critical_warning": {
		extract: smartCriticalWarning,
	},
	"disk.smart.attribute_failing": {
		extract: smartAttributeFailing,
	},
	"disk.smart.max_temp_c": {
		extract: smartMaxTempC,
	},
	"net.recv_bps": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return node.Network.Total.RecvBPS, true
		},
	},
	"net.sent_bps": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return node.Network.Total.SentBPS, true
		},
	},
	"conn.tcp": {
		extract: func(node metrics.NodeView) (float64, bool) {
			return float64(node.Connections.TCP), true
		},
	},
	"raid.failed": {
		extract: raidFailed,
	},
	"thermal.max_temp_c": {
		extract: thermalMaxTempC,
	},
}

var operatorRegistry = map[string]struct{}{
	">":  {},
	">=": {},
	"<":  {},
	"<=": {},
	"==": {},
	"!=": {},
}

var thresholdModeRegistry = map[string]struct{}{
	"static":    {},
	"core_plus": {},
}

var durationSecRegistry = map[int32]struct{}{
	0:   {},
	60:  {},
	300: {},
}

func NormalizeMetric(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", invalid("metric is required")
	}
	if _, ok := metricRegistry[name]; !ok {
		return "", invalid("metric is not supported")
	}
	return name, nil
}

func IsAllowedOperator(op string) bool {
	_, ok := operatorRegistry[strings.TrimSpace(op)]
	return ok
}

func IsAllowedThresholdMode(mode string) bool {
	_, ok := thresholdModeRegistry[strings.TrimSpace(mode)]
	return ok
}

func IsAllowedDurationSec(sec int32) bool {
	_, ok := durationSecRegistry[sec]
	return ok
}

func PrepareCreateRule(rule model.AlertRule) (model.AlertRule, error) {
	if rule.Generation == 0 {
		rule.Generation = 1
	}
	if strings.TrimSpace(rule.ThresholdMode) == "" {
		rule.ThresholdMode = "static"
	}
	rule.IsDeleted = false
	return NormalizeRuleModel(rule)
}

func NormalizeRuleModel(rule model.AlertRule) (model.AlertRule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		return model.AlertRule{}, invalid("name is required")
	}
	if utf8.RuneCountInString(rule.Name) > maxRuleNameRunes || containsControl(rule.Name) {
		return model.AlertRule{}, invalid("name must contain at most 128 characters and no control characters")
	}
	if !finite(rule.Threshold) {
		return model.AlertRule{}, invalid("threshold must be finite")
	}
	if !finite(rule.ThresholdOffset) {
		return model.AlertRule{}, invalid("threshold_offset must be finite")
	}

	metric, err := NormalizeMetric(rule.Metric)
	if err != nil {
		return model.AlertRule{}, err
	}
	rule.Metric = metric

	rule.Operator = strings.TrimSpace(rule.Operator)
	if rule.Operator == "" {
		return model.AlertRule{}, invalid("operator is required")
	}
	if !IsAllowedOperator(rule.Operator) {
		return model.AlertRule{}, invalid("operator is not supported")
	}

	rule.ThresholdMode = strings.TrimSpace(rule.ThresholdMode)
	if !IsAllowedThresholdMode(rule.ThresholdMode) {
		return model.AlertRule{}, invalid("threshold_mode is not supported")
	}

	if !IsAllowedDurationSec(rule.DurationSec) {
		return model.AlertRule{}, invalid("duration_sec is not supported")
	}
	if rule.CooldownMin < 0 || rule.CooldownMin > maxCooldownMin {
		return model.AlertRule{}, invalid("cooldown_min is not supported")
	}

	if rule.Generation <= 0 {
		return model.AlertRule{}, invalid("generation is required")
	}
	if rule.ThresholdMode == "core_plus" && !SupportsCorePlus(rule.Metric) {
		return model.AlertRule{}, invalidf("metric %s does not support core_plus", rule.Metric)
	}
	if rule.ThresholdMode == "static" && rule.ThresholdOffset != 0 {
		return model.AlertRule{}, invalid("threshold_offset must be 0 for static threshold_mode")
	}
	return rule, nil
}

func Compare(operator string, value, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func ExtractMetricValue(metricName string, node metrics.NodeView) (float64, bool) {
	spec, ok := metricRegistry[strings.TrimSpace(metricName)]
	if !ok {
		return 0, false
	}
	return spec.extract(node)
}

func SupportsCorePlus(metricName string) bool {
	spec, ok := metricRegistry[strings.TrimSpace(metricName)]
	return ok && spec.corePlusAllowed
}

func EffectiveThreshold(rule Threshold, node metrics.NodeView) (float64, error) {
	var threshold float64
	switch strings.TrimSpace(rule.Mode) {
	case "", "static":
		threshold = rule.Value
	case "core_plus":
		metricName := strings.TrimSpace(rule.Metric)
		spec, ok := metricRegistry[metricName]
		if !ok || !spec.corePlusAllowed {
			return 0, fmt.Errorf("metric %s does not support core_plus", metricName)
		}
		threshold = float64(ResolveCPUCores(node)) + rule.Value + rule.Offset
	default:
		return 0, fmt.Errorf("unsupported threshold_mode %s", rule.Mode)
	}
	if !finite(threshold) {
		return 0, errors.New("effective threshold is not finite")
	}
	return threshold, nil
}

func ResolveCPUCores(node metrics.NodeView) int {
	if node.CPU.CoresLogical > 0 {
		return node.CPU.CoresLogical
	}
	if node.CPU.CoresPhysical > 0 {
		return node.CPU.CoresPhysical
	}
	return 0
}

func pickPrimaryMount(node metrics.NodeView) (metrics.DiskMount, bool) {
	for _, mount := range node.Disk.Mounts {
		if strings.TrimSpace(mount.Mountpoint) == "/" {
			return mount, true
		}
	}
	// Mounts are sorted by size when the view is built. On systems without a
	// Unix root mount (notably Windows), the largest filesystem is primary.
	if len(node.Disk.Mounts) == 0 {
		return metrics.DiskMount{}, false
	}
	return node.Disk.Mounts[0], true
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func raidFailed(node metrics.NodeView) (float64, bool) {
	if node.Raid == nil || !node.Raid.Available {
		return 0, false
	}

	var n int
	for _, array := range node.Raid.Arrays {
		if array.Failed > 0 {
			n += array.Failed
			continue
		}
		if raidArrayUnhealthy(array.Health) {
			n++
		}
	}
	return float64(n), true
}

func raidArrayUnhealthy(health string) bool {
	health = strings.ToLower(strings.TrimSpace(health))
	return health != "" && health != "healthy"
}

func smartFailed(node metrics.NodeView) (float64, bool) {
	if node.Disk.Smart == nil {
		return 0, false
	}
	var n int
	for _, device := range node.Disk.Smart.Devices {
		if device.Health != nil && strings.EqualFold(strings.TrimSpace(*device.Health), "failed") {
			n++
		}
	}
	return float64(n), true
}

func smartCriticalWarning(node metrics.NodeView) (float64, bool) {
	if node.Disk.Smart == nil {
		return 0, false
	}
	var n int
	var seen bool
	for _, device := range node.Disk.Smart.Devices {
		if device.CriticalWarning == nil {
			continue
		}
		seen = true
		if *device.CriticalWarning != 0 {
			n++
		}
	}
	return float64(n), seen
}

func smartAttributeFailing(node metrics.NodeView) (float64, bool) {
	if node.Disk.Smart == nil {
		return 0, false
	}
	var n int
	var seen bool
	for _, device := range node.Disk.Smart.Devices {
		if device.FailingAttrs == nil {
			continue
		}
		seen = true
		for _, attr := range device.FailingAttrs {
			if strings.EqualFold(strings.TrimSpace(attr.WhenFailed), "FAILING_NOW") {
				n++
			}
		}
	}
	return float64(n), seen
}

func smartMaxTempC(node metrics.NodeView) (float64, bool) {
	if node.Disk.Smart == nil {
		return 0, false
	}
	return maxSmartTempC(node.Disk.Smart.Devices)
}

func thermalMaxTempC(node metrics.NodeView) (float64, bool) {
	if node.Thermal == nil {
		return 0, false
	}
	var max float64
	var ok bool
	for _, sensor := range node.Thermal.Sensors {
		if sensor.TempC == nil {
			continue
		}
		if !ok || *sensor.TempC > max {
			max = *sensor.TempC
			ok = true
		}
	}
	return max, ok
}

func maxSmartTempC(devices []metrics.DiskSmartDevice) (float64, bool) {
	var max float64
	var ok bool
	for _, device := range devices {
		if device.TempC == nil {
			continue
		}
		if !ok || *device.TempC > max {
			max = *device.TempC
			ok = true
		}
	}
	return max, ok
}
