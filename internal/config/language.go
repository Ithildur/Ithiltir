package config

import "dash/internal/lang"

const (
	LanguageEnglish = lang.English
	LanguageChinese = lang.Chinese
)

func (c AppConfig) EffectiveLanguage() string {
	return lang.Normalize(c.Language)
}
