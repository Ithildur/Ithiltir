package node

import (
	"context"
	"errors"
	"testing"

	"dash/internal/model"
	pgtest "dash/internal/testutil/postgres"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestGetServerBySecretUsesMemoryAuthIndex(t *testing.T) {
	st := newTestStore(nil, nil)
	ctx := context.Background()
	srv := model.Server{
		ID:           7,
		Name:         "node-7",
		Hostname:     "host-7",
		Secret:       "secret-7",
		DisplayOrder: 3,
		Tags:         datatypes.JSON([]byte(`["edge","db"]`)),
	}

	if err := st.SyncServerCache(ctx, srv); err != nil {
		t.Fatalf("SyncServerCache() error = %v", err)
	}

	got, err := st.GetServerBySecret(ctx, srv.Secret)
	if err != nil {
		t.Fatalf("GetServerBySecret() error = %v", err)
	}
	if got.ID != srv.ID {
		t.Fatalf("GetServerBySecret() id = %d, want %d", got.ID, srv.ID)
	}
	if got.Secret != srv.Secret {
		t.Fatalf("GetServerBySecret() secret = %q, want %q", got.Secret, srv.Secret)
	}
	if got.Name != srv.Name {
		t.Fatalf("GetServerBySecret() name = %q, want %q", got.Name, srv.Name)
	}
	if got.Hostname != srv.Hostname {
		t.Fatalf("GetServerBySecret() hostname = %q, want %q", got.Hostname, srv.Hostname)
	}
	if got.DisplayOrder != srv.DisplayOrder {
		t.Fatalf("GetServerBySecret() display_order = %d, want %d", got.DisplayOrder, srv.DisplayOrder)
	}
	if string(got.Tags) != string(srv.Tags) {
		t.Fatalf("GetServerBySecret() tags = %s, want %s", got.Tags, srv.Tags)
	}
}

func TestGetServerBySecretUnknownSecretReturnsNotFound(t *testing.T) {
	st := newTestStore(nil, nil)

	_, err := st.GetServerBySecret(context.Background(), "missing-secret")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetServerBySecret() error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestSyncServerCacheRotatesSecretInMemory(t *testing.T) {
	st := newTestStore(nil, nil)
	ctx := context.Background()

	oldSrv := model.Server{
		ID:       11,
		Name:     "node-11",
		Hostname: "host-11",
		Secret:   "old-secret",
	}
	if err := st.SyncServerCache(ctx, oldSrv); err != nil {
		t.Fatalf("SyncServerCache(old) error = %v", err)
	}

	newSrv := oldSrv
	newSrv.Secret = "new-secret"
	newSrv.Name = "node-11-new"
	st.syncServerCache(newSrv, oldSrv.Secret)

	if _, err := st.GetServerBySecret(ctx, oldSrv.Secret); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetServerBySecret(old) error = %v, want gorm.ErrRecordNotFound", err)
	}

	got, err := st.GetServerBySecret(ctx, newSrv.Secret)
	if err != nil {
		t.Fatalf("GetServerBySecret(new) error = %v", err)
	}
	if got.Secret != newSrv.Secret {
		t.Fatalf("GetServerBySecret(new) secret = %q, want %q", got.Secret, newSrv.Secret)
	}
	if got.Name != newSrv.Name {
		t.Fatalf("GetServerBySecret(new) name = %q, want %q", got.Name, newSrv.Name)
	}
}

func TestIntegrationCommitReconciliationIgnoresCancelledCaller(t *testing.T) {
	st := newTestStore(pgtest.NewDB(t), nil)
	commitErr := errors.New("commit result unknown")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, committed := range []bool{false, true} {
		name := "rolled-back"
		if committed {
			name = "committed"
		}
		t.Run(name, func(t *testing.T) {
			srv := model.Server{
				Name:     name,
				Hostname: name,
				Secret:   name + "-old-secret",
			}
			if err := st.db.Create(&srv).Error; err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if err := st.SyncServerCache(context.Background(), srv); err != nil {
				t.Fatalf("SyncServerCache() error = %v", err)
			}

			wantSecret := srv.Secret
			if committed {
				wantSecret = name + "-new-secret"
				if err := st.db.Model(&model.Server{}).Where("id = ?", srv.ID).Update("secret", wantSecret).Error; err != nil {
					t.Fatalf("Update(secret) error = %v", err)
				}
			}

			st.deleteServerMeta(srv.ID, srv.Secret)
			err := st.reconcileCommit(ctx, []int64{srv.ID}, commitErr)
			if !errors.Is(err, commitErr) {
				t.Fatalf("reconcileCommit() error = %v, want commit error", err)
			}
			if _, err := st.GetServerBySecret(context.Background(), srv.Secret); committed && !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("GetServerBySecret(old) error = %v, want gorm.ErrRecordNotFound", err)
			}
			if _, err := st.GetServerBySecret(context.Background(), wantSecret); err != nil {
				t.Fatalf("GetServerBySecret(current) error = %v", err)
			}
		})
	}
}
