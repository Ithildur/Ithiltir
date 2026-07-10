package alert

import (
	"fmt"
	"strings"

	"dash/internal/metrics"
)

type alertMessage struct {
	Title string
	Body  string
}

func buildOpenMessage(transition OpenTransition, configs ...MessageConfig) alertMessage {
	cfg := messageConfig(configs)
	text := textsFor(cfg.Language)
	server := serverLabel(transition.Snapshot, transition.ObjectID)
	if isOfflineRule(transition.Rule) {
		return alertMessage{
			Title: fmt.Sprintf(text.offlineOpenTitle, server),
			Body:  fmt.Sprintf(text.offlineOpenBody, formatAlertTime(transition.TriggeredAt, cfg)),
		}
	}

	ruleName := ruleDisplayName(transition.Rule, cfg.Language)
	titleName := openTitleName(transition.Rule.Metric, transition.Snapshot, ruleName, cfg.Language)
	detailLine := ""
	if detail := openMetricDetail(transition.Rule.Metric, transition.Snapshot, cfg.Language); detail != "" {
		detailLine = fmt.Sprintf(text.reasonLine, detail)
	}
	title := fmt.Sprintf(text.openTitle, titleName, server)
	body := fmt.Sprintf(
		text.openBody,
		server,
		ruleName,
		metricDisplayName(transition.Rule.Metric, cfg.Language),
		detailLine,
		formatMetricValue(transition.Rule.Metric, transition.CurrentValue),
		formatMetricValue(transition.Rule.Metric, transition.EffectiveThreshold),
		transition.Rule.DurationSec,
		formatAlertTime(transition.TriggeredAt, cfg),
	)
	return alertMessage{
		Title: title,
		Body:  body,
	}
}

func buildCloseMessage(transition CloseTransition, configs ...MessageConfig) alertMessage {
	cfg := messageConfig(configs)
	text := textsFor(cfg.Language)
	server := serverLabel(transition.Snapshot, transition.ObjectID)
	if isOfflineRule(transition.Rule) {
		return alertMessage{
			Title: fmt.Sprintf(text.offlineCloseTitle, server),
			Body:  fmt.Sprintf(text.offlineCloseBody, formatAlertTime(transition.ClosedAt, cfg)),
		}
	}

	ruleName := ruleDisplayName(transition.Rule, cfg.Language)
	currentValue := "-"
	if transition.CurrentValue != nil {
		currentValue = formatMetricValue(transition.Rule.Metric, *transition.CurrentValue)
	}
	return alertMessage{
		Title: fmt.Sprintf(text.closeTitle, ruleName, server),
		Body: fmt.Sprintf(
			text.closeBody,
			server,
			ruleName,
			metricDisplayName(transition.Rule.Metric, cfg.Language),
			currentValue,
			formatAlertTime(transition.ClosedAt, cfg),
		),
	}
}

type localizedName struct {
	zh string
	en string
}

var builtinRuleNames = map[string]localizedName{
	"node.offline":                     {zh: "节点离线", en: "Node offline"},
	"raid.failed":                      {zh: "RAID 失效", en: "RAID failure"},
	"disk.smart.failed":                {zh: "SMART 健康失败", en: "SMART health failure"},
	"disk.smart.nvme.critical_warning": {zh: "NVMe 关键警告", en: "NVMe critical warning"},
}

var metricNames = map[string]localizedName{
	"node.offline":                     {zh: "节点离线", en: "Node offline"},
	"raid.failed":                      {zh: "RAID 失效", en: "RAID failure"},
	"cpu.usage_ratio":                  {zh: "CPU 使用率", en: "CPU usage"},
	"cpu.load1":                        {zh: "1 分钟负载", en: "CPU load 1m"},
	"cpu.load5":                        {zh: "5 分钟负载", en: "CPU load 5m"},
	"cpu.load15":                       {zh: "15 分钟负载", en: "CPU load 15m"},
	"mem.used":                         {zh: "内存已用", en: "Memory used"},
	"mem.used_ratio":                   {zh: "内存使用率", en: "Memory usage"},
	"disk.usage.used_ratio":            {zh: "磁盘空间使用率", en: "Disk usage"},
	"disk.smart.failed":                {zh: "SMART 健康失败设备数", en: "Failed SMART devices"},
	"disk.smart.nvme.critical_warning": {zh: "NVMe 关键警告设备数", en: "NVMe critical warning devices"},
	"disk.smart.attribute_failing":     {zh: "SMART 属性失败数", en: "Failing SMART attributes"},
	"disk.smart.max_temp_c":            {zh: "SMART 最高温度", en: "Max SMART temperature"},
	"net.recv_bps":                     {zh: "入站带宽", en: "Inbound bandwidth"},
	"net.sent_bps":                     {zh: "出站带宽", en: "Outbound bandwidth"},
	"conn.tcp":                         {zh: "TCP 连接数", en: "TCP connections"},
	"thermal.max_temp_c":               {zh: "最高温度", en: "Max temperature"},
}

func ruleDisplayName(rule CompiledRule, language string) string {
	if rule.Builtin {
		if name, ok := localizedNameFor(builtinRuleNames, rule.Metric, language); ok {
			return name
		}
	}
	name := strings.TrimSpace(rule.Name)
	if name != "" {
		return name
	}
	return fmt.Sprintf("%s %s %s", metricDisplayName(rule.Metric, language), rule.Operator, formatMetricValue(rule.Metric, rule.Threshold))
}

func metricDisplayName(metricName, language string) string {
	if name, ok := localizedNameFor(metricNames, metricName, language); ok {
		return name
	}
	return metricName
}

func localizedNameFor(names map[string]localizedName, key, language string) (string, bool) {
	name, ok := names[strings.TrimSpace(key)]
	if !ok {
		return "", false
	}
	if language == messageLanguageEN {
		return name.en, true
	}
	return name.zh, true
}

func serverLabel(snapshot *metrics.NodeView, objectID int64) string {
	if snapshot != nil {
		if title := strings.TrimSpace(snapshot.Node.Title); title != "" {
			return title
		}
	}
	return fmt.Sprintf("server#%d", objectID)
}

func formatMetricValue(metricName string, value float64) string {
	if strings.HasSuffix(metricName, "_ratio") || strings.Contains(metricName, "usage_ratio") {
		if value <= 1 {
			return fmt.Sprintf("%.2f%%", value*100)
		}
		return fmt.Sprintf("%.2f%%", value)
	}
	if strings.HasPrefix(metricName, "mem.") && !strings.HasSuffix(metricName, "_ratio") {
		return fmt.Sprintf("%.0fB", value)
	}
	if strings.HasPrefix(metricName, "net.") {
		return fmt.Sprintf("%.2fB/s", value)
	}
	if strings.HasPrefix(metricName, "conn.") {
		return fmt.Sprintf("%.0f", value)
	}
	if strings.HasSuffix(metricName, "_temp_c") {
		return fmt.Sprintf("%.1fC", value)
	}
	return fmt.Sprintf("%.4g", value)
}
