package mtlogin

import (
	"context"
	"time"
)

func (s *Store) SetMTProtoLogin(_ context.Context, id string, raw []byte, ttl time.Duration) error {
	if s == nil || s.mem == nil || ttl <= 0 {
		return nil
	}
	s.mem.mtprotoMu.Lock()
	s.mem.mtproto[id] = mtprotoEntry{
		raw:       append([]byte(nil), raw...),
		expiresAt: time.Now().UTC().Add(ttl),
	}
	s.mem.mtprotoMu.Unlock()
	return nil
}

func (s *Store) GetMTProtoLogin(_ context.Context, id string) ([]byte, error) {
	if s == nil || s.mem == nil {
		return nil, nil
	}
	now := time.Now().UTC()
	s.mem.mtprotoMu.Lock()
	defer s.mem.mtprotoMu.Unlock()
	item, ok := s.mem.mtproto[id]
	if !ok {
		return nil, nil
	}
	if !item.expiresAt.After(now) {
		delete(s.mem.mtproto, id)
		return nil, nil
	}
	return append([]byte(nil), item.raw...), nil
}

func (s *Store) DeleteMTProtoLogin(_ context.Context, id string) error {
	if s == nil || s.mem == nil {
		return nil
	}
	s.mem.mtprotoMu.Lock()
	delete(s.mem.mtproto, id)
	s.mem.mtprotoMu.Unlock()
	return nil
}
