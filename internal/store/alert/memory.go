package alert

import (
	"sync"
	"time"
)

type memState struct {
	alertMu       sync.Mutex
	alertDirty    map[int64]struct{}
	alertInflight map[int64]time.Time
	alertRuntime  map[int64]map[string]string
	alertWake     chan struct{}
}

func newMemory() *memState {
	return &memState{
		alertDirty:    make(map[int64]struct{}),
		alertInflight: make(map[int64]time.Time),
		alertRuntime:  make(map[int64]map[string]string),
		alertWake:     make(chan struct{}, 1),
	}
}

func (m *memState) wakeAlert() {
	if m == nil {
		return
	}
	select {
	case m.alertWake <- struct{}{}:
	default:
	}
}
