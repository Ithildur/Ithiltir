package frontcache

import "sync"

type memState struct {
	mu                sync.RWMutex
	frontIDs          map[string]struct{}
	frontRuntime      map[string][]byte
	frontMetadata     map[string][]byte
	frontSmart        map[string][]byte
	frontThermal      map[string][]byte
	frontCatalog      bool
	frontGuestVisible map[string]struct{}
	guestCatalog      bool
}

func newMemory() *memState {
	return &memState{
		frontIDs:          make(map[string]struct{}),
		frontRuntime:      make(map[string][]byte),
		frontMetadata:     make(map[string][]byte),
		frontSmart:        make(map[string][]byte),
		frontThermal:      make(map[string][]byte),
		frontGuestVisible: make(map[string]struct{}),
	}
}
