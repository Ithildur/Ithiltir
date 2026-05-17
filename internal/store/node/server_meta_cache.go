package node

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dash/internal/model"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var serverMetaSelectColumns = []string{
	"id",
	"name",
	"hostname",
	"display_order",
	"ip",
	"os",
	"platform",
	"platform_version",
	"kernel_version",
	"arch",
	"cpu_model",
	"cpu_vendor",
	"cpu_cores_physical",
	"cpu_cores_logical",
	"cpu_sockets",
	"cpu_mhz",
	"mem_total",
	"swap_total",
	"disk_total",
	"root_path",
	"root_fs_type",
	"raid_supported",
	"raid_available",
	"interval_sec",
	"agent_version",
	"tags",
	"secret",
	"is_deleted",
}

// ServerMeta is the static server data used to assemble frontend and ingest views.
type ServerMeta struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	Hostname        string         `json:"hostname"`
	DisplayOrder    int            `json:"display_order"`
	IP              *string        `json:"ip,omitempty"`
	OS              *string        `json:"os,omitempty"`
	Platform        *string        `json:"platform,omitempty"`
	PlatformVersion *string        `json:"platform_version,omitempty"`
	KernelVersion   *string        `json:"kernel_version,omitempty"`
	Arch            *string        `json:"arch,omitempty"`
	CPUModel        *string        `json:"cpu_model,omitempty"`
	CPUVendor       *string        `json:"cpu_vendor,omitempty"`
	CPUCoresPhys    *int16         `json:"cpu_cores_physical,omitempty"`
	CPUCoresLog     *int16         `json:"cpu_cores_logical,omitempty"`
	CPUSockets      *int16         `json:"cpu_sockets,omitempty"`
	CPUMhz          *float64       `json:"cpu_mhz,omitempty"`
	MemTotal        *int64         `json:"mem_total,omitempty"`
	SwapTotal       *int64         `json:"swap_total,omitempty"`
	DiskTotal       *int64         `json:"disk_total,omitempty"`
	RootPath        *string        `json:"root_path,omitempty"`
	RootFSType      *string        `json:"root_fs_type,omitempty"`
	RaidSupported   *bool          `json:"raid_supported,omitempty"`
	RaidAvailable   *bool          `json:"raid_available,omitempty"`
	IntervalSec     *int32         `json:"interval_sec,omitempty"`
	AgentVersion    *string        `json:"agent_version,omitempty"`
	Tags            datatypes.JSON `json:"tags,omitempty"`
}

func metaFromServer(srv model.Server) ServerMeta {
	return ServerMeta{
		ID:              srv.ID,
		Name:            srv.Name,
		Hostname:        srv.Hostname,
		DisplayOrder:    srv.DisplayOrder,
		IP:              srv.IP,
		OS:              srv.OS,
		Platform:        srv.Platform,
		PlatformVersion: srv.PlatformVersion,
		KernelVersion:   srv.KernelVersion,
		Arch:            srv.Arch,
		CPUModel:        srv.CPUModel,
		CPUVendor:       srv.CPUVendor,
		CPUCoresPhys:    srv.CPUCoresPhys,
		CPUCoresLog:     srv.CPUCoresLog,
		CPUSockets:      srv.CPUSockets,
		CPUMhz:          srv.CPUMhz,
		MemTotal:        srv.MemTotal,
		SwapTotal:       srv.SwapTotal,
		DiskTotal:       srv.DiskTotal,
		RootPath:        srv.RootPath,
		RootFSType:      srv.RootFSType,
		RaidSupported:   srv.RaidSupported,
		RaidAvailable:   srv.RaidAvailable,
		IntervalSec:     srv.IntervalSec,
		AgentVersion:    srv.AgentVersion,
		Tags:            srv.Tags,
	}
}

func (m ServerMeta) toServer(secret string) model.Server {
	return model.Server{
		ID:              m.ID,
		Name:            m.Name,
		Hostname:        m.Hostname,
		Secret:          secret,
		DisplayOrder:    m.DisplayOrder,
		IP:              m.IP,
		OS:              m.OS,
		Platform:        m.Platform,
		PlatformVersion: m.PlatformVersion,
		KernelVersion:   m.KernelVersion,
		Arch:            m.Arch,
		CPUModel:        m.CPUModel,
		CPUVendor:       m.CPUVendor,
		CPUCoresPhys:    m.CPUCoresPhys,
		CPUCoresLog:     m.CPUCoresLog,
		CPUSockets:      m.CPUSockets,
		CPUMhz:          m.CPUMhz,
		MemTotal:        m.MemTotal,
		SwapTotal:       m.SwapTotal,
		DiskTotal:       m.DiskTotal,
		RootPath:        m.RootPath,
		RootFSType:      m.RootFSType,
		RaidSupported:   m.RaidSupported,
		RaidAvailable:   m.RaidAvailable,
		IntervalSec:     m.IntervalSec,
		AgentVersion:    m.AgentVersion,
		Tags:            m.Tags,
	}
}

type memNodeAuthBackend struct {
	mem *memState
}

func newNodeAuthBackend(mem *memState) nodeAuthBackend {
	return &memNodeAuthBackend{mem: mem}
}

func (s *Store) syncServerCache(_ context.Context, srv model.Server, oldSecret string) error {
	if s == nil || s.auth == nil {
		return fmt.Errorf("store: node auth backend is nil")
	}
	return s.auth.syncServerCache(srv, oldSecret)
}

func (s *Store) SyncServerCache(ctx context.Context, srv model.Server) error {
	return s.syncServerCache(ctx, srv, srv.Secret)
}

func (s *Store) deleteServerMeta(_ context.Context, id int64, secret string) error {
	if s == nil || s.auth == nil {
		return nil
	}
	return s.auth.deleteServerMeta(id, secret)
}

func (s *Store) getSecretByID(_ context.Context, id int64) (string, error) {
	if s == nil || s.auth == nil {
		return "", nil
	}
	return s.auth.getSecretByID(id)
}

func (s *Store) deleteMetaByID(ctx context.Context, id int64) error {
	secret, err := s.getSecretByID(ctx, id)
	if err != nil {
		return err
	}
	return s.deleteServerMeta(ctx, id, secret)
}

func (s *Store) RefreshMetaByID(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}

	var srv model.Server
	err := s.db.WithContext(ctx).
		Table("servers").
		Select(serverMetaSelectColumns).
		Where("id = ?", id).
		Take(&srv).
		Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return s.deleteMetaByID(ctx, id)
	}
	if srv.IsDeleted {
		return s.deleteServerMeta(ctx, srv.ID, srv.Secret)
	}
	return s.SyncServerCache(ctx, srv)
}

func (s *Store) RefreshMetaByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	var rows []model.Server
	if err := s.db.WithContext(ctx).
		Table("servers").
		Select(serverMetaSelectColumns).
		Where("id IN ?", ids).
		Find(&rows).
		Error; err != nil {
		return err
	}

	seen := make(map[int64]struct{}, len(rows))
	for _, srv := range rows {
		seen[srv.ID] = struct{}{}
		if srv.IsDeleted {
			if err := s.deleteServerMeta(ctx, srv.ID, srv.Secret); err != nil {
				return err
			}
			continue
		}
		if err := s.SyncServerCache(ctx, srv); err != nil {
			return err
		}
	}

	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		if err := s.deleteMetaByID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RebuildServerCache(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	if s.mem == nil {
		return fmt.Errorf("store: memory auth is nil")
	}

	var servers []model.Server
	if err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select(serverMetaSelectColumns).
		Where("is_deleted = ?", false).
		Find(&servers).
		Error; err != nil {
		return err
	}

	bySecret := make(map[string]authEntry, len(servers))
	byID := make(map[int64]authEntry, len(servers))
	for _, srv := range servers {
		secret := strings.TrimSpace(srv.Secret)
		if secret == "" || srv.ID <= 0 {
			continue
		}
		entry := authEntry{
			secret: secret,
			meta:   metaFromServer(srv),
		}
		bySecret[secret] = entry
		byID[srv.ID] = entry
	}

	s.mem.authMu.Lock()
	s.mem.authBySecret = bySecret
	s.mem.authByID = byID
	s.mem.authMu.Unlock()
	return nil
}

func (s *Store) GetServerBySecret(_ context.Context, secret string) (model.Server, error) {
	if s == nil || s.auth == nil {
		return model.Server{}, fmt.Errorf("store: node auth backend is nil")
	}
	return s.auth.getServerBySecret(secret)
}

func (b *memNodeAuthBackend) syncServerCache(srv model.Server, oldSecret string) error {
	if b == nil || b.mem == nil {
		return fmt.Errorf("store: memory auth is nil")
	}
	newSecret := strings.TrimSpace(srv.Secret)
	oldSecret = strings.TrimSpace(oldSecret)
	if newSecret == "" {
		return fmt.Errorf("store: secret is empty")
	}
	if srv.ID <= 0 {
		return fmt.Errorf("store: invalid server id %d", srv.ID)
	}

	entry := authEntry{
		secret: newSecret,
		meta:   metaFromServer(srv),
	}

	b.mem.authMu.Lock()
	defer b.mem.authMu.Unlock()

	if oldSecret != "" && oldSecret != newSecret {
		delete(b.mem.authBySecret, oldSecret)
	}
	if old, ok := b.mem.authByID[srv.ID]; ok && old.secret != "" && old.secret != newSecret {
		delete(b.mem.authBySecret, old.secret)
	}
	b.mem.authByID[srv.ID] = entry
	b.mem.authBySecret[newSecret] = entry
	return nil
}

func (b *memNodeAuthBackend) deleteServerMeta(id int64, secret string) error {
	if b == nil || b.mem == nil || id <= 0 {
		return nil
	}
	secret = strings.TrimSpace(secret)

	b.mem.authMu.Lock()
	defer b.mem.authMu.Unlock()

	if secret == "" {
		if old, ok := b.mem.authByID[id]; ok {
			secret = old.secret
		}
	}
	delete(b.mem.authByID, id)
	if secret != "" {
		delete(b.mem.authBySecret, secret)
	}
	return nil
}

func (b *memNodeAuthBackend) getSecretByID(id int64) (string, error) {
	if b == nil || b.mem == nil || id <= 0 {
		return "", nil
	}
	b.mem.authMu.RLock()
	defer b.mem.authMu.RUnlock()
	entry, ok := b.mem.authByID[id]
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(entry.secret), nil
}

func (b *memNodeAuthBackend) getServerBySecret(secret string) (model.Server, error) {
	if b == nil || b.mem == nil {
		return model.Server{}, fmt.Errorf("store: memory auth is nil")
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return model.Server{}, gorm.ErrRecordNotFound
	}

	b.mem.authMu.RLock()
	entry, ok := b.mem.authBySecret[secret]
	b.mem.authMu.RUnlock()
	if !ok {
		return model.Server{}, gorm.ErrRecordNotFound
	}
	if entry.meta.ID <= 0 {
		return model.Server{}, fmt.Errorf("invalid memory auth entry")
	}
	return entry.meta.toServer(secret), nil
}
