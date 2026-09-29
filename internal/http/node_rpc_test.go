package httpserver

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/nodewire"
	"dash/internal/store"
	nodestore "dash/internal/store/node"
	pgtest "dash/internal/testutil/postgres"
	"dash/internal/virt"
)

func TestIntegrationNodeHTTPAndRPC(t *testing.T) {
	db := pgtest.NewDB(t)
	st := store.New(db, nil, time.UTC, pgtest.ConfigCipher(t))
	httpNode, err := st.Node.CreateNode(t.Context(), "node-secret")
	if err != nil {
		t.Fatal(err)
	}
	rpcNode, err := st.Node.CreateNode(t.Context(), "rpc-secret")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(newTestHandler(t, st))
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetHTTP1(true)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	defer server.Close()
	conn, err := grpc.NewClient(strings.TrimPrefix(server.URL, "http://"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := nodewire.NewNodeClient(conn)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-node-secret", "rpc-secret", "x-forwarded-for", "203.0.113.9")
	post := func(path string, raw []byte, want int) []byte {
		t.Helper()
		request, err := http.NewRequestWithContext(t.Context(), "POST", server.URL+"/api/node/"+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Node-Secret", "node-secret")
		request.Header.Set("X-Forwarded-For", "203.0.113.9")
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if response.StatusCode != want {
			t.Fatalf("HTTP %s: %d %s", path, response.StatusCode, body)
		}
		return body
	}
	identity, err := client.Identify(ctx, &nodewire.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	var httpIdentity struct {
		InstallID string `json:"install_id"`
	}
	if err := json.Unmarshal(post("identity", []byte("{}"), 200), &httpIdentity); err != nil || httpIdentity.InstallID != identity.InstallId {
		t.Fatalf("identity mismatch: %v", err)
	}
	now := time.Now().UTC()
	snapshot := metrics.StaticMetrics{Version: "1.0.0", Timestamp: now, ReportIntervalSeconds: 3, System: metrics.StaticSystem{Hostname: "node", OS: "linux", Platform: "debian", PlatformVersion: "13", KernelVersion: "6", Arch: "amd64"}}
	snapshot.Memory.SwapTotal = new(int64(0))
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	post("static", raw, 200)
	if _, err := client.Static(ctx, &nodewire.Report{Json: raw}); err != nil {
		t.Fatal(err)
	}
	for round := range 2 {
		report := metrics.NodeReport{Version: "1.0.0", Hostname: "node", Timestamp: time.Now().UTC(), Metrics: metrics.Metrics{
			CPU:     metrics.CPUMetrics{UsageRatio: 0.25},
			Memory:  metrics.MemoryMetrics{Total: 4096, Used: 1024, Available: 3072, UsedRatio: 0.25},
			System:  metrics.SystemMetrics{Uptime: "1h", UptimeSeconds: 3600},
			Network: []metrics.NetIOMetrics{{Name: "eth0", BytesRecv: int64(1000 + round*100), BytesSent: int64(2000 + round*200), RecvRateBytesPerSec: 10, SentRateBytesPerSec: 20}},
		}}
		raw, err = json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		httpResponse := post("metrics", raw, 200)
		rpcResponse, err := client.Metrics(ctx, &nodewire.Report{Json: raw})
		if err != nil {
			t.Fatal(err)
		}
		if string(httpResponse) != string(rpcResponse.Json) && strings.TrimSpace(string(httpResponse)) != string(rpcResponse.Json) {
			t.Fatalf("response differs: HTTP=%s RPC=%s", httpResponse, rpcResponse.Json)
		}
	}
	var first, second model.ServerCurrentMetric
	if err := db.First(&first, "server_id = ?", httpNode.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&second, "server_id = ?", rpcNode.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.MetricValues, second.MetricValues) || !reflect.DeepEqual(first.MetricRuntime, second.MetricRuntime) {
		t.Fatal("transport changed stored metrics")
	}
	for _, id := range []int64{httpNode.ID, rpcNode.ID} {
		var count int64
		if err := db.Table("server_metrics").Where("server_id = ?", id).Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("metrics count=%d error=%v", count, err)
		}
		var node model.Server
		if err := db.First(&node, id).Error; err != nil || node.IP == nil || *node.IP != "203.0.113.9" || node.SwapTotal == nil || *node.SwapTotal != 0 {
			t.Fatalf("static/IP observation changed: %+v %v", node, err)
		}
	}
	vmTime := time.Now().UTC().Truncate(time.Microsecond)
	vmSnapshot := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve", CollectedAt: vmTime, LastSuccessAt: &vmTime, TTLSeconds: 90, Status: "ok", VMs: []virt.VM{{ID: 101, Name: "guest", Status: "running"}}}
	raw, err = json.Marshal(vmSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	post("virt", raw, 204)
	for range 2 {
		if _, err := client.Virt(ctx, &nodewire.Report{Json: raw}); err != nil {
			t.Fatal(err)
		}
	}
	a, err := st.Node.Virt(ctx, httpNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.Node.Virt(ctx, rpcNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Snapshot, b.Snapshot) {
		t.Fatal("VM snapshot changed by transport")
	}
	post("metrics", []byte("{}"), 422)
	if _, err := client.Metrics(ctx, &nodewire.Report{Json: []byte("{}")}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid report: %v", err)
	}
	if _, err := client.Metrics(ctx, &nodewire.Report{Json: bytes.Repeat([]byte(" "), 1<<20+1)}); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("report limit: %v", err)
	}
	stream, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&nodewire.NodeMessage{Body: &nodewire.NodeMessage_Capabilities{Capabilities: &nodewire.Capabilities{Version: "1.0.0", PveHistory: true}}}); err != nil {
		t.Fatal(err)
	}
	login, err := server.Client().Post(server.URL+"/api/auth/login", "application/json", strings.NewReader(`{"password":"test-password","persistence":"session"}`))
	if err != nil {
		t.Fatal(err)
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.UnmarshalRead(login.Body, &tokens); err != nil {
		t.Fatal(err)
	}
	login.Body.Close()
	adminGet := func(path string, authenticated bool) (*http.Response, []byte) {
		t.Helper()
		request, err := http.NewRequestWithContext(ctx, "GET", server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if authenticated {
			request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response, raw
	}
	base := "/api/admin/nodes/" + strconv.FormatInt(rpcNode.ID, 10) + "/virt"
	until := time.Now().Add(2 * time.Second)
	for {
		response, body := adminGet(base+"/capabilities", true)
		if response.StatusCode != 200 {
			t.Fatalf("capabilities: %d %s", response.StatusCode, body)
		}
		if bytes.Contains(body, []byte(`"connected":true`)) {
			break
		}
		if time.Now().After(until) {
			t.Fatal("query session did not connect")
		}
		time.Sleep(time.Millisecond)
	}
	if response, _ := adminGet(base+"/vms/101/history", false); response.StatusCode != 401 {
		t.Fatal("history missing admin boundary")
	}
	queryDone := make(chan error, 1)
	go func() {
		message, err := stream.Recv()
		if err != nil {
			queryDone <- err
			return
		}
		query := message.GetHistory()
		if query == nil || query.VmId != 101 || query.Timeframe != "hour" || query.TimeoutMs == 0 {
			queryDone <- fmt.Errorf("invalid reverse query: %v", message)
			return
		}
		raw, err := json.Marshal(virt.History{Source: "pve_rrd", VMID: 101, Timeframe: query.Timeframe, Consolidation: query.Consolidation, CollectedAt: time.Now().UTC(), Points: []virt.HistoryPoint{}})
		if err == nil {
			err = stream.Send(&nodewire.NodeMessage{Id: message.Id, Body: &nodewire.NodeMessage_Result{Result: &nodewire.QueryResult{Json: raw}}})
		}
		queryDone <- err
	}()
	response, body := adminGet(base+"/vms/101/history", true)
	if response.StatusCode != 200 || !bytes.Contains(body, []byte(`"source":"pve_rrd"`)) {
		t.Fatalf("history: %d %s", response.StatusCode, body)
	}
	if err := <-queryDone; err != nil {
		t.Fatal(err)
	}
	rotated := "rotated-rpc-secret"
	if err := st.Node.UpdateNode(ctx, rpcNode.ID, nodestore.NodeUpdate{Secret: &rotated}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Identify(ctx, &nodewire.Empty{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("rotated secret accepted: %v", err)
	}
	closed := make(chan error, 1)
	go func() {
		for {
			_, err := stream.Recv()
			if err != nil {
				closed <- err
				return
			}
		}
	}()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("old query session survived key rotation")
	}
	if err := st.Node.DeleteNode(ctx, rpcNode.ID); err != nil {
		t.Fatal(err)
	}
	rotatedCtx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("x-node-secret", rotated))
	if _, err := client.Identify(rotatedCtx, &nodewire.Empty{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("deleted node accepted: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if body := post("virt", raw, 503); !bytes.Contains(body, []byte(`"virt_unavailable"`)) {
		t.Fatalf("HTTP storage failure changed: %s", body)
	}
	activeCtx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("x-node-secret", httpNode.Secret))
	if _, err := client.Virt(activeCtx, &nodewire.Report{Json: raw}); status.Code(err) != codes.Unavailable || status.Convert(err).Message() != "virt_unavailable" {
		t.Fatalf("RPC storage failure: %v", err)
	}
}
