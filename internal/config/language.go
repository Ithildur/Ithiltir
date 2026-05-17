package config

import (
	"strings"
	"time"
)

const (
	LanguageEnglish = "en"
	LanguageChinese = "zh"
)

func (c AppConfig) EffectiveLanguage() string {
	switch strings.ToLower(strings.TrimSpace(c.Language)) {
	case LanguageEnglish, "english":
		return LanguageEnglish
	case LanguageChinese, "cn", "chinese", "zh-cn", "zh_hans":
		return LanguageChinese
	default:
		return LanguageChinese
	}
}

func (c AppConfig) EffectiveLocation() *time.Location {
	raw := strings.TrimSpace(c.Timezone)
	if raw == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(raw)
	if err != nil {
		return time.Local
	}
	return loc
}
