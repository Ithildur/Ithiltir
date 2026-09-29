package noderpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"dash/internal/model"
	"dash/internal/nodeingest"
	"dash/internal/nodesession"
	"dash/internal/nodewire"
	"dash/internal/serverid"
	"dash/internal/store"
	"dash/internal/virt"
)

func TestQueryStreamOutlivesHTTPRequestDeadlines(t *testing.T) {
	st := store.New(nil, nil, time.UTC, nil)
	if err := st.Node.SyncServerCache(t.Context(), model.Server{ID: 1, Secret: "secret"}); err != nil {
		t.Fatal(err)
	}
	ingest := nodeingest.New(st.Node, st.Metric, st.Front, st.Alert, serverid.New(filepath.Join(t.TempDir(), "id")), 17)
	hub := nodesession.New(ingest.Authenticate)
	rpc := New(ingest, hub, nil)
	defer rpc.Close()
	server := httptest.NewUnstartedServer(rpc)
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Config.ReadTimeout = 50 * time.Millisecond
	server.Config.WriteTimeout = 50 * time.Millisecond
	server.Start()
	defer server.Close()
	conn, err := grpc.NewClient(strings.TrimPrefix(server.URL, "http://"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-node-secret", "secret")
	stream, err := nodewire.NewNodeClient(conn).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&nodewire.NodeMessage{Body: &nodewire.NodeMessage_Capabilities{Capabilities: &nodewire.Capabilities{PveHistory: true}}}); err != nil {
		t.Fatal(err)
	}
	for !hub.Capabilities(ctx, 1).Connected {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	// Both the ordinary HTTP timeout and the per-write deadline must be absent
	// while a healthy stream is idle between completed queries.
	for _, idle := range []time.Duration{150 * time.Millisecond, 5100 * time.Millisecond} {
		time.Sleep(idle)
		done := make(chan error, 1)
		go func() {
			_, err := hub.History(ctx, 1, virt.HistoryQuery{VMID: 101, Timeframe: "hour", Consolidation: "AVERAGE"})
			done <- err
		}()
		command, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if command.GetHistory() == nil {
			t.Fatal("missing query")
		}
		if err := stream.Send(&nodewire.NodeMessage{Id: command.Id, Body: &nodewire.NodeMessage_Result{Result: &nodewire.QueryResult{Json: []byte("{}")}}}); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Node.SyncServerCache(ctx, model.Server{ID: 1, Secret: "rotated"}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("healthy peer lost credential-revocation status: %v", err)
	}
}
