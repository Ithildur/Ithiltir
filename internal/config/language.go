package config

import (
	"strings"
	"time"

	"dash/internal/lang"
)

const (
	LanguageEnglish = lang.English
	LanguageChinese = lang.Chinese
)

func (c AppConfig) EffectiveLanguage() string {
	return lang.Normalize(c.Language)
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
