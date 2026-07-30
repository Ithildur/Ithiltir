package node

import (
	"context"
	"errors"
	"testing"

	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"

	"gorm.io/gorm"
)

func newIntegrationStore(t *testing.T) *Store {
	t.Helper()

	db := pgtest.NewDB(t)
	return newTestStore(db, nil)
}

func TestIntegrationUpdateStaticRejectsStaleSecret(t *testing.T) {
	st := newIntegrationStore(t)
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
	st.syncServerCache(newSrv, srv.Secret)

	static := ServerStaticPatch{Hostname: strPtr("host-21")}

	err := st.UpdateStatic(ctx, srv.Secret, srv.ID, static)
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

func TestIntegrationUpdateStaticClearsSwapWithExplicitZero(t *testing.T) {
	st := newIntegrationStore(t)
	ctx := context.Background()

	previous := int64(2 << 30)
	srv := model.Server{
		Name:         "node-1",
		Hostname:     "node-1",
		Secret:       "node-secret",
		SwapTotal:    &previous,
		DisplayOrder: 1,
	}
	if err := st.db.WithContext(ctx).Create(&srv).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	zero := int64(0)
	if err := st.UpdateStatic(ctx, srv.Secret, srv.ID, ServerStaticPatch{SwapTotal: &zero}); err != nil {
		t.Fatalf("UpdateStatic() error = %v", err)
	}

	var stored model.Server
	if err := st.db.WithContext(ctx).Select("swap_total").First(&stored, srv.ID).Error; err != nil {
		t.Fatalf("load updated server: %v", err)
	}
	if stored.SwapTotal == nil || *stored.SwapTotal != 0 {
		t.Fatalf("stored swap total = %v, want explicit zero", stored.SwapTotal)
	}
}

func TestIntegrationUpdateStaticKeepsDiskObservationAtomic(t *testing.T) {
	st := newIntegrationStore(t)
	ctx := context.Background()

	oldTotal := int64(4 << 30)
	oldPath := "/old"
	oldFSType := "ext4"
	srv := model.Server{
		Name:         "node-1",
		Hostname:     "node-1",
		Secret:       "disk-observation-secret",
		DiskTotal:    &oldTotal,
		RootPath:     &oldPath,
		RootFSType:   &oldFSType,
		DisplayOrder: 1,
	}
	if err := st.db.WithContext(ctx).Create(&srv).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	hostname := "node-1-updated"
	if err := st.UpdateStatic(ctx, srv.Secret, srv.ID, ServerStaticPatch{Hostname: &hostname}); err != nil {
		t.Fatalf("UpdateStatic(no disk observation) error = %v", err)
	}
	preserved := loadServerDiskObservation(t, st.db.WithContext(ctx), srv.ID)
	if preserved.DiskTotal == nil || *preserved.DiskTotal != oldTotal ||
		preserved.RootPath == nil || *preserved.RootPath != oldPath ||
		preserved.RootFSType == nil || *preserved.RootFSType != oldFSType {
		t.Fatalf("disk observation after unavailable report = %+v, want preserved tuple", preserved)
	}

	if err := st.UpdateStatic(ctx, srv.Secret, srv.ID, ServerStaticPatch{
		Disk: &DiskObservation{Path: "/new"},
	}); err != nil {
		t.Fatalf("UpdateStatic(partial disk observation) error = %v", err)
	}
	replaced := loadServerDiskObservation(t, st.db.WithContext(ctx), srv.ID)
	if replaced.RootPath == nil || *replaced.RootPath != "/new" ||
		replaced.RootFSType != nil || replaced.DiskTotal != nil {
		t.Fatalf("disk observation = %+v, want new path with unknown dependent fields", replaced)
	}
}

func loadServerDiskObservation(t *testing.T, db *gorm.DB, id int64) model.Server {
	t.Helper()
	var server model.Server
	if err := db.Select("disk_total", "root_path", "root_fs_type").First(&server, id).Error; err != nil {
		t.Fatalf("load disk observation: %v", err)
	}
	return server
}
