package node

import (
	"context"
	"testing"
	"time"
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

func TestDeployGrantAllowsOnlyBoundPathBeforeExpiry(t *testing.T) {
	now := time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC)
	st := &Store{mem: newMemory()}

	token, err := st.grantDeployAccess("/deploy/linux/node_linux_amd64", now, time.Minute)
	if err != nil {
		t.Fatalf("grantDeployAccess() error = %v", err)
	}

	if !st.validDeployGrant(token, "/deploy/linux/node_linux_amd64", now.Add(time.Second)) {
		t.Fatal("validDeployGrant() = false, want true")
	}
	if st.validDeployGrant(token, "/deploy/linux/node_linux_arm64", now.Add(time.Second)) {
		t.Fatal("validDeployGrant() allowed a different asset path")
	}
	if st.validDeployGrant(token, "/deploy/linux/node_linux_amd64", now.Add(time.Minute)) {
		t.Fatal("validDeployGrant() allowed an expired grant")
	}
}
