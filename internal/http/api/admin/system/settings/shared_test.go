package settings

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dash/internal/store"
	pgtest "dash/internal/testutil/postgres"
	"github.com/Ithildur/EiluneKit/http/routes"
	"gorm.io/gorm"
)

func TestIntegrationSettingsReadContext(t *testing.T) {
	db := pgtest.NewDB(t)
	st := store.New(db, nil, time.UTC, pgtest.ConfigCipher(t))
	for name, cancelParent := range map[string]bool{"shared deadline": false, "parent cancellation": true} {
		t.Run(name, func(t *testing.T) {
			parent, cancel := context.WithCancel(t.Context())
			defer cancel()
			var queries []context.Context
			const callback = "test:settings_read_context"
			if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
				queries = append(queries, tx.Statement.Context)
				if cancelParent && len(queries) == 2 {
					cancel()
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Callback().Query().Remove(callback); err != nil {
					t.Error(err)
				}
			})

			_, err := loadSettings(parent, st.Metric, st.System)
			if cancelParent {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("loadSettings() error = %v, want parent cancellation", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(queries) < 2 {
				t.Fatalf("observed %d queries, want a multi-query settings read", len(queries))
			}
			deadline, ok := queries[0].Deadline()
			if !ok {
				t.Fatal("settings read has no deadline")
			}
			for _, query := range queries {
				got, ok := query.Deadline()
				if !ok || !got.Equal(deadline) {
					t.Errorf("query deadline = %v, want shared deadline %v", got, deadline)
				}
				if !errors.Is(query.Err(), context.Canceled) {
					t.Errorf("query context after loadSettings() = %v, want canceled", query.Err())
				}
			}
		})
	}
}

func TestIntegrationUptimeSettings(t *testing.T) {
	st := store.New(pgtest.NewDB(t), nil, time.UTC, pgtest.ConfigCipher(t))
	root := routes.NewBlueprint()
	root.Include("/api/admin/system/settings", Router(st))
	router, err := routes.NewHandler(root.Routes(), routes.HandlerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/admin/system/settings", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s settings = %d %s, want %d", method, w.Code, w.Body, status)
		}
		return w
	}
	read := func() settingsView {
		t.Helper()
		var view settingsView
		if err := json.Unmarshal(request("GET", "", 200).Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		return view
	}
	initial := read()
	if !initial.UptimeGuestVisible || initial.UptimeWarningSLA != 99 || initial.UptimeErrorSLA != 95 {
		t.Fatalf("uptime defaults = %+v", initial)
	}
	request("PATCH", `{"uptime_guest_visible":true,"uptime_warning_sla":99.9,"uptime_error_sla":98}`, 204)
	saved := read()
	if !saved.UptimeGuestVisible || saved.UptimeWarningSLA != 99.9 || saved.UptimeErrorSLA != 98 {
		t.Fatalf("saved uptime settings = %+v", saved)
	}
	for _, body := range []string{
		`{"uptime_error_sla":100,"page_title":"must roll back","history_guest_access_mode":"by_node","dash_update_channel":"prerelease","dash_update_mode":"notify"}`,
		`{"uptime_warning_sla":101}`,
		`{"uptime_error_sla":-1}`,
		`{"uptime_warning_sla":98}`,
	} {
		w := request("PATCH", body, 400)
		var failure struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &failure); err != nil || failure.Code != "invalid_fields" {
			t.Fatalf("invalid SLA response = %s, error %v", w.Body, err)
		}
		if got := read(); got != saved {
			t.Fatalf("invalid patch changed settings: got %+v, want %+v", got, saved)
		}
	}
	request("PATCH", `{"uptime_guest_visible":false,"uptime_error_sla":0}`, 204)
	request("PUT", `{"history_guest_access_mode":"disabled","dash_update_channel":"release","dash_update_mode":"manual","logo_url":"/brandlogo.svg","page_title":"Legacy client","topbar_text":"Ops"}`, 204)
	got := read()
	if got.UptimeGuestVisible || got.UptimeWarningSLA != 99.9 || got.UptimeErrorSLA != 0 || got.PageTitle != "Legacy client" {
		t.Fatalf("legacy PUT or explicit false/zero changed uptime settings: %+v", got)
	}
}

func TestSiteBrandPatchValidatesOnlySubmittedFields(t *testing.T) {
	title := " New title "
	patch, err := (settingsInput{PageTitle: &title}).siteBrandPatch()
	if err != nil {
		t.Fatalf("siteBrandPatch() error = %v", err)
	}
	if patch.LogoURL != nil || patch.TopbarText != nil {
		t.Fatalf("siteBrandPatch() changed unspecified fields: %+v", patch)
	}
	if patch.PageTitle == nil || *patch.PageTitle != "New title" {
		t.Fatalf("siteBrandPatch() page title = %v, want New title", patch.PageTitle)
	}
}

func TestValidLogoURLAcceptsDocumentedDataImages(t *testing.T) {
	mediaTypes := []string{
		"image/svg+xml",
		"image/png",
		"image/jpeg",
		"image/gif",
		"image/webp",
		"image/ico",
		"image/x-icon",
		"image/vnd.microsoft.icon",
	}
	for _, mediaType := range mediaTypes {
		t.Run(mediaType, func(t *testing.T) {
			if value := "data:" + mediaType + ";base64,AQ=="; !validLogoURL(value) {
				t.Fatalf("validLogoURL(%q) = false, want true", value)
			}
		})
	}
}
