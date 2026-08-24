package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	"dash/internal/model"
	"dash/internal/notify"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
)

const (
	notificationAndTrafficStateVersion int64 = 11
	notifyConfigConstraint                   = "chk_notify_channels_config_storage"
)

var notificationAndTrafficStateSQL = readSQL("0011_notification_and_traffic_state/schema.sql")

type legacyNotifyConfig struct {
	id     int64
	typ    model.NotifyType
	config []byte
	sealed []byte
}

func notificationAndTrafficState(keyPath string) *goose.Migration {
	return goose.NewGoMigration(
		notificationAndTrafficStateVersion,
		&goose.GoFunc{
			RunTx: func(ctx context.Context, tx *sql.Tx) error {
				if strings.TrimSpace(keyPath) == "" {
					return errors.New("notification config key path is empty")
				}
				return runNotificationAndTrafficState(ctx, tx, keyPath)
			},
		},
		nil,
	)
}

const stateSavepoint = "migration_0011_attempt"

func runNotificationAndTrafficState(ctx context.Context, tx *sql.Tx, keyPath string) error {
	delays := [...]time.Duration{
		10 * time.Second,
		30 * time.Second,
		time.Minute,
	}
	for attempt := range len(delays) + 1 {
		if _, err := tx.ExecContext(ctx, "SAVEPOINT "+stateSavepoint); err != nil {
			return fmt.Errorf("create notification and traffic state migration savepoint: %w", err)
		}

		err := applyNotificationAndTrafficState(ctx, tx, keyPath)
		if err == nil {
			if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+stateSavepoint); err != nil {
				return fmt.Errorf("release notification and traffic state migration savepoint: %w", err)
			}
			return nil
		}
		if attempt == len(delays) || !retryableStateError(err) {
			return err
		}
		if resetErr := resetStateAttempt(ctx, tx, err); resetErr != nil {
			return resetErr
		}

		delay := delays[attempt]
		slog.WarnContext(ctx, "retrying migration 0011 after transient database error",
			slog.Int("retry", attempt+1),
			slog.Int("max_retries", len(delays)),
			slog.Duration("delay", delay),
			slog.Any("error", err),
		)
		if err := waitStateRetry(ctx, delay); err != nil {
			return err
		}
	}
	return errors.New("notification and traffic state migration exhausted retries")
}

func applyNotificationAndTrafficState(ctx context.Context, tx *sql.Tx, keyPath string) error {
	if _, err := tx.ExecContext(ctx, notificationAndTrafficStateSQL); err != nil {
		return fmt.Errorf("run notification and traffic state SQL: %w", err)
	}
	return sealNotifyConfigs(ctx, tx, keyPath)
}

func resetStateAttempt(ctx context.Context, tx *sql.Tx, migrationErr error) error {
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+stateSavepoint); err != nil {
		return fmt.Errorf("rollback notification and traffic state migration after %v: %w", migrationErr, err)
	}
	if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+stateSavepoint); err != nil {
		return fmt.Errorf("release failed notification and traffic state migration attempt after %v: %w", migrationErr, err)
	}
	return nil
}

func retryableStateError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01", "55P03":
		return true
	case "XX000":
		message := strings.ToLower(pgErr.Message)
		return message == "tuple concurrently updated" || message == "tuple concurrently deleted"
	}
	return false
}

func waitStateRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait to retry notification and traffic state migration: %w", ctx.Err())
	}
}

func sealNotifyConfigs(ctx context.Context, tx *sql.Tx, keyPath string) error {
	if tx == nil {
		return errors.New("seal notification configs: tx is nil")
	}

	var sealedCount int64
	if err := tx.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM notify_channels WHERE config_sealed IS NOT NULL",
	).Scan(&sealedCount); err != nil {
		return fmt.Errorf("count encrypted notification configs: %w", err)
	}
	configCipher, err := loadOrCreateNotifyConfigCipher(keyPath, sealedCount)
	if err != nil {
		return err
	}

	queryRows, err := tx.QueryContext(ctx, `
		SELECT id, type::text, config::text, config_sealed
		FROM notify_channels
		ORDER BY id ASC
		FOR UPDATE
	`)
	if err != nil {
		return fmt.Errorf("load notification configs for migration: %w", err)
	}
	var rows []legacyNotifyConfig
	for queryRows.Next() {
		var row legacyNotifyConfig
		var typ string
		var config string
		if err := queryRows.Scan(&row.id, &typ, &config, &row.sealed); err != nil {
			_ = queryRows.Close()
			return fmt.Errorf("scan notification config for migration: %w", err)
		}
		row.typ = model.NotifyType(typ)
		row.config = []byte(config)
		rows = append(rows, row)
	}
	if err := queryRows.Err(); err != nil {
		_ = queryRows.Close()
		return fmt.Errorf("read notification configs for migration: %w", err)
	}
	if err := queryRows.Close(); err != nil {
		return fmt.Errorf("close notification config rows: %w", err)
	}

	for i := range rows {
		row := &rows[i]
		if len(row.sealed) != 0 {
			plain, err := configCipher.Open(row.id, row.typ, row.sealed)
			if err != nil {
				return fmt.Errorf("verify encrypted notification config %d: %w", row.id, err)
			}
			clear(plain)
			continue
		}
		payload, err := configCipher.Seal(row.id, row.typ, row.config)
		if err != nil {
			return fmt.Errorf("encrypt notification config %d: %w", row.id, err)
		}
		row.sealed = payload
	}

	if len(rows) != 0 {
		if _, err := tx.ExecContext(
			ctx,
			"ALTER TABLE notify_channels DISABLE TRIGGER notify_channels_updated_at",
		); err != nil {
			return fmt.Errorf("disable notification channel update trigger: %w", err)
		}
		for i := range rows {
			row := &rows[i]
			if _, err := tx.ExecContext(
				ctx,
				"UPDATE notify_channels SET config = '{}'::jsonb, config_sealed = $1 WHERE id = $2",
				row.sealed,
				row.id,
			); err != nil {
				return fmt.Errorf("store encrypted notification config %d: %w", row.id, err)
			}
			clear(row.config)
		}
		if _, err := tx.ExecContext(
			ctx,
			"ALTER TABLE notify_channels ENABLE TRIGGER notify_channels_updated_at",
		); err != nil {
			return fmt.Errorf("enable notification channel update trigger: %w", err)
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		"ALTER TABLE notify_channels VALIDATE CONSTRAINT "+notifyConfigConstraint,
	); err != nil {
		return fmt.Errorf("validate encrypted notification config constraint: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		ALTER TABLE notify_channels
		ALTER COLUMN config_sealed SET NOT NULL
	`); err != nil {
		return fmt.Errorf("require encrypted notification configs: %w", err)
	}
	return nil
}

func loadOrCreateNotifyConfigCipher(keyPath string, sealedCount int64) (*notify.ConfigCipher, error) {
	configCipher, err := notify.LoadConfigCipher(keyPath)
	if err == nil {
		return configCipher, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if sealedCount != 0 {
		return nil, fmt.Errorf(
			"notification config key is missing while %d encrypted configs exist; restore the key from backup",
			sealedCount,
		)
	}
	if err := notify.CreateConfigKey(keyPath); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	return notify.LoadConfigCipher(keyPath)
}
