package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ithildur/EiluneKit/appdir"

	"gopkg.in/yaml.v3"
)

const (
	EnvAdminPassword  = "monitor_dash_pwd"
	configNotFoundMsg = "config: no config file found (tried: config.local.yaml, config.yaml, configs/config.local.yaml, configs/config.yaml, $DASH_HOME/configs/*)"
)

// LoadRuntime loads runtime config for the selected Redis mode.
func LoadRuntime(path string, redisEnabled bool) (*Config, error) {
	var cfg Config

	resolved, err := resolveConfigPath(path)
	if err != nil {
		return nil, err
	}

	if err := readConfigFile(resolved, &cfg); err != nil {
		return nil, err
	}
	if err := overrideFromEnv(&cfg, redisEnabled); err != nil {
		return nil, err
	}
	compileLanguage(&cfg)
	if err := compileLocation(&cfg); err != nil {
		return nil, err
	}

	if err := compileDurations(&cfg, redisEnabled); err != nil {
		return nil, err
	}
	if err := compileHTTP(&cfg); err != nil {
		return nil, err
	}
	if err := compilePublicURL(&cfg); err != nil {
		return nil, err
	}
	if err := validateRuntime(&cfg, redisEnabled); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func resolveConfigPath(path string) (string, error) {
	if path == "" {
		path = firstConfigPath()
	}
	if path == "" {
		return "", errors.New(configNotFoundMsg)
	}
	return path, nil
}

func readConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: read file %q: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("config: parse file %q: %w", path, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("config: parse file %q: %w", path, err)
	}
	return fmt.Errorf("config: parse file %q: multiple YAML documents are not supported", path)
}

// LoadForMigrate loads config for migration only (database fields required).
func LoadForMigrate(path string) (*Config, error) {
	var cfg Config

	resolved, err := resolveConfigPath(path)
	if err != nil {
		return nil, err
	}

	if err := readConfigFile(resolved, &cfg); err != nil {
		return nil, err
	}
	if err := overrideFromEnv(&cfg, false); err != nil {
		return nil, err
	}
	compileLanguage(&cfg)
	if err := compileHTTP(&cfg); err != nil {
		return nil, err
	}

	if err := validateMigrate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func compileHTTP(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config: cfg is nil")
	}
	trusted, err := cfg.HTTP.EffectiveTrustedProxies()
	if err != nil {
		return fmt.Errorf("config: parse http.trusted_proxies: %w", err)
	}
	cfg.HTTP.TrustedProxyPrefixes = trusted
	return nil
}

func compileLanguage(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.App.Language = cfg.App.EffectiveLanguage()
}

func compileDurations(cfg *Config, redisEnabled bool) error {
	if cfg == nil {
		return fmt.Errorf("config: cfg is nil")
	}

	nodeOfflineThreshold, rawNodeOfflineThreshold, nodeOfflineThresholdErr := parseNodeOfflineThreshold(cfg.App.NodeOfflineThreshold)
	if nodeOfflineThresholdErr != nil {
		return fmt.Errorf(
			"config: parse app.node_offline_threshold %q (expected %s): %w",
			rawNodeOfflineThreshold,
			durationSyntaxHint,
			nodeOfflineThresholdErr,
		)
	}
	cfg.App.NodeOfflineThresholdDur = nodeOfflineThreshold

	d, specified, err := cfg.Database.EffectiveConnMaxLifetime()
	if err != nil {
		return fmt.Errorf("config: parse database.conn_max_lifetime: %w", err)
	}
	if specified {
		cfg.Database.ConnMaxLifetimeDur = d
	} else {
		cfg.Database.ConnMaxLifetimeDur = 0
	}

	if !redisEnabled || strings.TrimSpace(cfg.Redis.Addr) == "" {
		cfg.Redis.DialTimeoutDur = defaultRedisDialTimeout
		cfg.Redis.ReadTimeoutDur = defaultRedisReadTimeout
		cfg.Redis.WriteTimeoutDur = defaultRedisWriteTimeout
		return nil
	}

	redisDial, err := cfg.Redis.EffectiveDialTimeout()
	if err != nil {
		return fmt.Errorf("config: parse redis.dial_timeout: %w", err)
	}
	redisRead, err := cfg.Redis.EffectiveReadTimeout()
	if err != nil {
		return fmt.Errorf("config: parse redis.read_timeout: %w", err)
	}
	redisWrite, err := cfg.Redis.EffectiveWriteTimeout()
	if err != nil {
		return fmt.Errorf("config: parse redis.write_timeout: %w", err)
	}
	cfg.Redis.DialTimeoutDur = redisDial
	cfg.Redis.ReadTimeoutDur = redisRead
	cfg.Redis.WriteTimeoutDur = redisWrite

	return nil
}

func compilePublicURL(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config: cfg is nil")
	}

	raw := strings.TrimSpace(cfg.App.PublicURL)
	cfg.App.PublicURL = raw

	if raw == "" {
		cfg.App.PublicURLScheme = ""
		cfg.App.PublicURLHost = ""
		cfg.App.PublicURLBasePath = ""
		return nil
	}

	parsed := raw
	if !strings.Contains(parsed, "://") {
		parsed = publicURLWithDefaultScheme(parsed)
	}

	u, err := url.Parse(parsed)
	if err != nil {
		return fmt.Errorf("config: parse app.public_url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("config: app.public_url scheme must be http or https")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("config: app.public_url host is required")
	}
	if u.User != nil {
		return fmt.Errorf("config: app.public_url must not include user information")
	}
	if u.RawQuery != "" || u.ForceQuery {
		return fmt.Errorf("config: app.public_url must not include a query")
	}
	if u.Fragment != "" {
		return fmt.Errorf("config: app.public_url must not include a fragment")
	}

	cfg.App.PublicURLScheme = scheme
	cfg.App.PublicURLHost = u.Host

	path := u.EscapedPath()
	if path != "" && path != "/" {
		return fmt.Errorf("config: app.public_url must not include a path prefix (got %q)", path)
	}
	cfg.App.PublicURLBasePath = ""

	return nil
}

func publicURLWithDefaultScheme(raw string) string {
	scheme := defaultSchemeForPublicURL(raw)
	host, suffix := raw, ""
	if idx := strings.Index(raw, "/"); idx >= 0 {
		host, suffix = raw[:idx], raw[idx:]
	}
	if strings.Contains(host, ":") && net.ParseIP(host) != nil {
		host = "[" + host + "]"
	}
	return scheme + "://" + host + suffix
}

func defaultSchemeForPublicURL(raw string) string {
	host := raw
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if isIPHost(host) {
		return "http"
	}
	return "https"
}

func isIPHost(hostport string) bool {
	host := strings.TrimSpace(hostport)
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end > 0 {
			return net.ParseIP(host[1:end]) != nil
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return net.ParseIP(h) != nil
	}
	return net.ParseIP(host) != nil
}

func firstConfigPath() string {
	candidates := []string{
		"config.local.yaml",
		"config.yaml",
		filepath.Join("configs", "config.local.yaml"),
		filepath.Join("configs", "config.yaml"),
	}

	if home, err := appdir.DiscoverHome(DefaultAppDirOptions()); err == nil && home != "" {
		candidates = append(candidates,
			filepath.Join(home, "configs", "config.local.yaml"),
			filepath.Join(home, "configs", "config.yaml"),
		)
	}

	for _, p := range candidates {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func validateRuntime(cfg *Config, redisEnabled bool) error {
	missing := make([]string, 0, 8)
	missing = append(missing, requireRuntimeFields(cfg, redisEnabled)...)
	missing = append(missing, requireDatabaseFields(cfg)...)
	if err := validateMissing(missing); err != nil {
		return err
	}
	if err := validateJWTSigningKey(cfg.Auth.JWTSigningKey); err != nil {
		return err
	}
	if err := validateDatabasePool(cfg.Database); err != nil {
		return err
	}
	if cfg.Database.RetentionDays < 0 {
		return fmt.Errorf("config: database.retention_days must be >= 0")
	}
	if cfg.Database.TrafficRetentionDays < 0 {
		return fmt.Errorf("config: database.traffic_retention_days must be >= 0")
	}
	return nil
}

func validateJWTSigningKey(key string) error {
	if key != strings.TrimSpace(key) {
		return fmt.Errorf("config: auth.jwt_signing_key must not have surrounding whitespace")
	}
	if len(key) < 32 {
		return fmt.Errorf("config: auth.jwt_signing_key must be at least 32 bytes")
	}
	return nil
}

func validateMigrate(cfg *Config) error {
	missing := requireDatabaseFields(cfg)
	if err := validateMissing(missing); err != nil {
		return err
	}
	if err := validateDatabasePool(cfg.Database); err != nil {
		return err
	}
	if cfg.Database.RetentionDays < 0 {
		return fmt.Errorf("config: database.retention_days must be >= 0")
	}
	if cfg.Database.TrafficRetentionDays < 0 {
		return fmt.Errorf("config: database.traffic_retention_days must be >= 0")
	}
	return nil
}

func validateDatabasePool(cfg DatabaseConfig) error {
	if cfg.MaxOpenConns < 0 {
		return fmt.Errorf("config: database.max_open_conns must be >= 0")
	}
	if cfg.MaxIdleConns < 0 {
		return fmt.Errorf("config: database.max_idle_conns must be >= 0")
	}
	if cfg.MaxOpenConns > 0 && cfg.MaxIdleConns > cfg.MaxOpenConns {
		return fmt.Errorf("config: database.max_idle_conns must be <= database.max_open_conns when max_open_conns is positive")
	}
	d, specified, err := cfg.EffectiveConnMaxLifetime()
	if err != nil {
		return fmt.Errorf("config: parse database.conn_max_lifetime: %w", err)
	}
	if specified && d < 0 {
		return fmt.Errorf("config: database.conn_max_lifetime must be >= 0")
	}
	return nil
}

func requireRuntimeFields(cfg *Config, redisEnabled bool) []string {
	var missing []string
	if cfg.App.Listen == "" {
		missing = append(missing, "app.listen")
	}
	if strings.TrimSpace(cfg.App.PublicURL) == "" {
		missing = append(missing, "app.public_url")
	}
	if strings.TrimSpace(cfg.Auth.JWTSigningKey) == "" {
		missing = append(missing, "auth.jwt_signing_key")
	}
	if redisEnabled && strings.TrimSpace(cfg.Redis.Addr) == "" {
		missing = append(missing, "redis.addr")
	}
	return missing
}

func requireDatabaseFields(cfg *Config) []string {
	var missing []string
	if cfg.Database.Driver == "" {
		missing = append(missing, "database.driver")
	}
	if cfg.Database.Host == "" {
		missing = append(missing, "database.host")
	}
	if cfg.Database.Port == 0 {
		missing = append(missing, "database.port")
	}
	if cfg.Database.User == "" {
		missing = append(missing, "database.user")
	}
	if cfg.Database.Name == "" {
		missing = append(missing, "database.name")
	}
	return missing
}

func validateMissing(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("config: missing required fields: %s", strings.Join(missing, ", "))
}

func envString(key string, dst *string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = v
	}
}

func envInt(key string, dst *int) error {
	v, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) == "" {
		*dst = 0
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("config: parse environment %s as integer: %w", key, err)
	}
	*dst = n
	return nil
}

func overrideFromEnv(cfg *Config, redisEnabled bool) error {
	// app
	envString("APP_NAME", &cfg.App.Name)
	envString("APP_ENV", &cfg.App.Env)
	envString("APP_DASH_IP", &cfg.App.DashIP)
	envString("APP_LISTEN", &cfg.App.Listen)
	envString("APP_GRPC_PORT", &cfg.App.GRPCPort)
	envString("APP_PUBLIC_URL", &cfg.App.PublicURL)
	envString("APP_TIMEZONE", &cfg.App.Timezone)
	envString("APP_LANGUAGE", &cfg.App.Language)
	envString("APP_LOG_LEVEL", &cfg.App.LogLevel)
	envString("APP_LOG_FORMAT", &cfg.App.LogFormat)
	envString("APP_NODE_OFFLINE_THRESHOLD", &cfg.App.NodeOfflineThreshold)

	// database
	envString("DB_DRIVER", &cfg.Database.Driver)
	envString("DB_HOST", &cfg.Database.Host)
	if err := envInt("DB_PORT", &cfg.Database.Port); err != nil {
		return err
	}
	envString("DB_USER", &cfg.Database.User)
	envString("DB_PASSWORD", &cfg.Database.Password)
	envString("DB_NAME", &cfg.Database.Name)
	envString("DB_SSLMODE", &cfg.Database.SSLMode)
	if err := envInt("DB_MAX_OPEN_CONNS", &cfg.Database.MaxOpenConns); err != nil {
		return err
	}
	if err := envInt("DB_MAX_IDLE_CONNS", &cfg.Database.MaxIdleConns); err != nil {
		return err
	}
	envString("DB_CONN_MAX_LIFETIME", &cfg.Database.ConnMaxLifetime)
	if err := envInt("DB_RETENTION_DAYS", &cfg.Database.RetentionDays); err != nil {
		return err
	}
	if err := envInt("DB_TRAFFIC_RETENTION_DAYS", &cfg.Database.TrafficRetentionDays); err != nil {
		return err
	}

	if redisEnabled {
		// redis
		envString("REDIS_ADDR", &cfg.Redis.Addr)
		envString("REDIS_USERNAME", &cfg.Redis.Username)
		envString("REDIS_PASSWORD", &cfg.Redis.Password)
		if err := envInt("REDIS_DB", &cfg.Redis.DB); err != nil {
			return err
		}
		if err := envInt("REDIS_POOL_SIZE", &cfg.Redis.PoolSize); err != nil {
			return err
		}
		if err := envInt("REDIS_MIN_IDLE_CONNS", &cfg.Redis.MinIdleConns); err != nil {
			return err
		}
		envString("REDIS_DIAL_TIMEOUT", &cfg.Redis.DialTimeout)
		envString("REDIS_READ_TIMEOUT", &cfg.Redis.ReadTimeout)
		envString("REDIS_WRITE_TIMEOUT", &cfg.Redis.WriteTimeout)
	}

	// auth (env only)
	envString(EnvAdminPassword, &cfg.Auth.Password)
	return nil
}
