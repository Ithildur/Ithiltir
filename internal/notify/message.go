package notify

import (
	"fmt"
	"strings"
	"time"

	"dash/internal/lang"
)

type Message struct {
	Title    string
	Body     string
	Metadata map[string]string
}

type Messages struct {
	Chinese Message
	English Message
}

func (m Messages) For(language string) Message {
	if lang.Normalize(language) == lang.English {
		return m.English
	}
	return m.Chinese
}

func (m Message) Text() string {
	title := strings.TrimSpace(m.Title)
	body := strings.TrimSpace(m.Body)
	switch {
	case title == "":
		return body
	case body == "":
		return title
	default:
		return fmt.Sprintf("%s\n%s", title, body)
	}
}

func DefaultTestMessage(language string) Message {
	now := time.Now().UTC().Format(time.RFC3339)
	if lang.Normalize(language) == lang.English {
		return Message{
			Title: "Notification test",
			Body:  "This is a test alert from Ithiltir. Sent at: " + now,
		}
	}
	return Message{
		Title: "通知测试",
		Body:  "这是一条测试消息，发送时间: " + now,
	}
}

func TelegramBotExampleMessages(language string) []Message {
	now := time.Now().Local().Format("2006-01-02 15:04:05 MST")
	if lang.Normalize(language) == lang.English {
		return []Message{
			{
				Title: "Alert test example",
				Body: strings.Join([]string{
					"❌ Alert triggered: High CPU usage @ 9900x",
					"Status: opened",
					"Server: 9900x",
					"Rule: High CPU usage",
					"Metric: cpu.usage_ratio",
					"Current value: 92.00%",
					"Threshold: 90.00%",
					"Duration: 60s",
					"🕒 Triggered at: " + now,
					"",
					"❌ Offline alert: 9900x",
					"🕒 Triggered at: " + now,
				}, "\n"),
			},
			{
				Title: "Recovery test example",
				Body: strings.Join([]string{
					"✅ Alert recovered: High CPU usage @ 9900x",
					"Status: closed",
					"Server: 9900x",
					"Rule: High CPU usage",
					"Metric: cpu.usage_ratio",
					"Current value: 42.00%",
					"🕒 Recovered at: " + now,
					"",
					"✅ Online restored: 9900x",
					"🕒 Recovered at: " + now,
				}, "\n"),
			},
		}
	}
	return []Message{
		{
			Title: "告警测试示例",
			Body: strings.Join([]string{
				"❌ 告警触发: CPU 使用率过高 @ 9900x",
				"状态: opened",
				"服务器: 9900x",
				"规则: CPU 使用率过高",
				"指标: cpu.usage_ratio",
				"当前值: 92.00%",
				"阈值: 90.00%",
				"持续时间: 60s",
				"🕒 触发时间: " + now,
				"",
				"❌ 离线告警：9900x",
				"🕒 触发时间: " + now,
			}, "\n"),
		},
		{
			Title: "恢复测试示例",
			Body: strings.Join([]string{
				"✅ 告警恢复: CPU 使用率过高 @ 9900x",
				"状态: closed",
				"服务器: 9900x",
				"规则: CPU 使用率过高",
				"指标: cpu.usage_ratio",
				"当前值: 42.00%",
				"🕒 恢复时间: " + now,
				"",
				"✅ 恢复在线：9900x",
				"🕒 恢复时间: " + now,
			}, "\n"),
		},
	}
}
