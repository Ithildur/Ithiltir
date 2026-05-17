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

	ruleName := ruleDisplayName(transition.Rule)
	title := fmt.Sprintf(text.openTitle, ruleName, server)
	body := fmt.Sprintf(
		text.openBody,
		server,
		ruleName,
		transition.Rule.Metric,
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

	ruleName := ruleDisplayName(transition.Rule)
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
			transition.Rule.Metric,
			currentValue,
			formatAlertTime(transition.ClosedAt, cfg),
		),
	}
}

func ruleDisplayName(rule CompiledRule) string {
	name := strings.TrimSpace(rule.Name)
	if name != "" {
		return name
	}
	return fmt.Sprintf("%s %s %s", rule.Metric, rule.Operator, formatMetricValue(rule.Metric, rule.Threshold))
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
