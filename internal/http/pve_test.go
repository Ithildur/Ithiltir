package httpserver

import (
	"encoding/json/v2"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"dash/internal/store"
	nodestore "dash/internal/store/node"
	pgtest "dash/internal/testutil/postgres"
	"dash/internal/virt"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestIntegrationPVEGuestAccess(t *testing.T) {
	for _, mode := range []string{"memory", "redis"} {
		t.Run(mode, func(t *testing.T) {
			var client *redis.Client
			if mode == "redis" {
				cache := miniredis.RunT(t)
				client = redis.NewClient(&redis.Options{Addr: cache.Addr(), ContextTimeoutEnabled: true})
				t.Cleanup(func() { _ = client.Close() })
			}
			testPVEGuestAccess(t, client)
		})
	}
}

func testPVEGuestAccess(t *testing.T, client *redis.Client) {
	t.Helper()
	st := store.New(pgtest.NewDB(t), client, time.UTC, pgtest.ConfigCipher(t))
	visible, err := st.Node.CreateNode(t.Context(), "pve-visible-secret")
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := st.Node.CreateNode(t.Context(), "pve-hidden-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Node.UpdateNode(t.Context(), visible.ID, nodestore.NodeUpdate{IsGuestVisible: new(true)}); err != nil {
		t.Fatal(err)
	}
	if err := st.Node.UpdateNode(t.Context(), hidden.ID, nodestore.NodeUpdate{IsGuestVisible: new(false)}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	snapshot := virt.Snapshot{Schema: 1, Provider: "pve", Host: "pve-a", CollectedAt: now, LastSuccessAt: &now, TTLSeconds: 90, Status: "ok", VMs: []virt.VM{{ID: 100, Name: "private-vm", Status: "running", IPs: []string{"192.168.1.10"}}}}
	for _, node := range []struct {
		id     int64
		secret string
	}{{visible.ID, visible.Secret}, {hidden.ID, hidden.Secret}} {
		if err := st.Node.SaveVirt(t.Context(), node.id, node.secret, snapshot, now); err != nil {
			t.Fatal(err)
		}
	}
	handler := newTestHandler(t, st)
	if err := st.Node.RebuildServerCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, token string, status int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s; want %d", method, path, w.Code, w.Body, status)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("VM API missing no-store")
		}
		if status != 200 && strings.Contains(w.Body.String(), "private-vm") {
			t.Fatal("denied request leaked VM data")
		}
		return w.Body.Bytes()
	}
	var session struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(call("POST", "/api/auth/login", `{"password":"test-password","persistence":"session"}`, "", 200), &session); err != nil {
		t.Fatal(err)
	}
	if session.AccessToken == "" {
		t.Fatal("login returned no token")
	}
	path := "/api/virt/" + strconv.FormatInt(visible.ID, 10)
	hiddenPath := "/api/virt/" + strconv.FormatInt(hidden.ID, 10)
	history := "/vms/100/history"
	settingsPath := "/api/admin/system/settings"
	assertPolicy := func(want bool) {
		t.Helper()
		for _, endpoint := range []struct{ path, token string }{{"/api/statistics/access", ""}, {settingsPath, session.AccessToken}} {
			var policy struct {
				Visible *bool `json:"pve_guest_visible"`
			}
			if err := json.Unmarshal(call("GET", endpoint.path, "", endpoint.token, 200), &policy); err != nil {
				t.Fatal(err)
			}
			if policy.Visible == nil || *policy.Visible != want {
				t.Fatalf("%s policy = %+v, want %v", endpoint.path, policy, want)
			}
		}
	}
	assertPolicy(false)
	for _, token := range []string{"", "invalid-token"} {
		call("GET", path, "", token, 403)
		call("GET", path+history, "", token, 403)
	}
	call("GET", path, "", session.AccessToken, 200)
	call("GET", hiddenPath, "", session.AccessToken, 200)
	call("GET", path+history, "", session.AccessToken, 503)
	call("PATCH", settingsPath, `{"pve_guest_visible":true}`, session.AccessToken, 204)
	assertPolicy(true)
	var view nodestore.VirtView
	if err := json.Unmarshal(call("GET", path, "", "", 200), &view); err != nil {
		t.Fatal(err)
	}
	if view.Snapshot == nil || !view.Snapshot.CollectedAt.Equal(now) || len(view.Snapshot.VMs) != 1 || view.Snapshot.VMs[0].IPs[0] != "192.168.1.10" {
		t.Fatalf("public snapshot = %+v", view)
	}
	if view.VMCPUThreshold != 80 || view.VMMemoryThreshold != 90 || view.VMMemoryEnabled {
		t.Fatalf("unexpected VM defaults: %+v", view)
	}
	nodePath := "/api/admin/nodes/" + strconv.FormatInt(visible.ID, 10)
	call("PATCH", nodePath, `{"vm_cpu_threshold":75}`, "", 401)
	for _, body := range []string{`{"vm_cpu_threshold":0}`, `{"vm_memory_threshold":101}`, `{"vm_cpu_threshold":80.5}`} {
		call("PATCH", nodePath, body, session.AccessToken, 400)
	}
	call("PATCH", nodePath, `{"vm_cpu_threshold":75,"vm_memory_threshold":85,"vm_memory_enabled":true}`, session.AccessToken, 204)
	if err := json.Unmarshal(call("GET", path, "", "", 200), &view); err != nil {
		t.Fatal(err)
	}
	if view.VMCPUThreshold != 75 || view.VMMemoryThreshold != 85 || !view.VMMemoryEnabled || view.Snapshot == nil {
		t.Fatalf("updated VM settings not reflected in snapshot: %+v", view)
	}
	var nodes []struct {
		ID      int64 `json:"id"`
		CPU     int   `json:"vm_cpu_threshold"`
		Memory  int   `json:"vm_memory_threshold"`
		Enabled bool  `json:"vm_memory_enabled"`
	}
	if err := json.Unmarshal(call("GET", "/api/admin/nodes", "", session.AccessToken, 200), &nodes); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, node := range nodes {
		if node.ID != visible.ID {
			continue
		}
		found = true
		if node.CPU != 75 || node.Memory != 85 || !node.Enabled {
			t.Fatalf("saved VM settings missing from admin list: %+v", node)
		}
	}
	if !found {
		t.Fatalf("saved VM settings missing from admin list: %+v", nodes)
	}
	call("PATCH", nodePath, `{"vm_memory_enabled":false}`, session.AccessToken, 204)
	if err := json.Unmarshal(call("GET", path, "", "", 200), &view); err != nil {
		t.Fatal(err)
	}
	if view.VMCPUThreshold != 75 || view.VMMemoryThreshold != 85 || view.VMMemoryEnabled {
		t.Fatalf("partial VM update lost thresholds: %+v", view)
	}
	call("GET", path+history, "", "", 503)
	call("GET", path+"/vms/100/history?timeframe=invalid", "", "", 400)
	call("GET", hiddenPath, "", "", 404)
	call("GET", hiddenPath+history, "", "", 404)
	call("PUT", settingsPath, `{"history_guest_access_mode":"disabled","dash_update_channel":"release","dash_update_mode":"manual","logo_url":"/brandlogo.svg","page_title":"Ithiltir","topbar_text":"Ops"}`, session.AccessToken, 204)
	assertPolicy(true)
	call("PATCH", settingsPath, `{"pve_guest_visible":false}`, session.AccessToken, 204)
	assertPolicy(false)
	call("GET", path, "", "", 403)
	call("GET", path+history, "", "", 403)
	call("GET", path, "", session.AccessToken, 200)
}
