package node

import (
	"context"
	"strings"

	"dash/internal/model"
	appversion "dash/internal/version"
)

type AgentPlatform struct {
	OS   string
	Arch string
}

type AgentUpdateTarget struct {
	Version string
	URL     string
	SHA256  string
	Size    int64
}

func (s *Store) AgentPlatform(ctx context.Context, id int64) (AgentPlatform, error) {
	var row struct {
		OS   *string
		Arch *string
	}
	err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("os", "arch").
		Where("id = ? AND is_deleted = ?", id, false).
		Take(&row).
		Error
	if err != nil {
		return AgentPlatform{}, err
	}
	return AgentPlatform{
		OS:   strings.TrimSpace(deref(row.OS)),
		Arch: strings.TrimSpace(deref(row.Arch)),
	}, nil
}

func (s *Store) RequestAgentUpdate(id int64, target AgentUpdateTarget) {
	target.Version = strings.TrimSpace(target.Version)
	s.mem.updateMu.Lock()
	s.mem.updates[id] = agentUpdateState{target: target}
	s.mem.updateMu.Unlock()
}

func (s *Store) ResolveAgentUpdate(_ context.Context, id int64, current string) (AgentUpdateTarget, bool, error) {
	s.mem.updateMu.RLock()
	state, ok := s.mem.updates[id]
	s.mem.updateMu.RUnlock()
	if !ok {
		return AgentUpdateTarget{}, false, nil
	}

	target := state.target
	target.Version = strings.TrimSpace(target.Version)
	if target.Version != "" {
		cmp, err := appversion.Compare(strings.TrimSpace(current), target.Version)
		if err != nil {
			return AgentUpdateTarget{}, false, err
		}
		if cmp < 0 {
			return target, true, nil
		}
	}

	s.mem.updateMu.Lock()
	delete(s.mem.updates, id)
	s.mem.updateMu.Unlock()
	return AgentUpdateTarget{}, false, nil
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
