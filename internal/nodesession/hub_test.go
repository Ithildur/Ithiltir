package nodesession

import (
	"context"
	"errors"
	"testing"
	"time"

	"dash/internal/model"
	"dash/internal/virt"
)

func TestHistorySessionCapacityCancellationAndReplacement(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	hub := New(func(context.Context, string) (model.Server, error) {
		return model.Server{ID: 1}, nil
	})
	t.Cleanup(hub.Close)
	session := hub.Open(ctx, 1, "secret", Capabilities{PVEHistory: true})
	query := virt.HistoryQuery{VMID: 100, Timeframe: "hour", Consolidation: "AVERAGE"}
	results := make(chan error, 32)
	var firstCancel context.CancelFunc
	var firstID string
	for i := range 32 {
		requestCtx, stop := context.WithCancel(ctx)
		defer stop()
		go func() {
			_, err := hub.History(requestCtx, 1, query)
			results <- err
		}()
		select {
		case command := <-session.Commands():
			if i == 0 {
				firstCancel, firstID = stop, command.ID
			}
		case <-ctx.Done():
			t.Fatal("query was not dispatched")
		}
	}
	if _, err := hub.History(ctx, 1, query); !errors.Is(err, ErrBusy) {
		t.Fatalf("full session: %v", err)
	}
	firstCancel()
	select {
	case command := <-session.Commands():
		if !command.Cancel || command.ID != firstID {
			t.Fatalf("unexpected cancellation: %+v", command)
		}
	case <-ctx.Done():
		t.Fatal("cancel was not forwarded")
	}
	if err := <-results; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query: %v", err)
	}
	// A late reply cannot recreate pending work or block the stream reader.
	session.Complete(firstID, Result{JSON: []byte(`{}`)})
	replacement := hub.Open(ctx, 1, "secret", Capabilities{PVEHistory: true})
	for range 31 {
		select {
		case err := <-results:
			if !errors.Is(err, ErrOffline) {
				t.Fatalf("replaced session query: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("replaced session retained pending query")
		}
	}
	hub.Remove(session)
	go func() {
		_, err := hub.History(ctx, 1, query)
		results <- err
	}()
	select {
	case command := <-replacement.Commands():
		replacement.Complete(command.ID, Result{JSON: []byte(`{}`)})
	case <-ctx.Done():
		t.Fatal("replacement did not accept query")
	}
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	hub.Close()
	if _, err := hub.History(ctx, 1, query); !errors.Is(err, ErrOffline) {
		t.Fatalf("closed hub: %v", err)
	}
}
