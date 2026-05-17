package node

import (
	"context"
	"fmt"
	"time"

	"dash/internal/model"
	"dash/internal/store/frontcache"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Store struct {
	db      *gorm.DB
	mem     *memState
	front   *frontcache.Store
	auth    nodeAuthBackend
	runtime nodeRuntimeBackend
}

type nodeAuthBackend interface {
	syncServerCache(srv model.Server, oldSecret string) error
	deleteServerMeta(id int64, secret string) error
	getSecretByID(id int64) (string, error)
	getServerBySecret(secret string) (model.Server, error)
}

type nodeRuntimeBackend interface {
	getServerRuntimeIP(ctx context.Context, serverID int64) (string, bool, error)
	setServerRuntime(ctx context.Context, serverID int64, ip string, lastOnlineAt time.Time) error
}

func New(db *gorm.DB, redisClient *redis.Client, front *frontcache.Store) *Store {
	mem := newMemory()
	return &Store{
		db:      db,
		mem:     mem,
		front:   front,
		auth:    newNodeAuthBackend(mem),
		runtime: newNodeRuntimeBackend(redisClient, mem),
	}
}

func (s *Store) Validate() error {
	if s == nil {
		return fmt.Errorf("store: node store is nil")
	}
	if s.db == nil {
		return fmt.Errorf("store: DB is nil")
	}
	return nil
}
