package node

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"dash/internal/model"
	"dash/internal/store/frontcache"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLiteStore(t *testing.T) *Store {
	t.Helper()

	db := newSQLiteDB(t)
	return New(db, nil, frontcache.New(db, nil))
}

func newSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + url.QueryEscape(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(
		&model.Server{},
		&model.Group{},
		&model.ServerGroup{},
		&model.ServerMetric{},
		&model.ServerCurrentMetric{},
		&model.ServerCurrentDiskMetric{},
		&model.ServerCurrentDiskUsageMetric{},
		&model.ServerCurrentNICMetric{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func TestUpdateStaticRejectsStaleSecret(t *testing.T) {
	st := newSQLiteStore(t)
	ctx := context.Background()

	srv := model.Server{
		Name:         "Untitled",
		Hostname:     "host-21",
		Secret:       "old-secret",
		DisplayOrder: 1,
	}
	if err := st.db.WithContext(ctx).Create(&srv).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := st.SyncServerCache(ctx, srv); err != nil {
		t.Fatalf("SyncServerCache(old) error = %v", err)
	}

	newSrv := srv
	newSrv.Secret = "new-secret"
	if err := st.db.WithContext(ctx).
		Model(&model.Server{}).
		Where("id = ?", srv.ID).
		Update("secret", newSrv.Secret).
		Error; err != nil {
		t.Fatalf("Update(secret) error = %v", err)
	}
	if err := st.syncServerCache(ctx, newSrv, srv.Secret); err != nil {
		t.Fatalf("syncServerCache(new) error = %v", err)
	}

	patch := ServerStaticPatch{
		Hostname:        strPtr("host-21"),
		OS:              strPtr("linux"),
		Platform:        strPtr("ubuntu"),
		PlatformVersion: strPtr("24.04"),
		KernelVersion:   strPtr("6.8.0"),
		Arch:            strPtr("x86_64"),
		AgentVersion:    strPtr("1.0.0"),
		CPUCoresPhys:    int16Ptr(4),
		CPUCoresLog:     int16Ptr(8),
		CPUSockets:      int16Ptr(1),
		MemTotal:        int64Ptr(16 << 30),
		RaidSupported:   boolPtr(false),
		RaidAvailable:   boolPtr(false),
		IntervalSec:     int32Ptr(10),
	}

	err := st.UpdateStatic(ctx, srv.Secret, srv.ID, patch)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateStatic() error = %v, want gorm.ErrRecordNotFound", err)
	}

	if _, err := st.GetServerBySecret(ctx, srv.Secret); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetServerBySecret(old) error = %v, want gorm.ErrRecordNotFound", err)
	}
	got, err := st.GetServerBySecret(ctx, newSrv.Secret)
	if err != nil {
		t.Fatalf("GetServerBySecret(new) error = %v", err)
	}
	if got.Secret != newSrv.Secret {
		t.Fatalf("GetServerBySecret(new) secret = %q, want %q", got.Secret, newSrv.Secret)
	}
}

func int16Ptr(v int16) *int16 { return &v }
func int32Ptr(v int32) *int32 { return &v }
func int64Ptr(v int64) *int64 { return &v }
func boolPtr(v bool) *bool    { return &v }
