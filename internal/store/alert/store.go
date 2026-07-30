package alert

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type Store struct {
	db      *gorm.DB
	runtime *memState
	dirty   *dirtyQueue
}

func (s *Store) Validate() error {
	if s == nil || s.db == nil || s.runtime == nil || s.dirty == nil {
		return fmt.Errorf("store: alert store is not initialized")
	}
	return nil
}

func New(db *gorm.DB) *Store {
	return &Store{
		db:      db,
		runtime: newMemory(),
		dirty:   newDirtyQueue(),
	}
}

func (s *Store) WithTx(ctx context.Context, fn func(tx *Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Store{
			db:      tx,
			runtime: s.runtime,
			dirty:   s.dirty,
		})
	})
}
