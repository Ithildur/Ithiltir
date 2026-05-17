package config

import "testing"

func TestAppLanguageEffectiveValue(t *testing.T) {
	tests := map[string]string{
		"":        "zh",
		"en":      "en",
		"english": "en",
		"zh":      "zh",
		"cn":      "zh",
		"ja":      "zh",
	}

	for raw, want := range tests {
		if got := (AppConfig{Language: raw}).EffectiveLanguage(); got != want {
			t.Fatalf("EffectiveLanguage(%q) = %q, want %q", raw, got, want)
		}
	}
}
