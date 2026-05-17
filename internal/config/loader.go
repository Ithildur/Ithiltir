package config

import (
	"fmt"
	"log/slog"
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

// Warning captures non-fatal config issues for upper layers to log.
type Warning struct {
	Msg   string
	Err   error
	Attrs []slog.Attr
}

type warningCollector struct {
	items []Warning
}

func (c *warningCollector) add(msg string, err error, attrs ...slog.Attr) {
	if c == nil {
		return
	}
	c.items = append(c.items, Warning{
		Msg:   msg,
		Err:   err,
		Attrs: attrs,
	})
}

func Load(path string) (*Config, error) {
	cfg, _, err := LoadWithWarnings(path)
	return cfg, err
}

// LoadWithWarnings loads config and returns any non-fatal warnings.
func LoadWithWarnings(path string) (*Config, []Warning, error) {
	var cfg Config

	resolved, err := resolveConfigPath(path)
	if err != nil {
		return nil, nil, err
	}

	if err := readConfigFile(resolved, &cfg); err != nil {
		return nil, nil, err
	}
	warns := warningCollector{}
	overrideFromEnv(&cfg, &warns)
	compileLanguage(&cfg)

	if err := compileDurations(&cfg, &warns); err != nil {
		return nil, nil, err
	}
	if err := compileHTTP(&cfg); err != nil {
		return nil, nil, err
	}
	if err := compilePublicURL(&cfg); err != nil {
		return nil, nil, err
	}
	if err := validateRuntime(&cfg); err != nil {
		return nil, nil, err
	}

	return &cfg, warns.items, nil
}

func resolveConfigPath(path string) (string, error) {
	if path == "" {
		path = firstConfigPath()
	}
	if path == "" {
		return "", fmt.Errorf(configNotFoundMsg)
	}
	return path, nil
}

func readConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: read file %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("config: parse file %q: %w", path, err)
	}
	return nil
}

// LoadForMigrate loads config for migration only (database fields required).
func LoadForMigrate(path string) (*Config, error) {
	cfg, _, err := LoadForMigrateWithWarnings(path)
	return cfg, err
}

// LoadForMigrateWithWarnings loads config for migration and returns any non-fatal warnings.
func LoadForMigrateWithWarnings(path string) (*Config, []Warning, error) {
	var cfg Config

	resolved, err := resolveConfigPath(path)
	if err != nil {
		return nil, nil, err
	}

	if err := readConfigFile(resolved, &cfg); err != nil {
		return nil, nil, err
	}
	warns := warningCollector{}
	overrideFromEnv(&cfg, &warns)
	compileLanguage(&cfg)
	if err := compileHTTP(&cfg); err != nil {
		return nil, nil, err
	}

	if err := validateMigrate(&cfg); err != nil {
		return nil, nil, err
	}

	return &cfg, warns.items, nil
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

func compileDurations(cfg *Config, warns *warningCollector) error {
	if cfg == nil {
		return fmt.Errorf("config: cfg is nil")
	}

	nodeOfflineThreshold, rawNodeOfflineThreshold, nodeOfflineThresholdErr := parseNodeOfflineThreshold(cfg.App.NodeOfflineThreshold)
	cfg.App.NodeOfflineThresholdDur = nodeOfflineThreshold
	if nodeOfflineThresholdErr != nil {
		warns.add("config invalid duration, fallback to default",
			nodeOfflineThresholdErr,
			slog.String("key", "app.node_offline_threshold"),
			slog.String("value", rawNodeOfflineThreshold),
			slog.String("expected", durationSyntaxHint),
			slog.String("default", DefaultNodeOfflineThreshold.String()),
		)
	}

	d, specified, err := cfg.Database.EffectiveConnMaxLifetime()
	if err != nil {
		return fmt.Errorf("config: parse database.conn_max_lifetime: %w", err)
	}
	if specified {
		cfg.Database.ConnMaxLifetimeDur = d
	} else {
		cfg.Database.ConnMaxLifetimeDur = 0
	}

	if strings.TrimSpace(cfg.Redis.Addr) == "" {
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
		parsed = defaultSchemeForPublicURL(parsed) + "://" + parsed
	}

	u, err := url.Parse(parsed)
	if err != nil {
		return fmt.Errorf("config: parse app.public_url: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("config: app.public_url scheme is required")
	}
	if u.Host == "" {
		return fmt.Errorf("config: app.public_url host is required")
	}

	cfg.App.PublicURLScheme = u.Scheme
	cfg.App.PublicURLHost = u.Host

	basePath := strings.TrimSuffix(u.Path, "/")
	if strings.HasPrefix(basePath, "/") && basePath != "/" {
		return fmt.Errorf("config: app.public_url must not include a path prefix (got %q)", basePath)
	}
	cfg.App.PublicURLBasePath = ""

	return nil
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

func validateRuntime(cfg *Config) error {
	missing := make([]string, 0, 8)
	missing = append(missing, requireRuntimeFields(cfg)...)
	missing = append(missing, requireDatabaseFields(cfg)...)
	if err := validateMissing(missing); err != nil {
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

func validateMigrate(cfg *Config) error {
	missing := requireDatabaseFields(cfg)
	if err := validateMissing(missing); err != nil {
		return err
	}
	if cfg.Database.RetentionDays < 0 {
		return fmt.Errorf("config: database.retention_days must be >= 0")
	}
	if cfg.Database.TrafficRetentionDays < 0 {
		return fmt.Errorf("config: database.traffic_retention_days must be >= 0")
	}
	if _, _, err := cfg.Database.EffectiveConnMaxLifetime(); err != nil {
		return fmt.Errorf("config: parse database.conn_max_lifetime: %w", err)
	}
	return nil
}

func requireRuntimeFields(cfg *Config) []string {
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
	if v, ok := os.LookupEnv(key); ok && v != "" {
		*dst = v
	}
}

func envInt(key string, dst *int, warns *warningCollector) {
	if v, ok := os.LookupEnv(key); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			warns.add("config invalid int", err, slog.String("key", key), slog.String("value", v))
			return
		}
		*dst = n
	}
}

func envBool(key string, dst *bool, warns *warningCollector) {
	if v, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			warns.add("config invalid bool", err, slog.String("key", key), slog.String("value", v))
			return
		}
		*dst = b
	}
}

func overrideFromEnv(cfg *Config, warns *warningCollector) {
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
	envInt("DB_PORT", &cfg.Database.Port, warns)
	envString("DB_USER", &cfg.Database.User)
	envString("DB_PASSWORD", &cfg.Database.Password)
	envString("DB_NAME", &cfg.Database.Name)
	envString("DB_SSLMODE", &cfg.Database.SSLMode)
	envInt("DB_MAX_OPEN_CONNS", &cfg.Database.MaxOpenConns, warns)
	envInt("DB_MAX_IDLE_CONNS", &cfg.Database.MaxIdleConns, warns)
	envString("DB_CONN_MAX_LIFETIME", &cfg.Database.ConnMaxLifetime)
	envInt("DB_RETENTION_DAYS", &cfg.Database.RetentionDays, warns)
	envInt("DB_TRAFFIC_RETENTION_DAYS", &cfg.Database.TrafficRetentionDays, warns)

	// redis
	envString("REDIS_ADDR", &cfg.Redis.Addr)
	envString("REDIS_USERNAME", &cfg.Redis.Username)
	envString("REDIS_PASSWORD", &cfg.Redis.Password)
	envInt("REDIS_DB", &cfg.Redis.DB, warns)
	envInt("REDIS_POOL_SIZE", &cfg.Redis.PoolSize, warns)
	envInt("REDIS_MIN_IDLE_CONNS", &cfg.Redis.MinIdleConns, warns)
	envString("REDIS_DIAL_TIMEOUT", &cfg.Redis.DialTimeout)
	envString("REDIS_READ_TIMEOUT", &cfg.Redis.ReadTimeout)
	envString("REDIS_WRITE_TIMEOUT", &cfg.Redis.WriteTimeout)

	// auth (env only)
	envString(EnvAdminPassword, &cfg.Auth.Password)
}
