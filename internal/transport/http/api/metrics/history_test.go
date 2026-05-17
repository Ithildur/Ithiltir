package metrics

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"dash/internal/model"
	"dash/internal/store"
	"dash/internal/store/metricdata"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newHistoryTestStore(t *testing.T) (*store.Stores, *gorm.DB) {
	t.Helper()

	dsn := "file:" + url.QueryEscape(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&model.Server{}, &model.MetricSetting{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return store.New(db, nil), db
}

func TestHistoryGuestAccessDisabledByDefault(t *testing.T) {
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

func TestHistoryGuestAccessByNodeUsesGuestVisible(t *testing.T) {
	st, db := newHistoryTestStore(t)
	ctx := context.Background()
	h := newHandler(st.Metric, st.Front, nil)
	r := httptest.NewRequest("GET", "/api/metrics/history?server_id=1", nil)

	if err := st.Metric.SetHistoryGuestAccessMode(ctx, metricdata.HistoryGuestAccessByNode); err != nil {
		t.Fatalf("SetHistoryGuestAccessMode() error = %v", err)
	}
	if err := db.WithContext(ctx).Create(&model.Server{
		Name:           "hidden",
		Hostname:       "hidden-host",
		Secret:         "secret-hidden",
		DisplayOrder:   1,
		IsGuestVisible: false,
	}).Error; err != nil {
		t.Fatalf("Create(hidden) error = %v", err)
	}
	if err := db.WithContext(ctx).Create(&model.Server{
		Name:           "visible",
		Hostname:       "visible-host",
		Secret:         "secret-visible",
		DisplayOrder:   2,
		IsGuestVisible: true,
	}).Error; err != nil {
		t.Fatalf("Create(visible) error = %v", err)
	}

	allowed, err := h.canReadHistory(ctx, r, 1)
	if err != nil {
		t.Fatalf("canReadHistory(hidden) error = %v", err)
	}
	if allowed {
		t.Fatal("canReadHistory(hidden) = true, want false")
	}

	allowed, err = h.canReadHistory(ctx, r, 2)
	if err != nil {
		t.Fatalf("canReadHistory(visible) error = %v", err)
	}
	if !allowed {
		t.Fatal("canReadHistory(visible) = false, want true")
	}
}
