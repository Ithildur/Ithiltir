package alert

import (
	"testing"

	pgtest "dash/internal/testutil/postgres"

	"gorm.io/gorm"
)

func newTestStore(t *testing.T, db *gorm.DB) *Store {
	t.Helper()
	return New(db, pgtest.ConfigCipher(t))
}
