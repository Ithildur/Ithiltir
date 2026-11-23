package httpserver

import (
	"encoding/json/v2"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/store"
	pgtest "dash/internal/testutil/postgres"
)

func TestIntegrationFrontMetrics(t *testing.T) {
	db := pgtest.NewDB(t)
	st := store.New(db, nil, time.UTC, pgtest.ConfigCipher(t))
	handler := newTestHandler(t, st)
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
			t.Fatalf("%s %s: got %d %s, want %d", method, path, w.Code, w.Body, status)
		}
		return w.Body.Bytes()
	}
	var session struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(call("POST", "/api/auth/login", `{"password":"test-password","persistence":"session"}`, "", 200), &session); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	const path = "/api/front/metrics"
	for _, srv := range []model.Server{
		{Name: "public", Secret: "visible-secret", IsGuestVisible: true},
		{Name: "private", Secret: "hidden-secret"},
		{Name: "deleted", Secret: "deleted-secret", IsGuestVisible: true, IsDeleted: true},
	} {
		if err := db.Create(&srv).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.ServerCurrentMetric{ServerID: srv.ID, CollectedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
		nodePath := path + "/" + strconv.FormatInt(srv.ID, 10)
		if srv.IsDeleted {
			call("GET", nodePath, "", session.AccessToken, 404)
		} else {
			call("GET", nodePath, "", session.AccessToken, 200)
		}
		if !srv.IsGuestVisible || srv.IsDeleted {
			call("GET", nodePath, "", "", 404)
		}
	}
	call("GET", path+"/0", "", "", 400)
	call("GET", path+"/999999", "", "", 404)
	for _, query := range []string{"", "?limit=1", "?limit=1&offset=10"} {
		var poll struct {
			SentAt time.Time          `json:"sent_at"`
			Nodes  []metrics.NodeView `json:"nodes"`
		}
		start := time.Now()
		if err := json.Unmarshal(call("GET", path+query, "", "", 200), &poll); err != nil {
			t.Fatal(err)
		}
		if poll.SentAt.Before(start) || poll.SentAt.After(time.Now()) || poll.Nodes == nil {
			t.Fatalf("invalid poll envelope for %q: %+v", query, poll)
		}
		if query == "?limit=1&offset=10" {
			if len(poll.Nodes) != 0 {
				t.Fatal("expected empty page")
			}
			continue
		}
		if len(poll.Nodes) != 1 || poll.Nodes[0].Node.Title != "public" {
			t.Fatalf("unexpected visible nodes: %+v", poll.Nodes)
		}
		var node metrics.NodeView
		if err := json.Unmarshal(call("GET", path+"/"+poll.Nodes[0].Node.ID, "", "", 200), &node); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(node, poll.Nodes[0]) {
			t.Fatal("single-node view differs from list entry")
		}
	}
}
