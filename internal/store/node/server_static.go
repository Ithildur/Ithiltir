package node

import (
	"context"
	"fmt"

	"dash/internal/model"

	"gorm.io/gorm"
)

// ServerStaticPatch describes the static server fields updated by /api/node/static.
// Hostname/OS/Platform/... are required by the caller and treated as always set.
type ServerStaticPatch struct {
	Name     *string
	Hostname *string
	IP       *string

	OS              *string
	Platform        *string
	PlatformVersion *string
	KernelVersion   *string
	Arch            *string
	AgentVersion    *string

	CPUModel     *string
	CPUVendor    *string
	CPUCoresPhys *int16
	CPUCoresLog  *int16
	CPUSockets   *int16
	CPUMhz       *float64

	MemTotal   *int64
	SwapTotal  *int64
	DiskTotal  *int64
	RootPath   *string
	RootFSType *string

	RaidSupported *bool
	RaidAvailable *bool
	IntervalSec   *int32
}

func (p ServerStaticPatch) applyToServer(server *model.Server) {
	if server == nil {
		return
	}

	server.Hostname = *p.Hostname
	if p.IP != nil {
		server.IP = p.IP
	}
	if p.Name != nil {
		server.Name = *p.Name
	}

	server.OS = p.OS
	server.Platform = p.Platform
	server.PlatformVersion = p.PlatformVersion
	server.KernelVersion = p.KernelVersion
	server.Arch = p.Arch
	server.AgentVersion = p.AgentVersion

	server.CPUCoresPhys = p.CPUCoresPhys
	server.CPUCoresLog = p.CPUCoresLog
	server.CPUSockets = p.CPUSockets
	if p.CPUModel != nil {
		server.CPUModel = p.CPUModel
	}
	if p.CPUVendor != nil {
		server.CPUVendor = p.CPUVendor
	}
	if p.CPUMhz != nil {
		server.CPUMhz = p.CPUMhz
	}

	server.MemTotal = p.MemTotal
	if p.SwapTotal != nil {
		server.SwapTotal = p.SwapTotal
	}
	if p.DiskTotal != nil {
		server.DiskTotal = p.DiskTotal
	}
	if p.RootPath != nil {
		server.RootPath = p.RootPath
	}
	if p.RootFSType != nil {
		server.RootFSType = p.RootFSType
	}

	server.RaidSupported = p.RaidSupported
	server.RaidAvailable = p.RaidAvailable
	server.IntervalSec = p.IntervalSec
}

func (p ServerStaticPatch) updates() map[string]any {
	updates := map[string]any{
		"hostname":           *p.Hostname,
		"os":                 *p.OS,
		"platform":           *p.Platform,
		"platform_version":   *p.PlatformVersion,
		"kernel_version":     *p.KernelVersion,
		"arch":               *p.Arch,
		"agent_version":      *p.AgentVersion,
		"cpu_cores_physical": *p.CPUCoresPhys,
		"cpu_cores_logical":  *p.CPUCoresLog,
		"cpu_sockets":        *p.CPUSockets,
		"mem_total":          *p.MemTotal,
		"raid_supported":     *p.RaidSupported,
		"raid_available":     *p.RaidAvailable,
		"interval_sec":       *p.IntervalSec,
	}

	if p.Name != nil {
		updates["name"] = *p.Name
	}
	if p.IP != nil {
		updates["ip"] = *p.IP
	}
	if p.CPUModel != nil {
		updates["cpu_model"] = *p.CPUModel
	}
	if p.CPUVendor != nil {
		updates["cpu_vendor"] = *p.CPUVendor
	}
	if p.CPUMhz != nil {
		updates["cpu_mhz"] = *p.CPUMhz
	}
	if p.SwapTotal != nil {
		updates["swap_total"] = *p.SwapTotal
	}
	if p.DiskTotal != nil {
		updates["disk_total"] = *p.DiskTotal
	}
	if p.RootPath != nil {
		updates["root_path"] = *p.RootPath
	}
	if p.RootFSType != nil {
		updates["root_fs_type"] = *p.RootFSType
	}

	return updates
}

// UpdateStatic persists /api/node/static fields and refreshes the cached server state.
func (s *Store) UpdateStatic(ctx context.Context, secret string, serverID int64, patch ServerStaticPatch) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	if serverID <= 0 {
		return fmt.Errorf("store: invalid server id %d", serverID)
	}

	updates := patch.updates()
	if len(updates) == 0 {
		return nil
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()

	var old model.Server
	if err := tx.Model(&model.Server{}).
		Select(serverMetaSelectColumns).
		Where("id = ? AND secret = ? AND is_deleted = ?", serverID, secret, false).
		Take(&old).Error; err != nil {
		return err
	}

	res := tx.Model(&model.Server{}).
		Where("id = ? AND secret = ? AND is_deleted = ?", serverID, secret, false).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	var fresh model.Server
	if err := tx.Model(&model.Server{}).
		Select(serverMetaSelectColumns).
		Where("id = ? AND secret = ? AND is_deleted = ?", serverID, secret, false).
		Take(&fresh).Error; err != nil {
		return err
	}

	if err := tx.Commit().Error; err != nil {
		_ = s.RefreshMetaByID(ctx, serverID)
		return err
	}
	committed = true
	if err := s.syncServerCache(ctx, fresh, old.Secret); err != nil {
		_ = s.RefreshMetaByID(ctx, serverID)
		return fmt.Errorf("%w: %w", ErrServerMetaCacheUpdate, err)
	}
	return nil
}
