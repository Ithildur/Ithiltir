package uptime

import (
	"encoding/json/v2"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dash/internal/config"
	"dash/internal/migrate"
	"dash/internal/store"
	"dash/internal/store/metricdata"
	pgtest "dash/internal/testutil/postgres"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func TestIntegrationUptime(t *testing.T) {
	db := pgtest.NewDB(t)
	loc, err := time.LoadLocation("Asia/Kathmandu")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(db, nil, loc, pgtest.ConfigCipher(t))
	if err := migrate.SyncOnlineJob(t.Context(), db, config.DefaultNodeOfflineThreshold, loc); err != nil {
		t.Fatal(err)
	}
	root := routes.NewBlueprint()
	root.Include("/api/metrics/uptime", Router(st, loc), routes.IncludeMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Test-Admin") == "true" {
				r = r.WithContext(routes.WithAuthenticated(r.Context()))
			}
			next.ServeHTTP(w, r)
		})
	}))
	router, err := routes.NewHandler(root.Routes(), routes.HandlerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string, admin bool, status int) []byte {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/metrics/uptime"+path, nil)
		if admin {
			r.Header.Set("Test-Admin", "true")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("GET %s admin=%v: %d %s, want %d", path, admin, w.Code, w.Body, status)
		}
		return w.Body.Bytes()
	}
	if err := db.Exec(`
		INSERT INTO servers(id,name,hostname,secret,is_guest_visible,is_deleted) VALUES
		(101,'visible','visible','visible',true,false),
		(102,'hidden','hidden','hidden',false,false),
		(103,'deleted','deleted','deleted',true,true),
		(104,'new','new','new',true,false);
		UPDATE system_settings SET uptime_warning_sla=99.5, uptime_error_sla=97.5;
	`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -1)
	if err := db.Exec(`
		INSERT INTO node_online(server_id,minute,online) VALUES
		(101,?,true),(101,?,false),(101,?,true),(102,?,true),(103,?,false)
	`, day.Add(time.Minute), day.Add(2*time.Minute), day.Add(24*time.Hour-time.Minute), day, day).Error; err != nil {
		t.Fatal(err)
	}
	// Materialize the closed day; the endpoint must also include newer raw data.
	if err := db.Exec("CALL refresh_continuous_aggregate('node_online_1h', ?::timestamptz, ?::timestamptz)", day, day.AddDate(0, 0, 1)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO node_online VALUES (101, ?, false)", now.UTC().Truncate(time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	var denied dailyView
	if err := json.Unmarshal(request("", false, 200), &denied); err != nil || denied.Enabled || len(denied.Nodes) != 0 {
		t.Fatalf("guest disabled: %+v, %v", denied, err)
	}
	dayPath := "/day?server_id=101&date=" + day.Format(time.DateOnly)
	request(dayPath, false, 403)
	for _, admin := range []bool{true, false} {
		if !admin {
			if err := db.Exec("UPDATE system_settings SET uptime_guest_visible=true").Error; err != nil {
				t.Fatal(err)
			}
		}
		var daily dailyView
		if err := json.Unmarshal(request("/", admin, 200), &daily); err != nil {
			t.Fatal(err)
		}
		want := 2
		if admin {
			want = 3
		}
		if !daily.Enabled || len(daily.Nodes) != want || daily.Timezone != loc.String() || daily.WarningSLA != 99.5 || daily.ErrorSLA != 97.5 {
			t.Fatalf("daily metadata admin=%v: %+v", admin, daily)
		}
		for _, node := range daily.Nodes {
			if len(node.Days) != 45 || node.Days[0].Date != day.AddDate(0, 0, -43).Format(time.DateOnly) {
				t.Fatalf("calendar dates: %+v", node)
			}
			if node.ServerID == "101" {
				past, today := node.Days[43], node.Days[44]
				if past.Percent == nil || math.Abs(*past.Percent-200.0/3) > 0.00001 || past.Samples != 3 || today.Percent == nil || *today.Percent != 0 || today.Samples != 1 {
					t.Fatalf("weighted daily rates or real-time data: past %+v, today %+v", past, today)
				}
			}
			if node.ServerID == "104" {
				for _, empty := range node.Days {
					if empty.Percent != nil || empty.Samples != 0 {
						t.Fatalf("unreported node has uptime: %+v", empty)
					}
				}
			}
		}
		var hourly metricdata.UptimeHours
		if err := json.Unmarshal(request(dayPath, admin, 200), &hourly); err != nil {
			t.Fatal(err)
		}
		if hourly.Hours[0] == nil || *hourly.Hours[0] != 50 || hourly.Hours[23] == nil || *hourly.Hours[23] != 100 || hourly.Samples[0] != 2 || hourly.Hours[12] != nil {
			t.Fatalf("hourly observations: %+v", hourly)
		}
	}
	for _, id := range []int{102, 103, 999} {
		request(fmt.Sprintf("/day?server_id=%d&date=%s", id, day.Format(time.DateOnly)), false, 404)
	}
	for _, date := range []string{"invalid", day.AddDate(0, 0, -44).Format(time.DateOnly), now.AddDate(0, 0, 1).Format(time.DateOnly)} {
		request("/day?server_id=101&date="+date, true, 400)
	}
	if err := db.Exec("UPDATE system_settings SET uptime_guest_visible=false").Error; err != nil {
		t.Fatal(err)
	}
	request(dayPath, false, 403)
	// Changing the statistics timezone rebuilds only derived buckets. Returning
	// to the original timezone must preserve the minute history and its rates.
	for _, zone := range []*time.Location{time.UTC, loc} {
		if err := migrate.SyncOnlineJob(t.Context(), db, config.DefaultNodeOfflineThreshold, zone); err != nil {
			t.Fatal(err)
		}
	}
	var after metricdata.UptimeHours
	// Read only materialized history to verify bootstrap actually ran the job.
	if err := db.Exec("ALTER MATERIALIZED VIEW node_online_1h SET (timescaledb.materialized_only=true)").Error; err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(request(dayPath, true, 200), &after); err != nil || after.Hours[0] == nil || *after.Hours[0] != 50 || after.Samples[0] != 2 {
		t.Fatalf("timezone rebuild changed observations: %+v, error %v", after, err)
	}
}
