package alert

import (
	"context"

	"dash/internal/notify"

	"gorm.io/gorm"
)

type Store struct {
	db           *gorm.DB
	configCipher *notify.ConfigCipher
	dirty        *dirtyQueue
}

func New(db *gorm.DB, configCipher *notify.ConfigCipher) *Store {
	return &Store{
		db:           db,
		configCipher: configCipher,
		dirty:        newDirtyQueue(),
	}
}

func (s *Store) WithTx(ctx context.Context, fn func(tx *Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Store{
			db:           tx,
			configCipher: s.configCipher,
			dirty:        s.dirty,
		})
	})
}
