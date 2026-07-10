package alert

import (
	"time"

	"dash/internal/lang"
)

const (
	messageLanguageZH = lang.Chinese
	messageLanguageEN = lang.English
)

type MessageConfig struct {
	Language string
	Location *time.Location
}

type messageText struct {
	openTitle              string
	openBody               string
	reasonLine             string
	closeTitle             string
	closeBody              string
	offlineOpenTitle       string
	offlineOpenBody        string
	offlineCloseTitle      string
	offlineCloseBody       string
	unknownSmartDevice     string
	smartTitleMore         string
	smartDetailMore        string
	smartHealthFailed      string
	smartFailingAttrs      string
	nvmeWarning            string
	nvmeMediaErrors        string
	nvmeUnknownWarningBits string
	separator              string
	detailJoiner           string
}

var messageTexts = map[string]messageText{
	messageLanguageZH: {
		openTitle:              "告警触发: %s @ %s",
		openBody:               "状态: opened\n服务器: %s\n规则: %s\n指标: %s%s\n当前值: %s\n阈值: %s\n持续时间: %ds\n触发时间: %s",
		reasonLine:             "\n原因: %s",
		closeTitle:             "告警恢复: %s @ %s",
		closeBody:              "状态: closed\n服务器: %s\n规则: %s\n指标: %s\n当前值: %s\n恢复时间: %s",
		offlineOpenTitle:       "离线告警：%s",
		offlineOpenBody:        "触发时间: %s",
		offlineCloseTitle:      "恢复在线：%s",
		offlineCloseBody:       "恢复时间: %s",
		unknownSmartDevice:     "未知设备",
		smartTitleMore:         "%s，另有 %d 个",
		smartDetailMore:        "另有 %d 个设备",
		smartHealthFailed:      "健康检查失败",
		smartFailingAttrs:      "失败属性: %s",
		nvmeWarning:            "关键警告 0x%02X",
		nvmeMediaErrors:        "0E 媒体/数据完整性错误: %d",
		nvmeUnknownWarningBits: "未知位 0x%X",
		separator:              "：",
		detailJoiner:           "；",
	},
	messageLanguageEN: {
		openTitle:              "Alert triggered: %s @ %s",
		openBody:               "Status: opened\nServer: %s\nRule: %s\nMetric: %s%s\nCurrent value: %s\nThreshold: %s\nDuration: %ds\nTriggered at: %s",
		reasonLine:             "\nReason: %s",
		closeTitle:             "Alert recovered: %s @ %s",
		closeBody:              "Status: closed\nServer: %s\nRule: %s\nMetric: %s\nCurrent value: %s\nRecovered at: %s",
		offlineOpenTitle:       "Offline alert: %s",
		offlineOpenBody:        "Triggered at: %s",
		offlineCloseTitle:      "Online restored: %s",
		offlineCloseBody:       "Recovered at: %s",
		unknownSmartDevice:     "unknown device",
		smartTitleMore:         "%s, +%d more",
		smartDetailMore:        "%d more devices",
		smartHealthFailed:      "health check failed",
		smartFailingAttrs:      "failing attributes: %s",
		nvmeWarning:            "critical warning 0x%02X",
		nvmeMediaErrors:        "0E media/data integrity errors: %d",
		nvmeUnknownWarningBits: "unknown bits 0x%X",
		separator:              ": ",
		detailJoiner:           "; ",
	},
}

func messageConfig(configs []MessageConfig) MessageConfig {
	cfg := MessageConfig{Language: messageLanguageZH, Location: time.Local}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	cfg.Language = lang.Normalize(cfg.Language)
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	return cfg
}

func textsFor(raw string) messageText {
	if text, ok := messageTexts[lang.Normalize(raw)]; ok {
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
