package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
	"dash/internal/migrate"
	"dash/internal/notify"

	kitmigration "github.com/Ithildur/EiluneKit/postgres/migration"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"
)

const EnvDatabaseURL = "TEST_DATABASE_URL"

func NewDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, _ := newDB(t, nil)
	return db
}

// NewDBAt creates a fresh database migrated through target and returns the key
// path needed to advance it through the notification-config Go migration.
func NewDBAt(t testing.TB, target int64) (*gorm.DB, string) {
	t.Helper()
	return newDB(t, &target)
}

func ConfigCipher(t testing.TB) *notify.ConfigCipher {
	t.Helper()
	configCipher, err := notify.NewConfigCipher(make([]byte, notify.ConfigKeySize))
	if err != nil {
		t.Fatalf("create test notification config cipher: %v", err)
	}
	return configCipher
}

func newDB(t testing.TB, target *int64) (*gorm.DB, string) {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv(EnvDatabaseURL))
	if raw == "" {
		t.Skipf("%s is not set", EnvDatabaseURL)
	}

	name := databaseName()
	adminURL, targetURL, err := databaseURLs(raw, name)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvDatabaseURL, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := createDatabase(ctx, adminURL, name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	var db *gorm.DB
	t.Cleanup(func() {
		closeDB(db)
		dropDatabase(t, adminURL, name)
	})

	cfg, err := databaseConfig(targetURL)
	if err != nil {
		t.Fatalf("build test database config: %v", err)
	}
	db, err = infra.NewGORMTimescale(ctx, cfg)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "notify-config.key")
	if err := os.WriteFile(keyPath, make([]byte, notify.ConfigKeySize), 0o600); err != nil {
		t.Fatalf("create test notification config key: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("extract sql.DB: %v", err)
	}
	migrationCfg := migrate.New(sqlDB, keyPath)
	if target == nil {
		_, err = kitmigration.Run(ctx, migrationCfg)
	} else {
		_, err = kitmigration.RunTo(ctx, migrationCfg, *target)
	}
	if err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db, keyPath
}

func databaseName() string {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	return "ithiltir_test_" + id[:16]
}

func databaseURLs(raw, name string) (string, string, error) {
	admin, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if admin.Scheme != "postgres" && admin.Scheme != "postgresql" {
		return "", "", fmt.Errorf("database URL must use postgres or postgresql scheme")
	}
	withDefaultSSLMode(admin)

	target := *admin
	target.Path = "/" + name
	target.RawPath = ""
	withDefaultSSLMode(&target)

	return admin.String(), target.String(), nil
}

func withDefaultSSLMode(u *url.URL) {
	q := u.Query()
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
	}
}

func databaseConfig(raw string) (config.DatabaseConfig, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return config.DatabaseConfig{}, err
	}

	port := 5432
	if rawPort := u.Port(); rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil {
			return config.DatabaseConfig{}, fmt.Errorf("invalid port %q: %w", rawPort, err)
		}
		port = parsed
	}

	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return config.DatabaseConfig{}, fmt.Errorf("database name is empty")
	}

	password, _ := u.User.Password()
	sslMode := u.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}

	return config.DatabaseConfig{
		Driver:       "postgres",
		Host:         u.Hostname(),
		Port:         port,
		User:         u.User.Username(),
		Password:     password,
		Name:         name,
		SSLMode:      sslMode,
		MaxOpenConns: 4,
		MaxIdleConns: 2,
	}, nil
}

func createDatabase(ctx context.Context, adminURL, name string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	return err
}

func closeDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

func dropDatabase(t testing.TB, adminURL, name string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Errorf("open admin database for cleanup: %v", err)
		return
	}
	defer conn.Close(ctx)

	_, _ = conn.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", name)
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Errorf("drop test database %s: %v", name, err)
	}
}
