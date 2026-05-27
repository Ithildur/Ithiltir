package metrics

import (
	"context"
	"net/http/httptest"
	"testing"

	"dash/internal/model"
	"dash/internal/store"
	"dash/internal/store/metricdata"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/gorm"
)

func newHistoryTestStore(t *testing.T) (*store.Stores, *gorm.DB) {
	t.Helper()

	db := pgtest.NewDB(t)
	return store.New(db, nil), db
}

func TestIntegrationHistoryGuestAccessDisabledByDefault(t *testing.T) {
	st, _ := newHistoryTestStore(t)
	h := newHandler(st.Metric, st.Front, nil)
	r := httptest.NewRequest("GET", "/api/metrics/history?server_id=1", nil)

	allowed, err := h.canReadHistory(context.Background(), r, 1)
	if err != nil {
		t.Fatalf("canReadHistory() error = %v", err)
	}
	if allowed {
		t.Fatal("canReadHistory() = true, want false")
	}
}

func TestIntegrationHistoryGuestAccessByNodeUsesGuestVisible(t *testing.T) {
	st, db := newHistoryTestStore(t)
	ctx := context.Background()
	h := newHandler(st.Metric, st.Front, nil)
	r := httptest.NewRequest("GET", "/api/metrics/history?server_id=1", nil)

	if err := st.Metric.SetHistoryGuestAccessMode(ctx, metricdata.HistoryGuestAccessByNode); err != nil {
		t.Fatalf("SetHistoryGuestAccessMode() error = %v", err)
	}
	hidden := createHistoryServer(t, db, "hidden", false)
	visible := createHistoryServer(t, db, "visible", true)

	allowed, err := h.canReadHistory(ctx, r, hidden.ID)
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
	if err := db.WithContext(context.Background()).Create(&srv).Error; err != nil {
		t.Fatalf("Create(%s) error = %v", name, err)
	}
	return srv
}
