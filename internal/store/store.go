package store

import (
	"fmt"

	alertstore "dash/internal/store/alert"
	"dash/internal/store/frontcache"
	"dash/internal/store/metricdata"
	"dash/internal/store/mtlogin"
	"dash/internal/store/node"
	"dash/internal/store/system"
	"dash/internal/store/traffic"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Stores is the application store wiring. Leaf code should depend on the
// specific store it needs, not on this aggregate.
type Stores struct {
	Traffic *traffic.Store
	Metric  *metricdata.Store
	Front   *frontcache.Store
	Alert   *alertstore.Store
	Node    *node.Store
	System  *system.Store
	MTLogin *mtlogin.Store
}

// New wires concrete stores. DB/Redis may be nil; call Validate at startup.
func New(db *gorm.DB, redisClient *redis.Client) *Stores {
	front := frontcache.New(db, redisClient)
	alert := alertstore.New(db, redisClient)

	return &Stores{
		Traffic: traffic.New(db),
		Metric:  metricdata.New(db),
		Front:   front,
		Alert:   alert,
		Node:    node.New(db, redisClient, front),
		System:  system.New(db),
		MTLogin: mtlogin.New(redisClient),
	}
}

func (s *Stores) Validate() error {
	if s == nil {
		return fmt.Errorf("store: nil")
	}
	if s.Node == nil || s.Traffic == nil || s.Metric == nil || s.Front == nil || s.Alert == nil || s.System == nil || s.MTLogin == nil {
		return fmt.Errorf("store: incomplete")
	}
	if err := s.Node.Validate(); err != nil {
		return err
	}
	return nil
}

func MustNew(db *gorm.DB, redisClient *redis.Client) *Stores {
	s := New(db, redisClient)
	if err := s.Validate(); err != nil {
		panic(err)
	}
	return s
}
