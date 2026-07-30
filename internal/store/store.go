package store

import (
	"context"
	"fmt"
	"time"

	alertstore "dash/internal/store/alert"
	"dash/internal/store/frontcache"
	"dash/internal/store/frontprojection"
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
	db      *gorm.DB
	Traffic *traffic.Store
	Metric  *metricdata.Store
	Front   *frontcache.Store
	Alert   *alertstore.Store
	Node    *node.Store
	System  *system.Store
	MTLogin *mtlogin.Store
}

// New wires concrete stores. DB/Redis may be nil; call Validate at startup.
func New(db *gorm.DB, redisClient *redis.Client, trafficLoc *time.Location) *Stores {
	projection := frontprojection.New()
	front := frontcache.New(db, redisClient, projection)
	alert := alertstore.New(db)

	return &Stores{
		db:      db,
		Traffic: traffic.New(db),
		Metric:  metricdata.New(db),
		Front:   front,
		Alert:   alert,
		Node:    node.New(db, front, projection, trafficLoc),
		System:  system.New(db),
		MTLogin: mtlogin.New(),
	}
}

func (s *Stores) Validate() error {
	if s == nil {
		return fmt.Errorf("store: nil")
	}
	if s.db == nil {
		return fmt.Errorf("store: DB is nil")
	}
	if s.Node == nil || s.Traffic == nil || s.Metric == nil || s.Front == nil || s.Alert == nil || s.System == nil || s.MTLogin == nil {
		return fmt.Errorf("store: incomplete")
	}
	if err := s.Node.Validate(); err != nil {
		return err
	}
	if err := s.Front.Validate(); err != nil {
		return err
	}
	if err := s.Alert.Validate(); err != nil {
		return err
	}
	if err := s.MTLogin.Validate(); err != nil {
		return err
	}
	return nil
}

func (s *Stores) WithSettingsTx(ctx context.Context, fn func(metric *metricdata.Store, system *system.Store) error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(metricdata.New(tx), system.New(tx))
	})
}
