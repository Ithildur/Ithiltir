package migrate_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"testing"

	"dash/internal/migrate"
	"dash/internal/model"
	"dash/internal/notify"
	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationNotifyConfigMigrationRemovesPlaintextAndDoesNotReplaceLostKey(t *testing.T) {
	ctx := context.Background()
	db, keyPath := pgtest.NewDBAt(t, 10)
	var id int64
	if err := db.Raw(`
		INSERT INTO notify_channels (name, type, config, enabled, is_deleted)
		VALUES ('legacy', 'webhook', '{"url":"https://example.test/hook","secret":"stored-secret"}'::jsonb, TRUE, FALSE)
		RETURNING id
	`).Scan(&id).Error; err != nil {
		t.Fatalf("create legacy notification channel: %v", err)
	}

	result, err := migrate.Run(ctx, db, keyPath)
	if err != nil {
		t.Fatalf("run notification config encryption migration: %v", err)
	}
	if result.Applied != 1 {
		t.Fatalf("applied migrations = %d, want 1", result.Applied)
	}
	configCipher, err := notify.LoadConfigCipher(keyPath)
	if err != nil {
		t.Fatalf("LoadConfigCipher() error = %v", err)
	}
	if err := migrate.CheckNotifyConfigs(ctx, db, configCipher); err != nil {
		t.Fatalf("CheckNotifyConfigs() error = %v", err)
	}

	var row struct {
		Config       string `gorm:"column:config"`
		ConfigSealed []byte `gorm:"column:config_sealed"`
	}
	if err := db.Raw(`
		SELECT config::text AS config, config_sealed
		FROM notify_channels
		WHERE id = ?
	`, id).Scan(&row).Error; err != nil {
		t.Fatalf("load encrypted notification config: %v", err)
	}
	if row.Config != "{}" || len(row.ConfigSealed) == 0 {
		t.Fatalf("stored config = %q sealed_bytes=%d, want empty legacy config and ciphertext", row.Config, len(row.ConfigSealed))
	}
	plain, err := configCipher.Open(id, model.NotifyTypeWebhook, row.ConfigSealed)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(plain, &decoded); err != nil {
		t.Fatalf("decode decrypted config: %v", err)
	}
	if decoded["secret"] != "stored-secret" {
		t.Fatalf("decrypted secret = %q, want stored-secret", decoded["secret"])
	}
	if err := db.Exec(`
		UPDATE notify_channels
		SET config = '{"secret":"plaintext-regression"}'::jsonb
		WHERE id = ?
	`, id).Error; err == nil {
		t.Fatal("plaintext notification config update succeeded after migration")
	}

	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("remove notification config key: %v", err)
	}
	if _, err := migrate.Run(ctx, db, keyPath); err != nil {
		t.Fatalf("rerun current migration after key loss: %v", err)
	}
	if _, err := os.Stat(keyPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("lost notification config key was recreated: %v", err)
	}
}

func TestIntegrationNotifyConfigMigrationRecordsVersionAfterSealing(t *testing.T) {
	ctx := context.Background()
	db, keyPath := pgtest.NewDBAt(t, 10)
	if err := db.Exec(`
		INSERT INTO notify_channels (name, type, config, enabled, is_deleted)
		VALUES ('legacy', 'webhook', '{"url":"https://example.test/hook"}'::jsonb, TRUE, FALSE)
	`).Error; err != nil {
		t.Fatalf("create legacy notification channel: %v", err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("remove test notification config key: %v", err)
	}
	if err := os.Mkdir(keyPath, 0o700); err != nil {
		t.Fatalf("replace notification config key with directory: %v", err)
	}
	if _, err := migrate.Run(ctx, db, keyPath); err == nil {
		t.Fatal("Run() error = nil with invalid notification config key")
	}
	var version int64
	if err := db.Raw("SELECT max(version_id) FROM goose_db_version").Scan(&version).Error; err != nil {
		t.Fatalf("read schema version after failed encryption: %v", err)
	}
	if version != 10 {
		t.Fatalf("schema version after failed encryption = %d, want 10", version)
	}
	var sealedColumnExists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'notify_channels'
			  AND column_name = 'config_sealed'
		)
	`).Scan(&sealedColumnExists).Error; err != nil {
		t.Fatalf("inspect schema after failed encryption: %v", err)
	}
	if sealedColumnExists {
		t.Fatal("config_sealed column exists after failed migration")
	}

	var state struct {
		Config string `gorm:"column:config"`
	}
	if err := db.Raw(`
		SELECT config::text AS config
		FROM notify_channels
		WHERE name = 'legacy'
	`).Scan(&state).Error; err != nil {
		t.Fatalf("load notification config after failed encryption: %v", err)
	}
	if state.Config == "{}" {
		t.Fatalf("notification config changed after failed encryption: config=%q", state.Config)
	}

	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("remove invalid notification config key: %v", err)
	}
	if _, err := migrate.Run(ctx, db, keyPath); err != nil {
		t.Fatalf("retry notification config migration: %v", err)
	}
	if err := db.Raw("SELECT max(version_id) FROM goose_db_version").Scan(&version).Error; err != nil {
		t.Fatalf("read schema version after retry: %v", err)
	}
	if version != 11 {
		t.Fatalf("schema version after retry = %d, want 11", version)
	}
}
