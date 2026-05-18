package lang

import "strings"

const (
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
