package node

import (
	"context"
	"encoding/json/v2"
	"time"

	"dash/internal/model"
	"dash/internal/virt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveVirt serializes authentication and persistence with node deletion and key
// rotation. A retry of the same sample never refreshes its received timestamp.
func (s *Store) SaveVirt(ctx context.Context, id int64, secret string, snapshot virt.Snapshot, receivedAt time.Time) error {
	return s.WithMetricsIngest(ctx, id, func() error {
		server, err := s.GetServerBySecret(ctx, secret)
		if err != nil {
			return err
		}
		if server.ID != id {
			return gorm.ErrRecordNotFound
		}
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		row := model.ServerVirt{ServerID: id, CollectedAt: snapshot.CollectedAt, ReceivedAt: receivedAt, Snapshot: datatypes.JSON(raw)}
		return s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "server_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"collected_at", "received_at", "snapshot"}),
			Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "server_virt.collected_at < excluded.collected_at"}}},
		}).Create(&row).Error
	})
}

type VirtView struct {
	Snapshot   *virt.Snapshot `json:"snapshot"`
	ReceivedAt *time.Time     `json:"received_at"`
	Stale      bool           `json:"stale"`
}

func (s *Store) Virt(ctx context.Context, id int64) (VirtView, error) {
	view := VirtView{Stale: true}
	var row model.ServerVirt
	err := s.db.WithContext(ctx).Table("servers").
		Select("servers.id AS server_id, server_virt.collected_at, server_virt.received_at, server_virt.snapshot").
		Joins("LEFT JOIN server_virt ON server_virt.server_id = servers.id").
		Where("servers.id = ? AND NOT servers.is_deleted", id).Take(&row).Error
	if err != nil {
		return view, err
	}
	if len(row.Snapshot) == 0 {
		return view, nil
	}
	var snapshot virt.Snapshot
	if err := json.Unmarshal(row.Snapshot, &snapshot); err != nil {
		return view, err
	}
	view.Snapshot, view.ReceivedAt = &snapshot, &row.ReceivedAt
	ttl := time.Duration(snapshot.TTLSeconds) * time.Second
	view.Stale = snapshot.LastSuccessAt == nil || time.Since(*snapshot.LastSuccessAt) > ttl || time.Since(row.ReceivedAt) > ttl
	return view, nil
}
