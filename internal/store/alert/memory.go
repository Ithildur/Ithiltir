package alert

import (
	"sync"
)

type memState struct {
	alertMu      sync.Mutex
	alertRuntime map[int64]map[string]string
}

func newMemory() *memState {
	return &memState{
		alertRuntime: make(map[int64]map[string]string),
	}
}
