package node

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"dash/internal/model"
	appversion "dash/internal/version"
)

const (
	deployGrantTTL         = 30 * time.Minute
	deployGrantTokenLength = 32
)

type AgentPlatform struct {
	OS      string
	Arch    string
	Version string
}

type AgentUpdateTarget struct {
	Version string
	URL     string
	SHA256  string
	Size    int64
}

func (s *Store) AgentPlatform(ctx context.Context, id int64) (AgentPlatform, error) {
	var row struct {
		OS      *string
		Arch    *string
		Version *string `gorm:"column:agent_version"`
	}
	err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("os", "arch", "agent_version").
		Where("id = ? AND is_deleted = ?", id, false).
		Take(&row).
		Error
	if err != nil {
		return AgentPlatform{}, err
	}
	return AgentPlatform{
		OS:      strings.TrimSpace(deref(row.OS)),
		Arch:    strings.TrimSpace(deref(row.Arch)),
		Version: strings.TrimSpace(deref(row.Version)),
	}, nil
}

func (s *Store) RequestAgentUpdate(id int64, target AgentUpdateTarget) {
	if s == nil || s.mem == nil || id <= 0 {
		return
	}
	target.Version = strings.TrimSpace(target.Version)
	s.mem.authMu.RLock()
	if _, active := s.mem.authByID[id]; !active {
		s.mem.authMu.RUnlock()
		return
	}
	s.mem.updateMu.Lock()
	s.mem.updates[id] = agentUpdateState{target: target}
	s.mem.updateMu.Unlock()
	s.mem.authMu.RUnlock()
}

func (s *Store) ResolveAgentUpdate(id int64, current string) (AgentUpdateTarget, bool, error) {
	if s == nil || s.mem == nil || id <= 0 {
		return AgentUpdateTarget{}, false, nil
	}
	s.mem.updateMu.RLock()
	state, ok := s.mem.updates[id]
	s.mem.updateMu.RUnlock()
	if !ok {
		return AgentUpdateTarget{}, false, nil
	}

	target := state.target
	target.Version = strings.TrimSpace(target.Version)
	if target.Version != "" {
		ok, err := appversion.IsNodeUpdateTarget(current, target.Version)
		if err != nil {
			return AgentUpdateTarget{}, false, err
		}
		if ok {
			return target, true, nil
		}
	}

	s.mem.updateMu.Lock()
	if latest, exists := s.mem.updates[id]; exists && latest == state {
		delete(s.mem.updates, id)
	}
	s.mem.updateMu.Unlock()
	return AgentUpdateTarget{}, false, nil
}

// TODO: remove deploy grants after the minimum supported node can send X-Node-Secret on update downloads.
func (s *Store) GrantDeployAccess(assetPath string) (string, error) {
	return s.grantDeployAccess(assetPath, time.Now().UTC(), deployGrantTTL)
}

func (s *Store) grantDeployAccess(assetPath string, now time.Time, ttl time.Duration) (string, error) {
	if s == nil || s.mem == nil {
		return "", fmt.Errorf("store: memory deploy grants are nil")
	}
	assetPath = cleanDeployPath(assetPath)
	if assetPath == "" {
		return "", fmt.Errorf("store: deploy grant path is empty")
	}

	s.mem.deployGrantMu.Lock()
	defer s.mem.deployGrantMu.Unlock()
	pruneDeployGrantsLocked(s.mem.deployGrants, now)

	for range 5 {
		token, err := randomString(deployGrantTokenLength, secretAlphabet)
		if err != nil {
			return "", fmt.Errorf("generate deploy grant: %w", err)
		}
		if _, exists := s.mem.deployGrants[token]; exists {
			continue
		}
		s.mem.deployGrants[token] = deployGrantState{path: assetPath, expiresAt: now.Add(ttl)}
		return token, nil
	}
	return "", fmt.Errorf("store: deploy grant token collision")
}

func (s *Store) ValidDeployGrant(token, assetPath string) bool {
	return s.validDeployGrant(token, assetPath, time.Now().UTC())
}

func (s *Store) validDeployGrant(token, assetPath string, now time.Time) bool {
	if s == nil || s.mem == nil {
		return false
	}
	token = strings.TrimSpace(token)
	assetPath = cleanDeployPath(assetPath)
	if token == "" || assetPath == "" {
		return false
	}

	s.mem.deployGrantMu.Lock()
	defer s.mem.deployGrantMu.Unlock()
	grant, ok := s.mem.deployGrants[token]
	if !ok {
		return false
	}
	if !now.Before(grant.expiresAt) {
		delete(s.mem.deployGrants, token)
		return false
	}
	return grant.path == assetPath
}

func pruneDeployGrantsLocked(grants map[string]deployGrantState, now time.Time) {
	for token, grant := range grants {
		if !now.Before(grant.expiresAt) {
			delete(grants, token)
		}
	}
}

func cleanDeployPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return path.Clean(raw)
}
func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
