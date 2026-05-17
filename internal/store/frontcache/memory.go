package frontcache

import "sync"

type memState struct {
	mu                sync.RWMutex
	frontNodes        map[string][]byte
	frontSmart        map[string][]byte
	frontThermal      map[string][]byte
	frontMeta         bool
	frontGuestVisible map[string]struct{}
	guestVisibleMeta  bool
}

func newMemory() *memState {
	return &memState{
		frontNodes:        make(map[string][]byte),
		frontSmart:        make(map[string][]byte),
		frontThermal:      make(map[string][]byte),
		frontGuestVisible: make(map[string]struct{}),
	}
}
