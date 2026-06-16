package node

import (
	"context"
	"testing"
)

func TestResolveAgentUpdateKeepsDifferentBuildMetadata(t *testing.T) {
	st := &Store{mem: newMemory()}
	target := AgentUpdateTarget{
		Version: "1.0.0+build.2",
		URL:     "https://example.test/node",
		SHA256:  "sha",
		Size:    1,
	}
	st.RequestAgentUpdate(7, target)

	got, ok, err := st.ResolveAgentUpdate(context.Background(), 7, "1.0.0+build.1")
	if err != nil {
		t.Fatalf("ResolveAgentUpdate() error = %v", err)
	}
	if !ok {
		t.Fatal("ResolveAgentUpdate() ok = false, want true")
	}
	if got != target {
		t.Fatalf("ResolveAgentUpdate() target = %+v, want %+v", got, target)
	}
}
