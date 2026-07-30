package frontcache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/nodetags"
)

func (s *Store) FetchFrontNodes(ctx context.Context, staleAfterSec int, limit, offset int, authorized bool) ([]metrics.NodeView, error) {
	projections, err := s.fetchFrontProjections(ctx, staleAfterSec, limit, offset, authorized, 0, true)
	if err != nil {
		return nil, err
	}
	if len(projections) == 0 {
		return nil, nil
	}
	nodes := make([]metrics.NodeView, 0, len(projections))
	for _, projection := range projections {
		nodes = append(nodes, projection.Node)
	}
	return nodes, nil
}

// FetchCurrentNode reads the authoritative PostgreSQL projection. Alert
// evaluation uses this path when no ingest snapshot is queued; a frontend
// cache miss must never be interpreted as a recovered alert.
func (s *Store) FetchCurrentNode(ctx context.Context, id int64, staleAfterSec int) (*metrics.NodeView, error) {
	if id <= 0 {
		return nil, nil
	}
	projections, err := s.fetchFrontProjections(ctx, staleAfterSec, 1, 0, true, id, false)
	if err != nil || len(projections) == 0 {
		return nil, err
	}
	node := projections[0].Node
	return &node, nil
}

func (s *Store) ListCurrentNodeIDs(ctx context.Context) ([]int64, error) {
	if s == nil || s.db == nil {
		return nil, errMissingDB
	}
	var rows []struct {
		ID int64
	}
	err := s.db.WithContext(ctx).
		Table("server_current_metrics AS scm").
		Select("scm.server_id AS id").
		Joins("JOIN servers s ON s.id = scm.server_id AND s.is_deleted = ?", false).
		Order("scm.server_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (s *Store) fetchFrontProjections(ctx context.Context, staleAfterSec int, limit, offset int, authorized bool, serverID int64, withSmart bool) ([]frontNodeProjection, error) {
	var rows []model.ServerCurrentMetric
	query := s.db.WithContext(ctx).
		Table("server_current_metrics AS scm").
		Select("scm.*").
		Joins("JOIN servers s ON s.id = scm.server_id AND s.is_deleted = ?", false)
	if !authorized {
		query = query.Where("s.is_guest_visible = ?", true)
	}
	if serverID > 0 {
		query = query.Where("scm.server_id = ?", serverID)
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
	projections := make([]frontNodeProjection, 0, len(rows))
	logger := infra.Log()
	for _, m := range rows {
		srv, ok := serversByID[m.ServerID]
		if !ok {
			continue
		}
		report, err := metrics.BuildNodeReport(srv, m)
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
		if _, err := nodetags.ParseStored(srv.Tags); err != nil {
			logger.Warn("discard invalid stored node tags", err,
				slog.Int64("server_id", srv.ID),
			)
		}
		view := metrics.BuildNodeView(srv, report, staleAfterSec)
		nodes = append(nodes, view)
		projections = append(projections, frontNodeProjection{
			Node:        view,
			Meta:        frontNodeMetaFromServer(srv),
			MemoryTotal: m.MemTotal,
			SwapTotal:   m.SwapTotal,
		})
	}
	if withSmart {
		if err := s.applySmartRuntimeFields(ctx, nodes); err != nil {
			return nil, fmt.Errorf("load smart runtime: %w", err)
		}
	}
	for i := range nodes {
		projections[i].Node = nodes[i]
	}
	return projections, nil
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
			Total:       row.Total,
			Used:        row.Used,
			Free:        row.Free,
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
			ReadBytes:            row.ReadBytes,
			WriteBytes:           row.WriteBytes,
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
			BytesRecv:             row.BytesRecv,
			BytesSent:             row.BytesSent,
			RecvRateBytesPerSec:   row.RecvRateBytesPerSec,
			SentRateBytesPerSec:   row.SentRateBytesPerSec,
			PacketsRecv:           row.PacketsRecv,
			PacketsSent:           row.PacketsSent,
			RecvRatePacketsPerSec: row.RecvRatePacketsPerSec,
			SentRatePacketsPerSec: row.SentRatePacketsPerSec,
			ErrIn:                 row.ErrIn,
			ErrOut:                row.ErrOut,
			DropIn:                row.DropIn,
			DropOut:               row.DropOut,
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
