package config

import (
	"fmt"
	"strings"
	"time"
)

const timezoneSyntaxHint = "IANA timezone name, for example Asia/Shanghai or UTC"

func compileLocation(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config: cfg is nil")
	}
	raw := strings.TrimSpace(cfg.App.Timezone)
	cfg.App.Timezone = raw
	if raw == "" {
		cfg.App.Location = time.Local
		return nil
	}
	loc, err := time.LoadLocation(raw)
	if err != nil {
		return fmt.Errorf(
			"config: parse app.timezone %q: expected %s: %w",
			raw,
			timezoneSyntaxHint,
			err,
		)
	}
	cfg.App.Location = loc
	return nil
}

func (c AppConfig) EffectiveLocation() *time.Location {
	if c.Location != nil {
		return c.Location
	}
	return time.Local
}
