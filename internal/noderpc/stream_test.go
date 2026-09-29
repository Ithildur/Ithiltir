package noderpc

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
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/protobuf/proto"

	"dash/internal/model"
	"dash/internal/nodeingest"
	"dash/internal/nodesession"
	"dash/internal/nodewire"
	"dash/internal/serverid"
	"dash/internal/store"
	"dash/internal/virt"
)

// Keep TCP open and consume frames without granting any DATA window to Connect.
// Logical cancellation alone would pass query assertions while leaking the stream.
func TestBlockedQueryStreamReleasesTransport(t *testing.T) {
	for _, reason := range []string{"replacement", "credentials", "write_timeout"} {
		t.Run(reason, func(t *testing.T) {
			st := store.New(nil, nil, time.UTC, nil)
			if err := st.Node.SyncServerCache(t.Context(), model.Server{ID: 1, Secret: "secret"}); err != nil {
				t.Fatal(err)
			}
			ingest := nodeingest.New(st.Node, st.Metric, st.Front, st.Alert, serverid.New(filepath.Join(t.TempDir(), "id")), 17)
			hub := nodesession.New(ingest.Authenticate)
			rpc := New(ingest, hub, nil)
			ended := make(chan string, 8)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rpc.ServeHTTP(w, r)
				ended <- r.Header.Get("test-stream")
			}))
			server.Config.Protocols = new(http.Protocols)
			server.Config.Protocols.SetUnencryptedHTTP2(true)
			server.Start()
			defer func() { _ = server.Config.Close(); rpc.Close(); server.Close() }()
			peer := newQueryPeer(t, server.URL)
			defer peer.conn.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			peer.open(t, 1, nodewire.Node_Connect_FullMethodName, "secret", &nodewire.NodeMessage{Body: &nodewire.NodeMessage_Capabilities{Capabilities: &nodewire.Capabilities{PveHistory: true, Version: "first"}}})
			for hub.Capabilities(ctx, 1).Version != "first" {
				select {
				case <-ctx.Done():
					t.Fatal("session did not connect")
				case <-time.After(time.Millisecond):
				}
			}
			queryDone := make(chan error, 1)
			go func() {
				_, err := hub.History(ctx, 1, virt.HistoryQuery{VMID: 101, Timeframe: "hour", Consolidation: "AVERAGE"})
				queryDone <- err
			}()
			peer.wait(t, ctx, func(frame peerFrame) bool {
				return frame.headers && frame.stream == 1
			})
			secret := "secret"
			switch reason {
			case "replacement":
				peer.open(t, 3, nodewire.Node_Connect_FullMethodName, secret, &nodewire.NodeMessage{Body: &nodewire.NodeMessage_Capabilities{Capabilities: &nodewire.Capabilities{Version: "replacement"}}})
			case "credentials":
				secret = "rotated"
				if err := st.Node.SyncServerCache(ctx, model.Server{ID: 1, Secret: secret}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case id := <-ended:
				if id != "1" {
					t.Fatalf("unexpected stream ended: %s", id)
				}
			case <-ctx.Done():
				t.Fatal("HTTP stream retained after session ended")
			}
			if err := <-queryDone; !errors.Is(err, nodesession.ErrOffline) {
				t.Fatalf("pending query survived stream: %v", err)
			}
			// A unary report connection must remain usable when its query stream
			// is reset. Grant a window only to this new unary call.
			peer.open(t, 5, nodewire.Node_Identify_FullMethodName, secret, &nodewire.Empty{})
			if err := peer.framer.WriteWindowUpdate(5, 65535); err != nil {
				t.Fatal(err)
			}
			var response []byte
			peer.wait(t, ctx, func(frame peerFrame) bool {
				if frame.stream != 5 {
					return false
				}
				response = append(response, frame.data...)
				return frame.headers && frame.ended
			})
			var identity nodewire.Identity
			if len(response) < 5 || proto.Unmarshal(response[5:], &identity) != nil || identity.ProtocolVersion != 1 {
				t.Fatalf("shared connection could not identify after query reset: %x", response)
			}
		})
	}
}

type queryPeer struct {
	conn   net.Conn
	framer *http2.Framer
	frames chan peerFrame
	host   string
}

type peerFrame struct {
	stream  uint32
	headers bool
	ended   bool
	data    []byte
}

func newQueryPeer(t *testing.T, url string) *queryPeer {
	t.Helper()
	host := strings.TrimPrefix(url, "http://")
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
		t.Fatal(err)
	}
	p := &queryPeer{conn: conn, framer: http2.NewFramer(conn, conn), frames: make(chan peerFrame, 64), host: host}
	if err := p.framer.WriteSettings(http2.Setting{ID: http2.SettingInitialWindowSize, Val: 0}); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.frames)
		for {
			frame, err := p.framer.ReadFrame()
			if err != nil {
				return
			}
			event := peerFrame{stream: frame.Header().StreamID}
			switch frame := frame.(type) {
			case *http2.HeadersFrame:
				event.headers, event.ended = true, frame.StreamEnded()
			case *http2.DataFrame:
				event.data = bytes.Clone(frame.Data())
			}
			select {
			case p.frames <- event:
			case <-t.Context().Done():
				return
			}
		}
	}()
	return p
}

func (p *queryPeer) open(t *testing.T, id uint32, method, secret string, message proto.Message) {
	t.Helper()
	var headers bytes.Buffer
	encoder := hpack.NewEncoder(&headers)
	for _, field := range []hpack.HeaderField{
		{Name: ":method", Value: "POST"}, {Name: ":scheme", Value: "http"}, {Name: ":authority", Value: p.host},
		{Name: ":path", Value: method}, {Name: "content-type", Value: "application/grpc"},
		{Name: "x-node-secret", Value: secret}, {Name: "test-stream", Value: strconv.FormatUint(uint64(id), 10)},
	} {
		if err := encoder.WriteField(field); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.framer.WriteHeaders(http2.HeadersFrameParam{StreamID: id, BlockFragment: headers.Bytes(), EndHeaders: true}); err != nil {
		t.Fatal(err)
	}
	raw, err := proto.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 5+len(raw))
	binary.BigEndian.PutUint32(data[1:5], uint32(len(raw)))
	copy(data[5:], raw)
	if err := p.framer.WriteData(id, method != nodewire.Node_Connect_FullMethodName, data); err != nil {
		t.Fatal(err)
	}
}

func (p *queryPeer) wait(t *testing.T, ctx context.Context, match func(peerFrame) bool) {
	t.Helper()
	for {
		select {
		case frame, ok := <-p.frames:
			if !ok {
				t.Fatal("HTTP/2 connection closed")
			}
			if match(frame) {
				return
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
