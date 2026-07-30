package node

import (
	"time"

	"dash/internal/store/frontcache"
	"dash/internal/store/frontprojection"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func newTestStore(db *gorm.DB, client *redis.Client) *Store {
	projection := frontprojection.New()
	return New(db, frontcache.New(db, client, projection), projection, time.Local)
}
