package node

import (
	"sync"
	"time"
)

type memState struct {
	authMu       sync.RWMutex
	authBySecret map[string]authEntry
	authByID     map[int64]authEntry

	updateMu sync.RWMutex
	updates  map[int64]agentUpdateState

	deployGrantMu sync.RWMutex
	deployGrants  map[string]deployGrantState
}

type authEntry struct {
	secret string
	meta   ServerMeta
}

type agentUpdateState struct {
	target AgentUpdateTarget
}

type deployGrantState struct {
	path      string
	expiresAt time.Time
}

func newMemory() *memState {
	return &memState{
		authBySecret: make(map[string]authEntry),
		authByID:     make(map[int64]authEntry),
		updates:      make(map[int64]agentUpdateState),
		deployGrants: make(map[string]deployGrantState),
	}
}
