package config

const redactedValue = "***"

func redactIfSet(v string) string {
	if v == "" {
		return ""
	}
	return redactedValue
}

// RedactedForLog returns a copy of cfg with sensitive fields redacted for safe logging.
func RedactedForLog(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	c := *cfg
	c.Database.Password = redactIfSet(c.Database.Password)
	c.Redis.Password = redactIfSet(c.Redis.Password)

	// Auth is env-only and not marshaled from YAML, but redact anyway for safety.
	c.Auth.Password = redactIfSet(c.Auth.Password)
	c.Auth.JWTSigningKey = redactIfSet(c.Auth.JWTSigningKey)

	return &c
}
