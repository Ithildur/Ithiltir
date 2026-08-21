package lang

import "strings"

const (
	System  = "system"
	Chinese = "zh"
	English = "en"
)

func Normalize(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case English, "english":
		return English
	case Chinese, "cn", "chinese", "zh-cn", "zh_hans":
		return Chinese
	default:
		return Chinese
	}
}

func Resolve(preferred, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(preferred)) {
	case English:
		return English
	case Chinese:
		return Chinese
	default:
		return Normalize(fallback)
	}
}
