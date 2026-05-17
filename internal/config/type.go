package config

import (
	"net/netip"
	"time"
)

// AppConfig holds application-level settings.
type AppConfig struct {
	Name                 string `yaml:"name"`
	Env                  string `yaml:"env"`
	DashIP               string `yaml:"dash_ip"`
	Listen               string `yaml:"listen"`
	GRPCPort             string `yaml:"grpc_port"`
	PublicURL            string `yaml:"public_url"`
	Timezone             string `yaml:"timezone"`
	Language             string `yaml:"language"`
	LogLevel             string `yaml:"log_level"`
	LogFormat            string `yaml:"log_format"`
	NodeOfflineThreshold string `yaml:"node_offline_threshold"`

	// Derived fields (compiled in config.Load). Not part of YAML.
	PublicURLScheme         string        `yaml:"-"`
	PublicURLHost           string        `yaml:"-"`
	PublicURLBasePath       string        `yaml:"-"`
	NodeOfflineThresholdDur time.Duration `yaml:"-"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Driver               string `yaml:"driver"`
	Host                 string `yaml:"host"`
	Port                 int    `yaml:"port"`
	User                 string `yaml:"user"`
	Password             string `yaml:"password"`
	Name                 string `yaml:"name"`
	SSLMode              string `yaml:"sslmode"`
	MaxOpenConns         int    `yaml:"max_open_conns"`
	MaxIdleConns         int    `yaml:"max_idle_conns"`
	ConnMaxLifetime      string `yaml:"conn_max_lifetime"`
	RetentionDays        int    `yaml:"retention_days"`
	TrafficRetentionDays int    `yaml:"traffic_retention_days"`

	ConnMaxLifetimeDur time.Duration `yaml:"-"`
}

// RedisConfig holds redis settings.
type RedisConfig struct {
	Addr         string `yaml:"addr"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`

	DialTimeoutDur  time.Duration `yaml:"-"`
	ReadTimeoutDur  time.Duration `yaml:"-"`
	WriteTimeoutDur time.Duration `yaml:"-"`
}

// HTTPConfig holds HTTP proxy trust settings.
type HTTPConfig struct {
	TrustedProxies       []string       `yaml:"trusted_proxies"`
	TrustedProxyPrefixes []netip.Prefix `yaml:"-"`
}

// AuthConfig holds admin auth settings (env-only).
type AuthConfig struct {
	// JWTSigningKey is the HS256 signing key for issued JWT tokens.
	// It must be a high-entropy secret and should be set via config file (install script generates it).
	JWTSigningKey string `yaml:"jwt_signing_key"`

	// Password is the admin login password; env-only (monitor_dash_pwd).
	Password string `yaml:"-"`
}

// Config is the top-level application config.
type Config struct {
	App      AppConfig      `yaml:"app"`
	HTTP     HTTPConfig     `yaml:"http"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Auth     AuthConfig     `yaml:"auth"`
}
