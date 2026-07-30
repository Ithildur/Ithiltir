package node

import (
	"testing"
	"time"

	"dash/internal/metrics"
)

func TestNodeMutationsSerializeSameNodeOnly(t *testing.T) {
	st := newTestStore(nil, nil)
	release := make(chan struct{})
	firstEntered := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- st.mutations.projected(1, st.projection, func() error {
			close(firstEntered)
			<-release
			return nil
		})
	}()
	<-firstEntered

	otherEntered := make(chan struct{})
	otherDone := make(chan error, 1)
	go func() {
		otherDone <- st.mutations.projected(2, st.projection, func() error {
			close(otherEntered)
			return nil
		})
	}()
	select {
	case <-otherEntered:
	case <-time.After(time.Second):
		t.Fatal("different node mutation was serialized behind node 1")
	}

	sameEntered := make(chan struct{})
	sameDone := make(chan error, 1)
	go func() {
		sameDone <- st.mutations.projected(1, st.projection, func() error {
			close(sameEntered)
			return nil
		})
	}()
	select {
	case <-sameEntered:
		t.Fatal("same node mutation entered concurrently")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first mutation error = %v", err)
	}
	if err := <-sameDone; err != nil {
		t.Fatalf("same-node mutation error = %v", err)
	}
	if err := <-otherDone; err != nil {
		t.Fatalf("other-node mutation error = %v", err)
	}

	st.mutations.locksMu.Lock()
	remaining := len(st.mutations.locks)
	st.mutations.locksMu.Unlock()
	if remaining != 0 {
		t.Fatalf("node locks retained after last user = %d", remaining)
	}
}

func TestGlobalMutationExcludesNodeProjection(t *testing.T) {
	st := newTestStore(nil, nil)
	nodeRelease := make(chan struct{})
	nodeEntered := make(chan struct{})
	nodeDone := make(chan error, 1)
	go func() {
		nodeDone <- st.mutations.projected(1, st.projection, func() error {
			close(nodeEntered)
			<-nodeRelease
			return nil
		})
	}()
	<-nodeEntered

	globalEntered := make(chan struct{})
	globalDone := make(chan error, 1)
	go func() {
		globalDone <- st.mutations.global(st.projection, func() error {
			close(globalEntered)
			return nil
		})
	}()
	select {
	case <-globalEntered:
		t.Fatal("global mutation entered during a node projection mutation")
	case <-time.After(20 * time.Millisecond):
	}

	close(nodeRelease)
	if err := <-nodeDone; err != nil {
		t.Fatalf("node mutation error = %v", err)
	}
	if err := <-globalDone; err != nil {
		t.Fatalf("global mutation error = %v", err)
	}
}

func TestUnknownRuntimeCannotDeadlockNodeProjection(t *testing.T) {
	st := newTestStore(nil, nil)
	version := st.projection.Version()
	buildEntered := make(chan struct{})
	buildRelease := make(chan struct{})
	buildDone := make(chan error, 1)
	go func() {
		_, err := st.projection.Publish(version, func() error {
			close(buildEntered)
			<-buildRelease
			return nil
		})
		buildDone <- err
	}()
	<-buildEntered

	runtimeEntered := make(chan struct{})
	runtimeDone := make(chan error, 1)
	go func() {
		runtimeDone <- st.WithMetricsIngest(1, func() error {
			close(runtimeEntered)
			return st.front.PutNodeRuntime(t.Context(), metrics.NodeView{
				Node: metrics.NodeMeta{ID: "1"},
			}, 0, 0)
		})
	}()
	<-runtimeEntered

	projectedDone := make(chan error, 1)
	go func() {
		projectedDone <- st.mutations.projected(1, st.projection, func() error {
			return nil
		})
	}()

	close(buildRelease)
	for name, done := range map[string]<-chan error{
		"build":      buildDone,
		"runtime":    runtimeDone,
		"projection": projectedDone,
	} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s error = %v", name, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s deadlocked", name)
		}
	}
}
