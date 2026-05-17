package alert

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Store struct {
	db    *gorm.DB
	alert alertRuntimeBackend
}

type alertRuntimeBackend interface {
	markServerDirty(ctx context.Context, id int64) error
	claimDirtyServer(ctx context.Context, until time.Time) (int64, bool, error)
	ackDirtyServer(ctx context.Context, id int64) error
	requeueExpiredDirtyServers(ctx context.Context, now time.Time, limit int64) error
	waitDirtyWakeup(ctx context.Context, timeout time.Duration) error
	loadAlertRuntime(ctx context.Context, id int64) (map[string]string, error)
	saveAlertRuntime(ctx context.Context, id int64, deletes []string, updates map[string][]byte, clear bool) error
	listAlertRuntimeServerIDs(ctx context.Context) ([]int64, error)
}

func New(db *gorm.DB, redisClient *redis.Client) *Store {
	mem := newMemory()
	return &Store{
		db:    db,
		alert: newAlertRuntimeBackend(redisClient, mem),
	}
}

func (s *Store) WithTx(ctx context.Context, fn func(tx *Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Store{
			db:    tx,
			alert: s.alert,
		})
	})
}
