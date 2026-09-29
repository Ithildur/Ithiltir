// Package nodesession owns the bounded reverse-query sessions opened by nodes.
package nodesession

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"dash/internal/model"
	"dash/internal/virt"
)

var (
	ErrOffline     = errors.New("node_offline")
	ErrUnsupported = errors.New("unsupported")
	ErrBusy        = errors.New("busy")
	ErrSource      = errors.New("source_unavailable")
	ErrVMNotFound  = errors.New("vm_not_found")
)

type Capabilities struct {
	Connected     bool   `json:"connected"`
	PVEHistory    bool   `json:"pve_history"`
	Version       string `json:"version"`
	HelperVersion string `json:"helper_version"`
}
type Command struct {
	ID       string
	Query    virt.HistoryQuery
	Deadline time.Time
	Cancel   bool
}
type Result struct {
	JSON []byte
	Err  error
}

type Session struct {
	id       int64
	secret   string
	caps     Capabilities
	ctx      context.Context
	cancel   context.CancelFunc
	commands chan Command
	mu       sync.Mutex
	pending  map[string]chan Result
}
type Hub struct {
	mu           sync.Mutex
	sessions     map[int64]*Session
	next         atomic.Uint64
	closed       bool
	authenticate func(context.Context, string) (model.Server, error)
}

func New(authenticate func(context.Context, string) (model.Server, error)) *Hub {
	return &Hub{sessions: make(map[int64]*Session), authenticate: authenticate}
}
func (h *Hub) Open(ctx context.Context, id int64, secret string, caps Capabilities) *Session {
	ctx, cancel := context.WithCancel(ctx)
	caps.Connected = true
	s := &Session{id: id, secret: secret, caps: caps, ctx: ctx, cancel: cancel, commands: make(chan Command, 64), pending: make(map[string]chan Result)}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		cancel()
		return s
	}
	if previous := h.sessions[id]; previous != nil {
		previous.cancel()
	}
	h.sessions[id] = s
	return s
}
func (h *Hub) Remove(s *Session) {
	s.cancel()
	h.mu.Lock()
	if h.sessions[s.id] == s {
		delete(h.sessions, s.id)
	}
	h.mu.Unlock()
}
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for _, s := range h.sessions {
		s.cancel()
	}
	clear(h.sessions)
}
func (h *Hub) current(ctx context.Context, id int64) (*Session, error) {
	h.mu.Lock()
	s := h.sessions[id]
	h.mu.Unlock()
	if s == nil || s.ctx.Err() != nil {
		return nil, ErrOffline
	}
	server, err := h.authenticate(ctx, s.secret)
	if err != nil || server.ID != id {
		h.Remove(s)
		return nil, ErrOffline
	}
	return s, nil
}
func (h *Hub) Capabilities(ctx context.Context, id int64) Capabilities {
	s, err := h.current(ctx, id)
	if err != nil {
		return Capabilities{}
	}
	return s.caps
}
func (s *Session) Context() context.Context { return s.ctx }
func (s *Session) Commands() <-chan Command { return s.commands }
func (s *Session) Complete(id string, result Result) {
	s.mu.Lock()
	done := s.pending[id]
	s.mu.Unlock()
	if done != nil {
		select {
		case done <- result:
		default:
		}
	}
}
func (h *Hub) History(ctx context.Context, id int64, q virt.HistoryQuery) ([]byte, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s, err := h.current(ctx, id)
	if err != nil {
		return nil, err
	}
	if !s.caps.PVEHistory {
		return nil, ErrUnsupported
	}
	requestID := strconv.FormatUint(h.next.Add(1), 10)
	done := make(chan Result, 1)
	s.mu.Lock()
	if len(s.pending) >= 32 {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	s.pending[requestID] = done
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, requestID); s.mu.Unlock() }()
	deadline, _ := ctx.Deadline()
	select {
	case s.commands <- Command{ID: requestID, Query: q, Deadline: deadline}:
	case <-s.ctx.Done():
		return nil, ErrOffline
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, ErrBusy
	}
	select {
	case result := <-done:
		return result.JSON, result.Err
	case <-s.ctx.Done():
		return nil, ErrOffline
	case <-ctx.Done():
		select {
		case s.commands <- Command{ID: requestID, Cancel: true}:
		default:
			s.cancel()
		}
		return nil, ctx.Err()
	}
}
