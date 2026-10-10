package alert

import (
	"maps"
	"slices"
)

func (s *Service) loadRuntimeState(serverID int64) map[string]RuntimeState {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return maps.Clone(s.runtime[serverID])
}

func (s *Service) saveRuntimeState(serverID int64, current, next map[string]RuntimeState) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	stored := s.runtime[serverID]
	if stored == nil {
		stored = make(map[string]RuntimeState)
	}
	// Commit against the evaluation snapshot so control-loop changes take precedence.
	for key, previous := range current {
		if _, ok := next[key]; !ok && stored[key] == previous {
			delete(stored, key)
		}
	}
	for key, state := range next {
		previous, hadCurrent := current[key]
		if hadCurrent && previous == state {
			continue
		}
		actual, hadStored := stored[key]
		if hadStored == hadCurrent && actual == previous {
			stored[key] = state
		}
	}
	if len(stored) == 0 {
		delete(s.runtime, serverID)
		return
	}
	s.runtime[serverID] = stored
}

func (s *Service) runtimeServerIDs() []int64 {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return slices.Collect(maps.Keys(s.runtime))
}
