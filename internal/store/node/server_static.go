package node

import (
	"context"
	"fmt"
	"strings"

	"dash/internal/config"
	"dash/internal/model"

	"gorm.io/gorm"
)

// ServerStaticPatch contains only static fields observed in one node report.
// A nil field means that this report must preserve the stored value.
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

	MemTotal  *int64
	SwapTotal *int64
	Disk      *DiskObservation

	RaidSupported *bool
	RaidAvailable *bool
	IntervalSec   *int32
}

// DiskObservation is one representative logical-disk observation. Path is
// the identity boundary; an empty filesystem type or non-positive capacity is
// stored as unknown instead of being inherited from another observation.
type DiskObservation struct {
	Path   string
	FSType string
	Total  int64
}

func (p ServerStaticPatch) updates() map[string]any {
	updates := make(map[string]any, 23)
	if p.Name != nil {
		updates["name"] = *p.Name
	}
	if p.Hostname != nil {
		updates["hostname"] = *p.Hostname
	}
	if p.IP != nil {
		updates["ip"] = *p.IP
	}
	if p.OS != nil {
		updates["os"] = *p.OS
	}
	if p.Platform != nil {
		updates["platform"] = *p.Platform
	}
	if p.PlatformVersion != nil {
		updates["platform_version"] = *p.PlatformVersion
	}
	if p.KernelVersion != nil {
		updates["kernel_version"] = *p.KernelVersion
	}
	if p.Arch != nil {
		updates["arch"] = *p.Arch
	}
	if p.AgentVersion != nil {
		updates["agent_version"] = *p.AgentVersion
	}
	if p.CPUModel != nil {
		updates["cpu_model"] = *p.CPUModel
	}
	if p.CPUVendor != nil {
		updates["cpu_vendor"] = *p.CPUVendor
	}
	if p.CPUCoresPhys != nil {
		updates["cpu_cores_physical"] = *p.CPUCoresPhys
	}
	if p.CPUCoresLog != nil {
		updates["cpu_cores_logical"] = *p.CPUCoresLog
	}
	if p.CPUSockets != nil {
		updates["cpu_sockets"] = *p.CPUSockets
	}
	if p.CPUMhz != nil {
		updates["cpu_mhz"] = *p.CPUMhz
	}
	if p.MemTotal != nil {
		updates["mem_total"] = *p.MemTotal
	}
	if p.SwapTotal != nil {
		updates["swap_total"] = *p.SwapTotal
	}
	if p.Disk != nil {
		path := strings.TrimSpace(p.Disk.Path)
		if path != "" {
			updates["root_path"] = path
			updates["root_fs_type"] = nil
			if fsType := strings.TrimSpace(p.Disk.FSType); fsType != "" {
				updates["root_fs_type"] = fsType
			}
			updates["disk_total"] = nil
			if p.Disk.Total > 0 {
				updates["disk_total"] = p.Disk.Total
			}
		}
	}
	if p.RaidSupported != nil {
		updates["raid_supported"] = *p.RaidSupported
	}
	if p.RaidAvailable != nil {
		updates["raid_available"] = *p.RaidAvailable
	}
	if p.IntervalSec != nil {
		updates["interval_sec"] = *p.IntervalSec
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
	return s.mutations.projected(serverID, s.projection, func() error {
		return s.updateStatic(ctx, secret, serverID, patch)
	})
}

func (s *Store) updateStatic(ctx context.Context, secret string, serverID int64, patch ServerStaticPatch) error {
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
	updates := patch.updates()
	if name := strings.TrimSpace(old.Name); name != "" && name != "Untitled" {
		delete(updates, "name")
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

	// Only PostgreSQL-owned fields represented in frontend metadata invalidate
	// Redis. Runtime samples, SMART data and catalog membership stay intact.
	if staticUpdatesFrontMetadata(updates) {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout)
		cacheErr := s.front.RemoveNodeMetadata(cacheCtx, serverID)
		cancel()
		if cacheErr != nil {
			return fmt.Errorf("%w: %w", ErrFrontCacheUpdate, cacheErr)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return s.reconcileCommit(ctx, []int64{serverID}, err)
	}
	committed = true
	s.syncServerCache(fresh, old.Secret)
	return nil
}

func staticUpdatesFrontMetadata(updates map[string]any) bool {
	for _, field := range []string{
		"name",
		"os",
		"platform",
		"platform_version",
		"kernel_version",
		"arch",
		"cpu_model",
		"cpu_cores_physical",
		"cpu_cores_logical",
		"cpu_sockets",
		"mem_total",
		"swap_total",
		"root_path",
		"root_fs_type",
	} {
		if _, ok := updates[field]; ok {
			return true
		}
	}
	return false
}
