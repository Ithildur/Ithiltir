package noderpc

import (
	"net/http"
	"sync"
	"time"
)

// queryStream owns the HTTP/2 stream and all application goroutines using it.
// Context cancellation alone cannot interrupt a flow-control-blocked HTTP write.
type queryStream struct {
	http.ResponseWriter
	controller *http.ResponseController
	tasks      sync.WaitGroup
	mu         sync.Mutex
	draining   bool
	closed     bool
}

func newQueryStream(w http.ResponseWriter) (*queryStream, error) {
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		return nil, err
	}
	if err := controller.SetWriteDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return &queryStream{ResponseWriter: w, controller: controller}, nil
}

func (s *queryStream) enter() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	// Hold a task reference while Connect can start its reader and writer.
	s.tasks.Add(1)
	return true
}

func (s *queryStream) leave() {
	s.mu.Lock()
	if !s.closed {
		s.draining = true
		// Allow the final gRPC status through, but never wait for the peer to
		// reopen its window. This deadline applies only to this HTTP/2 stream.
		_ = s.controller.SetWriteDeadline(time.Now().Add(time.Second))
	}
	s.mu.Unlock()
	s.tasks.Done()
}

func (s *queryStream) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	// gRPC owns stream-context cancellation; the request owns joining our tasks.
	s.tasks.Wait()
}

func (s *queryStream) writeDeadline(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.draining {
		return
	}
	var deadline time.Time
	if active {
		deadline = time.Now().Add(5 * time.Second)
	}
	_ = s.controller.SetWriteDeadline(deadline)
}

func (s *queryStream) Write(p []byte) (int, error) {
	s.writeDeadline(true)
	defer s.writeDeadline(false)
	return s.ResponseWriter.Write(p)
}

func (s *queryStream) Flush() {
	s.writeDeadline(true)
	defer s.writeDeadline(false)
	_ = s.controller.Flush()
}
