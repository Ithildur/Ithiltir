package config

import (
	"errors"
	"strings"
	"time"
)

const durationSyntaxHint = "Go duration syntax, e.g. 14s, 2m, 1h"

var errDurationNotPositive = errors.New("duration must be positive")

func (c AppConfig) EffectiveNodeOfflineThreshold() time.Duration {
	if c.NodeOfflineThresholdDur > 0 {
		return c.NodeOfflineThresholdDur
	}
	d, _, err := parseNodeOfflineThreshold(c.NodeOfflineThreshold)
	if err != nil {
		return DefaultNodeOfflineThreshold
	}
	return d
}

func parseNodeOfflineThreshold(raw string) (time.Duration, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultNodeOfflineThreshold, raw, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return DefaultNodeOfflineThreshold, raw, err
	}
	if d <= 0 {
		return DefaultNodeOfflineThreshold, raw, errDurationNotPositive
	}
	return d, raw, nil
}

// EffectiveConnMaxLifetime returns (duration, specified, error).
// specified is true when ConnMaxLifetime is explicitly set (even to 0).
func (c DatabaseConfig) EffectiveConnMaxLifetime() (time.Duration, bool, error) {
	raw := strings.TrimSpace(c.ConnMaxLifetime)
	if raw == "" {
		return 0, false, nil
	}
	if c.ConnMaxLifetimeDur != 0 {
		return c.ConnMaxLifetimeDur, true, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, true, err
	}
	return d, true, nil
}

func (c DatabaseConfig) EffectiveRetentionDays() int {
	if c.RetentionDays > 0 {
		return c.RetentionDays
	}
	return DefaultRetentionDays
}

func (c DatabaseConfig) EffectiveTrafficRetentionDays() int {
	if c.TrafficRetentionDays > 0 {
		return c.TrafficRetentionDays
	}
	days := c.EffectiveRetentionDays()
	if days < DefaultTrafficRetentionDays {
		return DefaultTrafficRetentionDays
	}
	return days
}

func (c RedisConfig) EffectiveDialTimeout() (time.Duration, error) {
	return effectiveDurationWithDefault(c.DialTimeout, c.DialTimeoutDur, defaultRedisDialTimeout)
}

func (c RedisConfig) EffectiveReadTimeout() (time.Duration, error) {
	return effectiveDurationWithDefault(c.ReadTimeout, c.ReadTimeoutDur, defaultRedisReadTimeout)
}

func (c RedisConfig) EffectiveWriteTimeout() (time.Duration, error) {
	return effectiveDurationWithDefault(c.WriteTimeout, c.WriteTimeoutDur, defaultRedisWriteTimeout)
}

func effectiveDurationWithDefault(raw string, compiled time.Duration, def time.Duration) (time.Duration, error) {
	if compiled > 0 {
		return compiled, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return def, nil
	}
	return d, nil
}
