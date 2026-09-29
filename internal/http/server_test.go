package httpserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/protobuf/proto"

	"dash/internal/model"
	"dash/internal/nodeingest"
	"dash/internal/noderpc"
	"dash/internal/nodesession"
	"dash/internal/nodewire"
	"dash/internal/serverid"
	"dash/internal/store"
	"dash/internal/virt"
)

func TestShutdownWithActiveQueryStream(t *testing.T) {
	for _, test := range []struct {
		name   string
		window uint32
	}{
		{"receiving", 65535},
		{"blocked", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := store.New(nil, nil, time.UTC, nil)
			if err := st.Node.SyncServerCache(t.Context(), model.Server{ID: 1, Secret: "secret"}); err != nil {
				t.Fatal(err)
			}
			ingest := nodeingest.New(st.Node, st.Metric, st.Front, st.Alert, serverid.New(filepath.Join(t.TempDir(), "id")), 17)
			hub := nodesession.New(ingest.Authenticate)
			rpc := noderpc.New(ingest, hub, nil)
			server := httptest.NewUnstartedServer(rpc)
			server.Config.Protocols = new(http.Protocols)
			server.Config.Protocols.SetUnencryptedHTTP2(true)
			server.Start()
			defer func() {
				_ = server.Config.Close()
				rpc.Close()
				server.Close()
			}()
			s := &HTTPServer{server: server.Config, rpc: rpc}
			conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
				t.Fatal(err)
			}
			framer := http2.NewFramer(conn, conn)
			if err := framer.WriteSettings(http2.Setting{ID: http2.SettingInitialWindowSize, Val: test.window}); err != nil {
				t.Fatal(err)
			}
			var headers bytes.Buffer
			encoder := hpack.NewEncoder(&headers)
			for _, field := range []hpack.HeaderField{
				{Name: ":method", Value: "POST"},
				{Name: ":scheme", Value: "http"},
				{Name: ":authority", Value: strings.TrimPrefix(server.URL, "http://")},
				{Name: ":path", Value: nodewire.Node_Connect_FullMethodName},
				{Name: "content-type", Value: "application/grpc"},
				{Name: "x-node-secret", Value: "secret"},
			} {
				if err := encoder.WriteField(field); err != nil {
					t.Fatal(err)
				}
			}
			if err := framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 1, BlockFragment: headers.Bytes(), EndHeaders: true}); err != nil {
				t.Fatal(err)
			}
			raw, err := proto.Marshal(&nodewire.NodeMessage{Body: &nodewire.NodeMessage_Capabilities{Capabilities: &nodewire.Capabilities{PveHistory: true}}})
			if err != nil {
				t.Fatal(err)
			}
			frame := make([]byte, 5+len(raw))
			binary.BigEndian.PutUint32(frame[1:5], uint32(len(raw)))
			copy(frame[5:], raw)
			if err := framer.WriteData(1, false, frame); err != nil {
				t.Fatal(err)
			}
			gotHeaders := make(chan struct{}, 1)
			go func() {
				for {
					frame, err := framer.ReadFrame()
					if err != nil {
						return
					}
					if _, ok := frame.(*http2.HeadersFrame); ok {
						select {
						case gotHeaders <- struct{}{}:
						default:
						}
					}
				}
			}()
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			for !hub.Capabilities(ctx, 1).Connected {
				select {
				case <-ctx.Done():
					t.Fatal("query stream did not connect")
				case <-time.After(time.Millisecond):
				}
			}
			queryDone := make(chan error, 1)
			go func() {
				_, err := hub.History(ctx, 1, virt.HistoryQuery{VMID: 101, Timeframe: "hour", Consolidation: "AVERAGE"})
				queryDone <- err
			}()
			select {
			case <-gotHeaders:
			case <-ctx.Done():
				t.Fatal("query stream did not begin writing")
			}
			shutdownCtx, stop := context.WithTimeout(t.Context(), 4*time.Second)
			defer stop()
			shutdownDone := make(chan error, 1)
			go func() { shutdownDone <- s.shutdown(shutdownCtx) }()
			select {
			case err := <-shutdownDone:
				if err != nil {
					t.Fatalf("query stream prevented graceful shutdown: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown waited for peer to close its connection")
			}
			if err := <-queryDone; !errors.Is(err, nodesession.ErrOffline) {
				t.Fatalf("pending query survived shutdown: %v", err)
			}
		})
	}
}

func TestShutdownDrainsHTTPRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			_, _ = io.WriteString(w, "completed")
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	rpc := noderpc.New(nil, nodesession.New(nil), nil)
	defer rpc.Close()
	s := &HTTPServer{server: server.Config, rpc: rpc}
	requestDone := make(chan error, 1)
	go func() {
		response, err := server.Client().Get(server.URL)
		if err == nil {
			defer response.Body.Close()
			var body []byte
			body, err = io.ReadAll(response.Body)
			if err == nil && string(body) != "completed" {
				err = errors.New("HTTP response was interrupted")
			}
		}
		requestDone <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.shutdown(ctx) }()
	select {
	case err := <-done:
		close(release)
		t.Fatalf("shutdown did not drain active request: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
}
