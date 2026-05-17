package alert

import (
	"strings"
	"time"
)

const (
	messageLanguageZH = "zh"
	messageLanguageEN = "en"
)

type MessageConfig struct {
	Language string
	Location *time.Location
}

type messageText struct {
	openTitle         string
	openBody          string
	closeTitle        string
	closeBody         string
	offlineOpenTitle  string
	offlineOpenBody   string
	offlineCloseTitle string
	offlineCloseBody  string
}

var messageTexts = map[string]messageText{
	messageLanguageZH: {
		openTitle:         "告警触发: %s @ %s",
		openBody:          "状态: opened\n服务器: %s\n规则: %s\n指标: %s\n当前值: %s\n阈值: %s\n持续时间: %ds\n触发时间: %s",
		closeTitle:        "告警恢复: %s @ %s",
		closeBody:         "状态: closed\n服务器: %s\n规则: %s\n指标: %s\n当前值: %s\n恢复时间: %s",
		offlineOpenTitle:  "离线告警：%s",
		offlineOpenBody:   "触发时间: %s",
		offlineCloseTitle: "恢复在线：%s",
		offlineCloseBody:  "恢复时间: %s",
	},
	messageLanguageEN: {
		openTitle:         "Alert triggered: %s @ %s",
		openBody:          "Status: opened\nServer: %s\nRule: %s\nMetric: %s\nCurrent value: %s\nThreshold: %s\nDuration: %ds\nTriggered at: %s",
		closeTitle:        "Alert recovered: %s @ %s",
		closeBody:         "Status: closed\nServer: %s\nRule: %s\nMetric: %s\nCurrent value: %s\nRecovered at: %s",
		offlineOpenTitle:  "Offline alert: %s",
		offlineOpenBody:   "Triggered at: %s",
		offlineCloseTitle: "Online restored: %s",
		offlineCloseBody:  "Recovered at: %s",
	},
}

func messageConfig(configs []MessageConfig) MessageConfig {
	cfg := MessageConfig{Language: messageLanguageZH, Location: time.Local}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	cfg.Language = normalizeMessageLanguage(cfg.Language)
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return cfg
}

func normalizeMessageLanguage(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case messageLanguageEN, "english":
		return messageLanguageEN
	case messageLanguageZH, "cn", "chinese", "zh-cn", "zh_hans":
		return messageLanguageZH
	default:
		return messageLanguageZH
	}
}

func textsFor(language string) messageText {
	if text, ok := messageTexts[normalizeMessageLanguage(language)]; ok {
		return text
	}
	return messageTexts[messageLanguageZH]
}

func formatAlertTime(t time.Time, cfg MessageConfig) string {
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return t.In(cfg.Location).Format("2006-01-02 15:04:05 MST")
}

func isOfflineRule(rule CompiledRule) bool {
	return rule.Metric == "node.offline"
}
