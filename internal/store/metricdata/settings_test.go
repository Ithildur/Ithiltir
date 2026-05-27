package metricdata

import (
	"context"
	"testing"

	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationHistoryGuestAccessMode(t *testing.T) {
	st := New(pgtest.NewDB(t))
	ctx := context.Background()

	mode, err := st.GetHistoryGuestAccessMode(ctx)
	if err != nil {
		t.Fatalf("GetHistoryGuestAccessMode() error = %v", err)
	}
	if mode != HistoryGuestAccessDisabled {
		t.Fatalf("GetHistoryGuestAccessMode() = %q, want %q", mode, HistoryGuestAccessDisabled)
	}

	if err := st.SetHistoryGuestAccessMode(ctx, HistoryGuestAccessByNode); err != nil {
		t.Fatalf("SetHistoryGuestAccessMode(by_node) error = %v", err)
	}
	mode, err = st.GetHistoryGuestAccessMode(ctx)
	if err != nil {
		t.Fatalf("GetHistoryGuestAccessMode() error = %v", err)
	}
	if mode != HistoryGuestAccessByNode {
		t.Fatalf("GetHistoryGuestAccessMode() = %q, want %q", mode, HistoryGuestAccessByNode)
	}

	if err := st.SetHistoryGuestAccessMode(ctx, HistoryGuestAccessMode("public")); err == nil {
		t.Fatal("SetHistoryGuestAccessMode(invalid) error = nil, want error")
	}
}
