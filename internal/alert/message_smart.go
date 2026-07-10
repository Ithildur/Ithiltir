package alert

import (
	"fmt"
	"strings"

	"dash/internal/metrics"
)

const (
	smartTitleDeviceLimit  = 3
	smartDetailDeviceLimit = 5
)

func openTitleName(metricName string, snapshot *metrics.NodeView, fallback, language string) string {
	devices := smartDevicesForMetric(metricName, snapshot)
	if len(devices) == 0 {
		return fallback
	}
	text := textsFor(language)
	return fallback + text.separator + smartDeviceTitleList(devices, text)
}

func openMetricDetail(metricName string, snapshot *metrics.NodeView, language string) string {
	devices := smartDevicesForMetric(metricName, snapshot)
	if len(devices) == 0 {
		return ""
	}
	text := textsFor(language)

	details := make([]string, 0, len(devices))
	for _, device := range devices {
		var detail string
		switch metricName {
		case "disk.smart.failed":
			detail = smartFailedDetail(device, text, language)
		case "disk.smart.nvme.critical_warning":
			detail = smartCriticalDetail(device, text, language)
		case "disk.smart.attribute_failing":
			detail = smartAttrDetail(device, text)
		}
		if detail != "" {
			details = append(details, detail)
		}
	}
	if len(details) == 0 {
		return ""
	}
	return strings.Join(limitSmartDetails(details, text), text.detailJoiner)
}

func smartDevicesForMetric(metricName string, snapshot *metrics.NodeView) []metrics.DiskSmartDevice {
	if snapshot == nil || snapshot.Disk.Smart == nil {
		return nil
	}
	devices := make([]metrics.DiskSmartDevice, 0)
	for _, device := range snapshot.Disk.Smart.Devices {
		switch metricName {
		case "disk.smart.failed":
			if smartHealthFailed(device) {
				devices = append(devices, device)
			}
		case "disk.smart.nvme.critical_warning":
			if smartHasCriticalWarning(device) {
				devices = append(devices, device)
			}
		case "disk.smart.attribute_failing":
			if len(smartFailingAttrs(device)) > 0 {
				devices = append(devices, device)
			}
		}
	}
	return devices
}

func smartFailedDetail(device metrics.DiskSmartDevice, text messageText, language string) string {
	parts := []string{smartDeviceLabel(device, text) + ": " + text.smartHealthFailed}
	if attrs := smartFailingAttrs(device); len(attrs) > 0 {
		parts = append(parts, smartAttrsText(attrs, text))
	}
	if smartHasCriticalWarning(device) {
		parts = append(parts, nvmeWarningText(*device.CriticalWarning, text, language))
	}
	if counters := nvmeErrorCountersText(device, text); counters != "" {
		parts = append(parts, counters)
	}
	return strings.Join(parts, text.detailJoiner)
}

func smartCriticalDetail(device metrics.DiskSmartDevice, text messageText, language string) string {
	if !smartHasCriticalWarning(device) {
		return ""
	}
	parts := []string{nvmeWarningText(*device.CriticalWarning, text, language)}
	if counters := nvmeErrorCountersText(device, text); counters != "" {
		parts = append(parts, counters)
	}
	return smartDeviceLabel(device, text) + ": " + strings.Join(parts, text.detailJoiner)
}

func smartAttrDetail(device metrics.DiskSmartDevice, text messageText) string {
	attrs := smartFailingAttrs(device)
	if len(attrs) == 0 {
		return ""
	}
	return smartDeviceLabel(device, text) + ": " + smartAttrsText(attrs, text)
}

func smartHealthFailed(device metrics.DiskSmartDevice) bool {
	return device.Health != nil && strings.EqualFold(strings.TrimSpace(*device.Health), "failed")
}

func smartHasCriticalWarning(device metrics.DiskSmartDevice) bool {
	return device.CriticalWarning != nil && *device.CriticalWarning != 0
}

func smartFailingAttrs(device metrics.DiskSmartDevice) []metrics.DiskSmartAttr {
	if len(device.FailingAttrs) == 0 {
		return nil
	}
	attrs := make([]metrics.DiskSmartAttr, 0, len(device.FailingAttrs))
	for _, attr := range device.FailingAttrs {
		if strings.EqualFold(strings.TrimSpace(attr.WhenFailed), "FAILING_NOW") {
			attrs = append(attrs, attr)
		}
	}
	return attrs
}

func smartDeviceTitleList(devices []metrics.DiskSmartDevice, text messageText) string {
	labels := make([]string, 0, len(devices))
	for _, device := range devices {
		label := smartDeviceShortLabel(device, text)
		if label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return ""
	}
	if len(labels) <= smartTitleDeviceLimit {
		return strings.Join(labels, ", ")
	}
	visible := strings.Join(labels[:smartTitleDeviceLimit], ", ")
	remaining := len(labels) - smartTitleDeviceLimit
	return fmt.Sprintf(text.smartTitleMore, visible, remaining)
}

func smartDeviceShortLabel(device metrics.DiskSmartDevice, text messageText) string {
	for _, raw := range []string{device.Name, device.DevicePath, device.Ref, device.Serial, device.WWN} {
		if value := strings.TrimSpace(raw); value != "" {
			return value
		}
	}
	return text.unknownSmartDevice
}

func smartDeviceLabel(device metrics.DiskSmartDevice, text messageText) string {
	label := smartDeviceShortLabel(device, text)
	model := strings.TrimSpace(device.Model)
	if model == "" {
		return label
	}
	return label + " (" + model + ")"
}

func smartAttrsText(attrs []metrics.DiskSmartAttr, text messageText) string {
	items := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		name := strings.TrimSpace(attr.Name)
		if name == "" && attr.ID > 0 {
			name = fmt.Sprintf("ID %d", attr.ID)
		}
		if name == "" {
			continue
		}
		if attr.ID > 0 && !strings.EqualFold(name, fmt.Sprintf("ID %d", attr.ID)) {
			name = fmt.Sprintf("%s (ID %d)", name, attr.ID)
		}
		items = append(items, name)
	}
	if len(items) == 0 {
		return ""
	}
	return fmt.Sprintf(text.smartFailingAttrs, strings.Join(items, ", "))
}

func nvmeWarningText(value uint64, text messageText, language string) string {
	reasons := nvmeWarningReasons(value, language)
	if len(reasons) == 0 {
		return fmt.Sprintf(text.nvmeWarning, value)
	}
	if language == messageLanguageEN {
		return fmt.Sprintf(text.nvmeWarning+" (%s)", value, strings.Join(reasons, ", "))
	}
	return fmt.Sprintf(text.nvmeWarning+"（%s）", value, strings.Join(reasons, "，"))
}

func nvmeErrorCountersText(device metrics.DiskSmartDevice, text messageText) string {
	if device.MediaErrors == nil {
		return ""
	}
	return fmt.Sprintf(text.nvmeMediaErrors, *device.MediaErrors)
}

func nvmeWarningReasons(value uint64, language string) []string {
	type reason struct {
		bit uint
		zh  string
		en  string
	}
	items := []reason{
		{bit: 0, zh: "备用空间低", en: "low spare"},
		{bit: 1, zh: "温度超限", en: "temperature exceeded"},
		{bit: 2, zh: "可靠性降低", en: "reliability degraded"},
		{bit: 3, zh: "只读", en: "read-only"},
		{bit: 4, zh: "断电保护失败", en: "volatile backup failed"},
		{bit: 5, zh: "PMR 进入只读模式", en: "PMR in read-only mode"},
	}
	reasons := make([]string, 0, len(items)+1)
	var known uint64
	for _, item := range items {
		mask := uint64(1) << item.bit
		known |= mask
		if value&mask == 0 {
			continue
		}
		if language == messageLanguageEN {
			reasons = append(reasons, item.en)
		} else {
			reasons = append(reasons, item.zh)
		}
	}
	if unknown := value &^ known; unknown != 0 {
		reasons = append(reasons, fmt.Sprintf(textsFor(language).nvmeUnknownWarningBits, unknown))
	}
	return reasons
}

func limitSmartDetails(details []string, text messageText) []string {
	if len(details) <= smartDetailDeviceLimit {
		return details
	}
	out := append([]string{}, details[:smartDetailDeviceLimit]...)
	remaining := len(details) - smartDetailDeviceLimit
	out = append(out, fmt.Sprintf(text.smartDetailMore, remaining))
	return out
}
