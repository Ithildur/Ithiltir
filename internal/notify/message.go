package notify

import (
	"fmt"
	"strings"
	"time"
)

type Message struct {
	Title    string
	Body     string
	Metadata map[string]string
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

func DefaultTestMessage() Message {
	now := time.Now().UTC().Format(time.RFC3339)
	return Message{
		Title: "通知测试",
		Body:  "这是一条测试消息，发送时间: " + now,
	}
}

func TelegramBotExampleMessages() []Message {
	now := time.Now().Local().Format("2006-01-02 15:04:05 MST")
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
