package metricdata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dash/internal/metrics"
	"dash/internal/model"
	"github.com/Ithildur/EiluneKit/postgres/dbtypes"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MetricsSample is one node metrics snapshot plus rows derived from the same report.
type MetricsSample struct {
	ServerID  int64
	Metric    model.ServerMetric
	Updates   map[string]any
	DiskIO    []metrics.DiskBaseIOMetrics
	DiskSmart *metrics.DiskSmart
	DiskUsage []metrics.DiskLogicalMetrics
	Network   []metrics.NetIOMetrics
}

// SaveMetrics persists a node metrics snapshot and related tables in a single transaction.
func (s *Store) SaveMetrics(ctx context.Context, sample MetricsSample) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: db is nil")
	}
	diskIORows := buildDiskIORows(sample.ServerID, sample.Metric.CollectedAt, sample.DiskIO)
	diskPhysicalRows := buildDiskPhysicalRows(sample.ServerID, sample.Metric.CollectedAt, sample.DiskSmart)
	diskUsageRows := buildDiskUsageRows(sample.ServerID, sample.Metric.CollectedAt, sample.DiskUsage)
	nicRows := buildNICRows(sample.ServerID, sample.Metric.CollectedAt, sample.Network)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sample.Metric).Error; err != nil {
			return err
		}
		if err := insertDiskIO(tx, diskIORows); err != nil {
			return err
		}
		if err := insertDiskPhysical(tx, diskPhysicalRows); err != nil {
			return err
		}
		if err := insertDiskUsage(tx, diskUsageRows); err != nil {
			return err
		}
		if err := insertNICs(tx, nicRows); err != nil {
			return err
		}
		if err := saveCurrentMetrics(tx, sample.ServerID, sample.Metric, diskIORows, diskUsageRows, nicRows); err != nil {
			return err
		}
		if len(sample.Updates) > 0 {
			if err := tx.Model(&model.Server{}).Where("id = ?", sample.ServerID).Updates(sample.Updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func buildDiskIORows(serverID int64, collectedAt time.Time, items []metrics.DiskBaseIOMetrics) []model.DiskMetric {
	rows := make([]model.DiskMetric, 0, len(items))
	if serverID <= 0 {
		return rows
	}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		ref := strings.TrimSpace(item.Ref)
		if ref == "" {
			ref = name
		}
		row := model.DiskMetric{
			ServerID:             serverID,
			Name:                 name,
			Ref:                  ref,
			Kind:                 strings.TrimSpace(item.Kind),
			Role:                 strings.TrimSpace(item.Role),
			Path:                 strings.TrimSpace(item.DevicePath),
			CollectedAt:          collectedAt,
			ReadBytes:            int64(item.ReadBytes),
			WriteBytes:           int64(item.WriteBytes),
			ReadRateBytesPerSec:  item.ReadRateBytesPerSec,
			WriteRateBytesPerSec: item.WriteRateBytesPerSec,
			IOPS:                 item.IOPS,
			ReadIOPS:             item.ReadIOPS,
			WriteIOPS:            item.WriteIOPS,
			UtilRatio:            item.UtilRatio,
			QueueLength:          item.QueueLength,
			WaitMs:               item.WaitMs,
			ServiceMs:            item.ServiceMs,
		}
		rows = append(rows, row)
	}
	return rows
}

func buildDiskPhysicalRows(serverID int64, collectedAt time.Time, smart *metrics.DiskSmart) []model.DiskPhysicalMetric {
	if serverID <= 0 || smart == nil {
		return nil
	}
	rows := make([]model.DiskPhysicalMetric, 0, len(smart.Devices))
	seen := make(map[string]struct{}, len(smart.Devices))
	for _, device := range smart.Devices {
		name := strings.TrimSpace(device.Name)
		if name == "" || !metrics.IsSmartTemperatureDevice(device) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		ref := strings.TrimSpace(device.Ref)
		if ref == "" {
			ref = name
		}
		rows = append(rows, model.DiskPhysicalMetric{
			ServerID:    serverID,
			Name:        name,
			Ref:         ref,
			Path:        strings.TrimSpace(device.DevicePath),
			CollectedAt: collectedAt,
			TempC:       *device.TempC,
		})
	}
	return rows
}

func insertDiskPhysical(tx *gorm.DB, rows []model.DiskPhysicalMetric) error {
	if tx == nil || len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func insertDiskIO(tx *gorm.DB, rows []model.DiskMetric) error {
	if tx == nil || len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func buildDiskUsageRows(serverID int64, collectedAt time.Time, items []metrics.DiskLogicalMetrics) []model.DiskUsageMetric {
	rows := make([]model.DiskUsageMetric, 0, len(items))
	if serverID <= 0 {
		return rows
	}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		ref := strings.TrimSpace(item.Ref)
		if ref == "" {
			ref = name
		}
		total := int64(item.Total)
		if total == 0 {
			total = int64(item.Used + item.Free)
		}
		rows = append(rows, model.DiskUsageMetric{
			ServerID:    serverID,
			Name:        name,
			Ref:         ref,
			Kind:        strings.TrimSpace(item.Kind),
			Mountpoint:  strings.TrimSpace(item.Mountpoint),
			Path:        strings.TrimSpace(item.DevicePath),
			CollectedAt: collectedAt,
			Total:       total,
			Used:        int64(item.Used),
			Free:        int64(item.Free),
			UsedRatio:   item.UsedRatio,
			FSType:      mountpointFSType(item),
			Devices:     dbtypes.TextArray(sanitizeDevices(item.Devices)),
			Health:      strings.TrimSpace(item.Health),
			Level:       strings.TrimSpace(item.Level),
		})
	}
	return rows
}

func insertDiskUsage(tx *gorm.DB, rows []model.DiskUsageMetric) error {
	if tx == nil || len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func buildNICRows(serverID int64, collectedAt time.Time, items []metrics.NetIOMetrics) []model.NICMetric {
	rows := make([]model.NICMetric, 0, len(items))
	if serverID <= 0 {
		return rows
	}
	for _, item := range items {
		iface := strings.TrimSpace(item.Name)
		if iface == "" {
			continue
		}
		rows = append(rows, model.NICMetric{
			ServerID:              serverID,
			Iface:                 iface,
			CollectedAt:           collectedAt,
			BytesRecv:             int64(item.BytesRecv),
			BytesSent:             int64(item.BytesSent),
			RecvRateBytesPerSec:   item.RecvRateBytesPerSec,
			SentRateBytesPerSec:   item.SentRateBytesPerSec,
			PacketsRecv:           int64(item.PacketsRecv),
			PacketsSent:           int64(item.PacketsSent),
			RecvRatePacketsPerSec: item.RecvRatePacketsPerSec,
			SentRatePacketsPerSec: item.SentRatePacketsPerSec,
			ErrIn:                 int64(item.ErrIn),
			ErrOut:                int64(item.ErrOut),
			DropIn:                int64(item.DropIn),
			DropOut:               int64(item.DropOut),
		})
	}
	return rows
}

func insertNICs(tx *gorm.DB, rows []model.NICMetric) error {
	if tx == nil || len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveCurrentMetrics(tx *gorm.DB, serverID int64, metric model.ServerMetric, diskIO []model.DiskMetric, diskUsage []model.DiskUsageMetric, nics []model.NICMetric) error {
	if tx == nil || serverID <= 0 {
		return nil
	}

	current := model.ServerCurrentMetric{
		ServerID:        serverID,
		CollectedAt:     metric.CollectedAt,
		ReportedAt:      metric.ReportedAt,
		MetricsSnapshot: metric.MetricsSnapshot,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}},
		DoUpdates: clause.AssignmentColumns(model.ServerCurrentMetricUpdateColumns()),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "server_current_metrics.collected_at <= excluded.collected_at"},
		}},
	}).Create(&current)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	if err := replaceCurrentDiskIO(tx, serverID, diskIO); err != nil {
		return err
	}
	if err := replaceCurrentDiskUsage(tx, serverID, diskUsage); err != nil {
		return err
	}
	return replaceCurrentNICs(tx, serverID, nics)
}

func replaceCurrentDiskIO(tx *gorm.DB, serverID int64, rows []model.DiskMetric) error {
	if err := tx.Where("server_id = ?", serverID).Delete(&model.ServerCurrentDiskMetric{}).Error; err != nil {
		return err
	}
	current := currentDiskIORows(rows)
	if len(current) == 0 {
		return nil
	}
	return tx.Create(&current).Error
}

func replaceCurrentDiskUsage(tx *gorm.DB, serverID int64, rows []model.DiskUsageMetric) error {
	if err := tx.Where("server_id = ?", serverID).Delete(&model.ServerCurrentDiskUsageMetric{}).Error; err != nil {
		return err
	}
	current := currentDiskUsageRows(rows)
	if len(current) == 0 {
		return nil
	}
	return tx.Create(&current).Error
}

func replaceCurrentNICs(tx *gorm.DB, serverID int64, rows []model.NICMetric) error {
	if err := tx.Where("server_id = ?", serverID).Delete(&model.ServerCurrentNICMetric{}).Error; err != nil {
		return err
	}
	current := currentNICRows(rows)
	if len(current) == 0 {
		return nil
	}
	return tx.Create(&current).Error
}

func currentDiskIORows(rows []model.DiskMetric) []model.ServerCurrentDiskMetric {
	current := make([]model.ServerCurrentDiskMetric, 0, len(rows))
	for _, row := range rows {
		current = append(current, model.ServerCurrentDiskMetric{
			ServerID:             row.ServerID,
			Name:                 row.Name,
			Ref:                  row.Ref,
			Kind:                 row.Kind,
			Role:                 row.Role,
			Path:                 row.Path,
			CollectedAt:          row.CollectedAt,
			ReadBytes:            row.ReadBytes,
			WriteBytes:           row.WriteBytes,
			ReadRateBytesPerSec:  row.ReadRateBytesPerSec,
			WriteRateBytesPerSec: row.WriteRateBytesPerSec,
			IOPS:                 row.IOPS,
			ReadIOPS:             row.ReadIOPS,
			WriteIOPS:            row.WriteIOPS,
			UtilRatio:            row.UtilRatio,
			QueueLength:          row.QueueLength,
			WaitMs:               row.WaitMs,
			ServiceMs:            row.ServiceMs,
		})
	}
	return current
}

func currentDiskUsageRows(rows []model.DiskUsageMetric) []model.ServerCurrentDiskUsageMetric {
	current := make([]model.ServerCurrentDiskUsageMetric, 0, len(rows))
	for _, row := range rows {
		current = append(current, model.ServerCurrentDiskUsageMetric{
			ServerID:    row.ServerID,
			Name:        row.Name,
			Ref:         row.Ref,
			Kind:        row.Kind,
			Mountpoint:  row.Mountpoint,
			Path:        row.Path,
			CollectedAt: row.CollectedAt,
			Total:       row.Total,
			Used:        row.Used,
			Free:        row.Free,
			UsedRatio:   row.UsedRatio,
			FSType:      row.FSType,
			Devices:     row.Devices,
			Health:      row.Health,
			Level:       row.Level,
		})
	}
	return current
}

func currentNICRows(rows []model.NICMetric) []model.ServerCurrentNICMetric {
	current := make([]model.ServerCurrentNICMetric, 0, len(rows))
	for _, row := range rows {
		current = append(current, model.ServerCurrentNICMetric{
			ServerID:              row.ServerID,
			Iface:                 row.Iface,
			CollectedAt:           row.CollectedAt,
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
			Extra:                 row.Extra,
		})
	}
	return current
}

func mountpointFSType(item metrics.DiskLogicalMetrics) string {
	mountpoint := strings.TrimSpace(item.Mountpoint)
	if mountpoint == "" || len(item.Mountpoints) == 0 {
		return ""
	}
	if info, ok := item.Mountpoints[mountpoint]; ok {
		return strings.TrimSpace(info.FSType)
	}
	return ""
}

func sanitizeDevices(devices []string) []string {
	if len(devices) == 0 {
		return nil
	}
	out := make([]string, 0, len(devices))
	seen := make(map[string]struct{}, len(devices))
	for _, dev := range devices {
		dev = strings.TrimSpace(dev)
		if dev == "" {
			continue
		}
		if _, ok := seen[dev]; ok {
			continue
		}
		seen[dev] = struct{}{}
		out = append(out, dev)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
