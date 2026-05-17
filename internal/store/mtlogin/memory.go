package mtlogin

import (
	"sync"
	"time"
)

type memState struct {
	mtprotoMu sync.Mutex
	mtproto   map[string]mtprotoEntry
}

type mtprotoEntry struct {
	raw       []byte
	expiresAt time.Time
}

func newMemory() *memState {
	return &memState{mtproto: make(map[string]mtprotoEntry)}
}
