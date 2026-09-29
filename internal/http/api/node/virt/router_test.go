package virt_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	adminvirt "dash/internal/http/api/admin/nodes/id/virt"
	nodevirt "dash/internal/http/api/node/virt"
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
	root.Include("/api/admin/nodes/{id}/virt", adminvirt.Router(st.Node, nil), routes.IncludeAuth(routes.AuthRequired))
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
	path := "/api/admin/nodes/" + strconv.FormatInt(server.ID, 10) + "/virt"
	call("GET", path, "", nil, false, 401)
	call("GET", path, "", nil, true, 200)
	now := time.Now().UTC().Truncate(time.Microsecond)
	snapshot := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve", CollectedAt: now, LastSuccessAt: &now, TTLSeconds: 90, Status: "ok", VMs: []virt.VM{{ID: 100, Name: "guest", Status: "running", IPs: []string{"192.0.2.10", "2001:db8::10"}}}}
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
	if view.Stale || view.Snapshot == nil || !view.Snapshot.CollectedAt.Equal(now) || len(view.Snapshot.VMs) != 1 {
		t.Fatalf("latest snapshot: %+v", view)
	}
	firstReceived := *view.ReceivedAt
	if view.Snapshot.VMs[0].IPsCollectedAt == nil || !view.Snapshot.VMs[0].IPsCollectedAt.Equal(ipTime) || view.Snapshot.VMs[0].IPsTTLSeconds != 900 {
		t.Fatal("IP freshness was lost or refreshed with hot data")
	}
	for _, vm := range []virt.VM{
		{ID: 100, Status: "running", IPsTTLSeconds: 900},
		{ID: 100, Status: "running", IPsCollectedAt: &ipTime},
		{ID: 100, Status: "running", IPsCollectedAt: new(now.Add(time.Second)), IPsTTLSeconds: 900},
	} {
		invalid := snapshot
		invalid.VMs = []virt.VM{vm}
		post(invalid, server.Secret, 400)
	}
	if len(view.Snapshot.VMs[0].IPs) != 2 || view.Snapshot.VMs[0].IPs[1] != "2001:db8::10" {
		t.Fatal("guest IPs were lost in persistence")
	}
	for _, ips := range [][]string{{"not-an-ip"}, {"127.0.0.1"}, {"fe80::1"}, {"ff02::1"}, {"192.0.2.10", "::ffff:192.0.2.10"}, make([]string, 129)} {
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
	if err != nil || !view.Stale || view.Snapshot.Status != "error" || len(view.Snapshot.VMs) != 1 {
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
	if err := st.Node.DeleteNode(t.Context(), server.ID); err != nil {
		t.Fatal(err)
	}
	post(snapshot, rotated, 401)
	call("GET", path, "", nil, true, 404)
	var count int64
	if err := db.Table("server_virt").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("deleted node retained VM snapshot: %d %v", count, err)
	}
}
