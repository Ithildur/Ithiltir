package virt_test

import (
	"bytes"
	"encoding/json/v2"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	nodevirt "dash/internal/http/api/node/virt"
	publicvirt "dash/internal/http/api/virt"
	"dash/internal/nodeingest"
	"dash/internal/store"
	nodestore "dash/internal/store/node"
	pgtest "dash/internal/testutil/postgres"
	"dash/internal/virt"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func TestIntegrationVirtLifecycle(t *testing.T) {
	db := pgtest.NewDB(t)
	st := store.New(db, nil, time.UTC, pgtest.ConfigCipher(t))
	server, err := st.Node.CreateNode(t.Context(), "virt-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	root := routes.NewBlueprint()
	ingest := nodeingest.New(st.Node, st.Metric, st.Front, st.Alert, nil, 0)
	root.Include("/api/node/virt", nodevirt.Router(ingest, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(401) })))
	root.Include("/api/virt", publicvirt.Router(st, nil))
	router, err := routes.NewHandler(root.Routes(), routes.HandlerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, secret string, raw []byte, admin bool, want int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Node-Secret", secret)
		if admin {
			req = req.WithContext(routes.WithAuthenticated(req.Context()))
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s; want %d", method, path, w.Code, w.Body, want)
		}
		return w.Body.Bytes()
	}
	path := "/api/virt/" + strconv.FormatInt(server.ID, 10)
	call("GET", path, "", nil, false, 403)
	call("GET", path, "", nil, true, 200)
	now := time.Now().UTC().Truncate(time.Microsecond)
	snapshot := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve", CollectedAt: now, LastSuccessAt: &now, TTLSeconds: 90, Status: "ok", VMs: []virt.VM{{ID: 100, Name: "guest", Status: "running", IPs: []string{"192.0.2.10", "2001:db8::10"}}}}
	snapshot.VMs[0].NetRateLimits = map[string]float64{"net0": 104.86, "net1": 0}
	vmTime := now.Add(-time.Second)
	snapshot.VMs[0].CollectedAt, snapshot.VMs[0].TTLSeconds, snapshot.VMs[0].CPU = &vmTime, 90, new(0.25)
	expiredVMTime := now.Add(-2 * time.Minute)
	snapshot.VMs = append(snapshot.VMs, virt.VM{ID: 101, Name: "old", Status: "running", QMPStatus: "running", CollectedAt: &expiredVMTime, TTLSeconds: 90, CPU: new(0.75), Memory: new(int64(1024)), NetIn: new(int64(100)), Uptime: new(int64(600))})
	for i := range 1022 {
		snapshot.VMs[0].IPs = append(snapshot.VMs[0].IPs, "10.0."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256))
	}
	ipTime := now.Add(-5 * time.Minute)
	snapshot.VMs[0].IPsCollectedAt, snapshot.VMs[0].IPsTTLSeconds = &ipTime, 900
	post := func(sample virt.Snapshot, secret string, want int) {
		t.Helper()
		raw, err := json.Marshal(sample)
		if err != nil {
			t.Fatal(err)
		}
		call("POST", "/api/node/virt", secret, raw, false, want)
	}
	post(snapshot, "wrong", 401)
	post(snapshot, server.Secret, 204)
	post(snapshot, server.Secret, 204)
	old := snapshot
	old.CollectedAt = now.Add(-time.Minute)
	old.LastSuccessAt = &old.CollectedAt
	old.VMs = []virt.VM{}
	post(old, server.Secret, 204)
	var view nodestore.VirtView
	if err := json.Unmarshal(call("GET", path, "", nil, true, 200), &view); err != nil {
		t.Fatal(err)
	}
	if view.Stale || view.Snapshot == nil || !view.Snapshot.CollectedAt.Equal(now) || len(view.Snapshot.VMs) != 2 {
		t.Fatalf("latest snapshot: %+v", view)
	}
	firstReceived := *view.ReceivedAt
	if live := view.Snapshot.VMs[0]; live.CollectedAt == nil || !live.CollectedAt.Equal(vmTime) || live.CPU == nil || *live.CPU != 0.25 {
		t.Fatalf("fresh VM lost its observation time or metrics: %+v", live)
	}
	if expired := view.Snapshot.VMs[1]; expired.Status != "unknown" || expired.CPU != nil || expired.Memory != nil || expired.NetIn != nil || expired.Uptime != nil || !expired.CollectedAt.Equal(expiredVMTime) {
		t.Fatalf("fresh snapshot exposed expired VM metrics: %+v", expired)
	}
	if !maps.Equal(view.Snapshot.VMs[0].NetRateLimits, snapshot.VMs[0].NetRateLimits) {
		t.Fatal("NIC rate limits were lost or changed in snapshot")
	}
	if view.Snapshot.VMs[0].IPsCollectedAt == nil || !view.Snapshot.VMs[0].IPsCollectedAt.Equal(ipTime) || view.Snapshot.VMs[0].IPsTTLSeconds != 900 {
		t.Fatal("IP freshness was lost or refreshed with hot data")
	}
	for _, vm := range []virt.VM{
		{ID: 100, Status: "running", TTLSeconds: 90},
		{ID: 100, Status: "running", CollectedAt: &vmTime},
		{ID: 100, Status: "running", CollectedAt: new(now.Add(time.Second)), TTLSeconds: 90},
		{ID: 100, Status: "running", NetRateLimits: map[string]float64{"net0": -1}},
		{ID: 100, Status: "running", IPsTTLSeconds: 900},
		{ID: 100, Status: "running", IPsCollectedAt: &ipTime},
		{ID: 100, Status: "running", IPsCollectedAt: new(now.Add(time.Second)), IPsTTLSeconds: 900},
	} {
		invalid := snapshot
		invalid.VMs = []virt.VM{vm}
		post(invalid, server.Secret, 400)
	}
	if len(view.Snapshot.VMs[0].IPs) != 1024 || view.Snapshot.VMs[0].IPs[1] != "2001:db8::10" {
		t.Fatal("guest IPs were lost in snapshot")
	}
	for _, ips := range [][]string{{"not-an-ip"}, {"127.0.0.1"}, {"fe80::1"}, {"ff02::1"}, {"192.0.2.10", "::ffff:192.0.2.10"}, append(slices.Clone(snapshot.VMs[0].IPs), "198.51.100.1")} {
		invalid := snapshot
		invalid.VMs = []virt.VM{snapshot.VMs[0]}
		invalid.VMs[0].IPs = ips
		post(invalid, server.Secret, 400)
	}
	post(snapshot, server.Secret, 204)
	view, err = st.Node.Virt(t.Context(), server.ID)
	if err != nil || !view.ReceivedAt.Equal(firstReceived) {
		t.Fatal("duplicate refreshed receipt time", err)
	}
	var hostRows int64
	if err := db.Table("server_current_metrics").Where("server_id = ?", server.ID).Count(&hostRows).Error; err != nil || hostRows != 0 {
		t.Fatalf("VM ingest touched host uptime: rows=%d error=%v", hostRows, err)
	}
	invalid := snapshot
	invalid.VMs = append(invalid.VMs, invalid.VMs[0])
	post(invalid, server.Secret, 400)
	call("POST", "/api/node/virt", server.Secret, bytes.Repeat([]byte(" "), virt.MaxBytes+1), false, 413)
	failed := snapshot
	failed.CollectedAt = now.Add(time.Second)
	failed.LastSuccessAt = new(now.Add(-5 * time.Minute))
	failed.Status, failed.Error = "error", "collection timed out"
	post(failed, server.Secret, 204)
	view, err = st.Node.Virt(t.Context(), server.ID)
	if err != nil || !view.Stale || view.Snapshot.Status != "error" || len(view.Snapshot.VMs) != 2 {
		t.Fatal("failed collection lost its previous inventory or freshness", err)
	}
	empty := snapshot
	empty.CollectedAt = now.Add(2 * time.Second)
	empty.LastSuccessAt = &empty.CollectedAt
	empty.VMs = []virt.VM{}
	post(empty, server.Secret, 204)
	view, err = st.Node.Virt(t.Context(), server.ID)
	if err != nil || view.Stale || view.Snapshot.Status != "ok" || len(view.Snapshot.VMs) != 0 {
		t.Fatal("successful empty inventory was not published", err)
	}
	rotated := "rotated-virt-secret"
	if err := st.Node.UpdateNode(t.Context(), server.ID, nodestore.NodeUpdate{Secret: &rotated}); err != nil {
		t.Fatal(err)
	}
	post(snapshot, server.Secret, 401)
	post(snapshot, rotated, 204)
	restarted := store.New(db, nil, time.UTC, pgtest.ConfigCipher(t))
	if err := restarted.Node.RebuildServerCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err = restarted.Node.Virt(t.Context(), server.ID)
	if err != nil || view.Snapshot != nil || view.ReceivedAt != nil || !view.Stale {
		t.Fatalf("restart retained VM data: %+v, %v", view, err)
	}
	if err := st.Node.DeleteNode(t.Context(), server.ID); err != nil {
		t.Fatal(err)
	}
	post(snapshot, rotated, 401)
	call("GET", path, "", nil, true, 404)
}
