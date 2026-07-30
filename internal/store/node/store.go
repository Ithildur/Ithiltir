package node

import (
	"fmt"
	"time"

	"dash/internal/store/frontcache"
	"dash/internal/store/frontprojection"
	"gorm.io/gorm"
)

type Store struct {
	db         *gorm.DB
	mem        *memState
	front      *frontcache.Store
	projection *frontprojection.Gate
	mutations  nodeMutations
	trafficLoc *time.Location
}

func New(db *gorm.DB, front *frontcache.Store, projection *frontprojection.Gate, trafficLoc *time.Location) *Store {
	mem := newMemory()
	return &Store{
		db:         db,
		mem:        mem,
		front:      front,
		projection: projection,
		trafficLoc: trafficLoc,
	}
}

func (s *Store) Validate() error {
	if s == nil {
		return fmt.Errorf("store: node store is nil")
	}
	if s.db == nil || s.mem == nil || s.front == nil || s.projection == nil || s.trafficLoc == nil {
		return fmt.Errorf("store: node store is not initialized")
	}
	return nil
}

// WithMetricsIngest serializes the full ingest path with lifecycle changes for
// the authenticated node. Runtime samples do not count as structural
// projection mutations.
func (s *Store) WithMetricsIngest(id int64, fn func() error) error {
	if s == nil {
		return fmt.Errorf("store: node store is nil")
	}
	return s.mutations.runtime(id, fn)
}
