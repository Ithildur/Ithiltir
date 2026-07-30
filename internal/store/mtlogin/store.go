package mtlogin

import (
	"fmt"
)

type Store struct {
	mem *memState
}

func (s *Store) Validate() error {
	if s == nil || s.mem == nil {
		return fmt.Errorf("store: mtproto login store is not initialized")
	}
	return nil
}

func New() *Store {
	return &Store{mem: newMemory()}
}
