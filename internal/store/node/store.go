package node

import (
	"context"
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

// WithMetricsIngest serializes the full ingest path with lifecycle changes for
// the authenticated node. Runtime samples do not count as structural
// projection mutations.
func (s *Store) WithMetricsIngest(ctx context.Context, id int64, fn func() error) error {
	return s.mutations.runtime(ctx, id, fn)
}
