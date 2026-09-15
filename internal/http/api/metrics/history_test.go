package metrics

import (
	"net/http/httptest"
	"testing"
	"time"

	"dash/internal/model"
	"dash/internal/store"
	"dash/internal/store/metricdata"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/gorm"
)

func TestIntegrationHistoryGuestAccess(t *testing.T) {
	db := pgtest.NewDB(t)
	st := store.New(db, nil, time.Local, pgtest.ConfigCipher(t))
	ctx := t.Context()
	h := newHandler(st.Metric, st.Front, st.Node, nil)
	r := httptest.NewRequest("GET", "/api/metrics/history?server_id=1", nil)
	hidden := createHistoryServer(t, db, "hidden", false)
	visible := createHistoryServer(t, db, "visible", true)

	allowed, err := h.canReadHistory(ctx, r, visible.ID)
	if err != nil {
		t.Fatalf("canReadHistory() error = %v", err)
	}
	if allowed {
		t.Fatal("default policy allowed guest history access")
	}

	if err := st.Metric.SetHistoryGuestAccessMode(ctx, metricdata.HistoryGuestAccessByNode); err != nil {
		t.Fatalf("SetHistoryGuestAccessMode() error = %v", err)
	}
	allowed, err = h.canReadHistory(ctx, r, hidden.ID)
	if err != nil {
		t.Fatalf("canReadHistory(hidden) error = %v", err)
	}
	if allowed {
		t.Fatal("canReadHistory(hidden) = true, want false")
	}

	allowed, err = h.canReadHistory(ctx, r, visible.ID)
	if err != nil {
		t.Fatalf("canReadHistory(visible) error = %v", err)
	}
	if !allowed {
		t.Fatal("canReadHistory(visible) = false, want true")
	}
}

func createHistoryServer(t *testing.T, db *gorm.DB, name string, guestVisible bool) model.Server {
	t.Helper()

	srv := model.Server{
		Name:           name,
		Hostname:       name + "-host",
		Secret:         name + "-secret",
		IsGuestVisible: guestVisible,
	}
	if err := db.WithContext(t.Context()).Create(&srv).Error; err != nil {
		t.Fatalf("Create(%s) error = %v", name, err)
	}
	return srv
}
