package frontcache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/model"
)

func (s *Store) FetchFrontNodes(ctx context.Context, staleAfterSec int, limit, offset int, authorized bool) ([]metrics.NodeView, error) {
	var rows []model.ServerCurrentMetric
	query := s.db.WithContext(ctx).
		Table("server_current_metrics AS scm").
		Select("scm.*").
		Joins("JOIN servers s ON s.id = scm.server_id AND s.is_deleted = ?", false)
	if !authorized {
		query = query.Where("s.is_guest_visible = ?", true)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Order("s.display_order DESC, s.name ASC, s.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(rows))
	for _, m := range rows {
		ids = append(ids, m.ServerID)
	}

	var servers []model.Server
	if err := s.db.WithContext(ctx).
		Where("id IN ?", ids).
		Where("is_deleted = ?", false).
		Find(&servers).Error; err != nil {
		return nil, err
	}
	serversByID := make(map[int64]model.Server, len(servers))
	for _, srv := range servers {
		serversByID[srv.ID] = srv
	}

	logicalByID, err := s.fetchDiskLogical(ctx, ids)
	if err != nil {
		return nil, err
	}
	baseIOByID, err := s.fetchDiskBaseIO(ctx, ids)
	if err != nil {
		return nil, err
	}
	nicsByID, err := s.fetchNICs(ctx, ids)
	if err != nil {
		return nil, err
	}

	nodes := make([]metrics.NodeView, 0, len(rows))
	logger := infra.Log()
	for _, m := range rows {
		srv, ok := serversByID[m.ServerID]
		if !ok {
			continue
		}
		report, err := metrics.BuildNodeReport(srv, m.ToServerMetric())
		if err != nil {
			logger.Warn("build front snapshot failed", err,
				slog.Int64("server_id", m.ServerID),
				slog.Time("collected_at", m.CollectedAt),
			)
			continue
		}
		if logical, ok := logicalByID[m.ServerID]; ok {
			report.Metrics.Disk.Logical = applyRootFSType(logical, srv.RootPath, srv.RootFSType)
		}
		if baseIO, ok := baseIOByID[m.ServerID]; ok {
			report.Metrics.Disk.BaseIO = baseIO
		}
		if nics, ok := nicsByID[m.ServerID]; ok {
			report.Metrics.Network = nics
		}
		view, err := metrics.BuildNodeView(srv, report, staleAfterSec)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, view)
	}
	if err := s.applySmartRuntimeFields(ctx, nodes); err != nil {
		return nil, fmt.Errorf("load smart runtime: %w", err)
	}
	return nodes, nil
}

func (s *Store) fetchDiskLogical(ctx context.Context, ids []int64) (map[int64][]metrics.DiskLogicalMetrics, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []model.ServerCurrentDiskUsageMetric
	if err := s.db.WithContext(ctx).
		Table("server_current_disk_usage_metrics").
		Where("server_id IN ?", ids).
		Order("server_id ASC, name ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[int64][]metrics.DiskLogicalMetrics, len(ids))
	for _, row := range rows {
		mounts := mountsFromDiskUsage(row.Mountpoint, row.FSType)
		out[row.ServerID] = append(out[row.ServerID], metrics.DiskLogicalMetrics{
			Kind:        row.Kind,
			Name:        row.Name,
			DevicePath:  row.Path,
			Ref:         row.Ref,
			Total:       uint64(row.Total),
			Used:        uint64(row.Used),
			Free:        uint64(row.Free),
			UsedRatio:   row.UsedRatio,
			Health:      row.Health,
			Level:       row.Level,
			Mountpoint:  row.Mountpoint,
			Mountpoints: mounts,
			Devices:     []string(row.Devices),
		})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (s *Store) fetchDiskBaseIO(ctx context.Context, ids []int64) (map[int64][]metrics.DiskBaseIOMetrics, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []model.ServerCurrentDiskMetric
	if err := s.db.WithContext(ctx).
		Table("server_current_disk_metrics").
		Where("server_id IN ?", ids).
		Order("server_id ASC, name ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[int64][]metrics.DiskBaseIOMetrics, len(ids))
	for _, row := range rows {
		out[row.ServerID] = append(out[row.ServerID], metrics.DiskBaseIOMetrics{
			Kind:                 row.Kind,
			Name:                 row.Name,
			DevicePath:           row.Path,
			Ref:                  row.Ref,
			Role:                 row.Role,
			ReadBytes:            uint64(row.ReadBytes),
			WriteBytes:           uint64(row.WriteBytes),
			ReadRateBytesPerSec:  row.ReadRateBytesPerSec,
			WriteRateBytesPerSec: row.WriteRateBytesPerSec,
			ReadIOPS:             row.ReadIOPS,
			WriteIOPS:            row.WriteIOPS,
			IOPS:                 row.IOPS,
			UtilRatio:            row.UtilRatio,
			QueueLength:          row.QueueLength,
			WaitMs:               row.WaitMs,
			ServiceMs:            row.ServiceMs,
		})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (s *Store) fetchNICs(ctx context.Context, ids []int64) (map[int64][]metrics.NetIOMetrics, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []model.ServerCurrentNICMetric
	if err := s.db.WithContext(ctx).
		Table("server_current_nic_metrics").
		Where("server_id IN ?", ids).
		Order("server_id ASC, iface ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[int64][]metrics.NetIOMetrics, len(ids))
	for _, row := range rows {
		out[row.ServerID] = append(out[row.ServerID], metrics.NetIOMetrics{
			Name:                  row.Iface,
			BytesRecv:             uint64(row.BytesRecv),
			BytesSent:             uint64(row.BytesSent),
			RecvRateBytesPerSec:   row.RecvRateBytesPerSec,
			SentRateBytesPerSec:   row.SentRateBytesPerSec,
			PacketsRecv:           uint64(row.PacketsRecv),
			PacketsSent:           uint64(row.PacketsSent),
			RecvRatePacketsPerSec: row.RecvRatePacketsPerSec,
			SentRatePacketsPerSec: row.SentRatePacketsPerSec,
			ErrIn:                 uint64(row.ErrIn),
			ErrOut:                uint64(row.ErrOut),
			DropIn:                uint64(row.DropIn),
			DropOut:               uint64(row.DropOut),
		})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func applyRootFSType(items []metrics.DiskLogicalMetrics, rootPath *string, rootFSType *string) []metrics.DiskLogicalMetrics {
	if len(items) == 0 || rootPath == nil || rootFSType == nil {
		return items
	}
	path := strings.TrimSpace(*rootPath)
	fsType := strings.TrimSpace(*rootFSType)
	if path == "" || fsType == "" {
		return items
	}
	for i := range items {
		if strings.TrimSpace(items[i].Mountpoint) != path {
			continue
		}
		if items[i].Mountpoints == nil {
			items[i].Mountpoints = map[string]metrics.DiskMountpointMetrics{}
		}
		if _, ok := items[i].Mountpoints[path]; !ok {
			items[i].Mountpoints[path] = metrics.DiskMountpointMetrics{FSType: fsType}
		}
	}
	return items
}

func mountsFromDiskUsage(mountpoint, fsType string) map[string]metrics.DiskMountpointMetrics {
	mp := strings.TrimSpace(mountpoint)
	fsType = strings.TrimSpace(fsType)
	if mp == "" || fsType == "" {
		return nil
	}
	return map[string]metrics.DiskMountpointMetrics{
		mp: {FSType: fsType},
	}
}
