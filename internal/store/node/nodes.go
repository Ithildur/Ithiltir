package node

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"dash/internal/config"
	"dash/internal/model"
	trafficstore "dash/internal/store/traffic"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	secretLength   = 16
	secretAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

// NodeUpdate holds typed fields for updating a server node.
// Nil fields are not updated; non-nil fields are applied.
// GroupIDs uses tri-state semantics: nil = no change, empty = clear all, non-empty = replace.
type NodeUpdate struct {
	Name                     *string
	IsGuestVisible           *bool
	TrafficP95Enabled        *bool
	TrafficCycleMode         *trafficstore.ServerCycleMode
	TrafficBillingStartDay   *int
	TrafficBillingAnchorDate *string
	TrafficBillingTimezone   *string
	DisplayOrder             *int
	Tags                     *datatypes.JSON
	Secret                   *string
	GroupIDs                 *[]int64
}

type NodeItem struct {
	ID                       int64          `json:"id"`
	Name                     string         `json:"name"`
	Hostname                 string         `json:"hostname"`
	IP                       *string        `json:"ip,omitempty"`
	OS                       *string        `json:"os,omitempty"`
	Arch                     *string        `json:"arch,omitempty"`
	IsGuestVisible           bool           `json:"is_guest_visible"`
	TrafficP95Enabled        bool           `json:"traffic_p95_enabled"`
	TrafficCycleMode         string         `json:"traffic_cycle_mode"`
	TrafficBillingStartDay   int16          `json:"traffic_billing_start_day"`
	TrafficBillingAnchorDate string         `json:"traffic_billing_anchor_date"`
	TrafficBillingTimezone   string         `json:"traffic_billing_timezone"`
	Secret                   string         `json:"secret"`
	Tags                     datatypes.JSON `json:"tags"`
	DisplayOrder             int            `json:"display_order"`
	GroupIDs                 []int64        `json:"group_ids" gorm:"-"`
	AgentVersion             *string        `json:"version" gorm:"column:agent_version"`
}

// Nodes returns nodes with basic fields for listing.
func (s *Store) Nodes(ctx context.Context) ([]NodeItem, error) {
	var nodes []NodeItem
	err := s.db.WithContext(ctx).
		Model(&model.Server{}).
		Select("id", "name", "hostname", "ip", "os", "arch", "is_guest_visible", "traffic_p95_enabled", "traffic_cycle_mode", "traffic_billing_start_day", "traffic_billing_anchor_date", "traffic_billing_timezone", "secret", "tags", "display_order", "agent_version").
		Where("is_deleted = ?", false).
		Order("display_order DESC").
		Find(&nodes).
		Error
	return nodes, err
}

func (s *Store) GroupRelations(ctx context.Context, serverIDs []int64) ([]model.ServerGroup, error) {
	if len(serverIDs) == 0 {
		return nil, nil
	}
	var relations []model.ServerGroup
	err := s.db.WithContext(ctx).
		Where("server_id IN ?", serverIDs).
		Find(&relations).
		Error
	return relations, err
}

func (s *Store) CreateNode(ctx context.Context, secret string) (model.Server, error) {
	var created model.Server
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return created, tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()

	defaultGroupID, err := ensureDefaultGroupID(ctx, tx)
	if err != nil {
		return created, err
	}

	var maxOrder int
	if err := tx.Model(&model.Server{}).
		Where("is_deleted = ?", false).
		Select("COALESCE(MAX(display_order), 0)").
		Scan(&maxOrder).Error; err != nil {
		return created, err
	}

	srv := model.Server{
		Name:         "Untitled",
		Hostname:     "Untitled",
		Secret:       secret,
		AgentVersion: strPtr("unknown"),
		DisplayOrder: maxOrder + 1,
	}
	if err := tx.Create(&srv).Error; err != nil {
		if isDuplicateError(err) {
			return created, ErrDuplicateSecret
		}
		return created, err
	}

	rel := model.ServerGroup{
		ServerID: srv.ID,
		GroupID:  defaultGroupID,
	}
	if err := tx.Create(&rel).Error; err != nil {
		return created, err
	}

	if err := tx.Commit().Error; err != nil {
		_ = s.RefreshMetaByID(ctx, srv.ID)
		return created, err
	}
	committed = true
	created = srv

	var syncErr error
	if err := s.clearFrontSnapshotCache(ctx); err != nil {
		syncErr = errors.Join(syncErr, err)
	}
	if err := s.SyncServerCache(ctx, srv); err != nil {
		_ = s.RefreshMetaByID(ctx, srv.ID)
		syncErr = errors.Join(syncErr, fmt.Errorf("%w: %w", ErrServerMetaCacheUpdate, err))
	}
	return created, syncErr
}

func (s *Store) GenerateSecret() (string, error) {
	return randomString(secretLength, secretAlphabet)
}

func randomString(n int, alphabet string) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[num.Int64()]
	}
	return string(b), nil
}

func strPtr(s string) *string {
	return &s
}

func isDuplicateError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "UNIQUE constraint")
}

func (s *Store) UpdateNode(ctx context.Context, id int64, upd NodeUpdate) error {
	if err := s.patchNode(ctx, id, upd); err != nil {
		if isForeignKeyViolation(err) {
			return ErrInvalidGroupIDs
		}
		return err
	}
	return nil
}

func needsMetaRefresh(upd NodeUpdate) bool {
	return upd.Name != nil || upd.DisplayOrder != nil || upd.Secret != nil || upd.Tags != nil
}

func (s *Store) loadServerMeta(tx *gorm.DB, id int64) (model.Server, error) {
	var srv model.Server
	err := tx.Model(&model.Server{}).
		Select(serverMetaSelectColumns).
		Where("id = ? AND is_deleted = ?", id, false).
		Take(&srv).Error
	return srv, err
}

func (s *Store) patchNode(ctx context.Context, id int64, upd NodeUpdate) error {
	var old model.Server
	var fresh model.Server

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

	needOld := needsMetaRefresh(upd)
	if needOld {
		srv, err := s.loadServerMeta(tx, id)
		if err != nil {
			return err
		}
		old = srv
	}

	if upd.Secret != nil {
		secret := strings.TrimSpace(*upd.Secret)
		if secret == "" {
			return ErrInvalidSecret
		}
		upd.Secret = &secret
	}

	if err := s.patchFields(tx, id, upd); err != nil {
		return err
	}
	if err := s.syncGroups(tx, id, upd.GroupIDs); err != nil {
		return err
	}

	if needsMetaRefresh(upd) {
		var err error
		fresh, err = s.loadServerMeta(tx, id)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		_ = s.RefreshMetaByID(ctx, id)
		return err
	}
	committed = true

	var syncErr error
	if err := s.syncFrontNodeCache(ctx, id, upd); err != nil {
		syncErr = errors.Join(syncErr, err)
	}
	if needsMetaRefresh(upd) {
		if err := s.syncServerCache(ctx, fresh, old.Secret); err != nil {
			_ = s.RefreshMetaByID(ctx, id)
			syncErr = errors.Join(syncErr, fmt.Errorf("%w: %w", ErrServerMetaCacheUpdate, err))
		}
	}
	return syncErr
}

func (s *Store) patchFields(tx *gorm.DB, id int64, upd NodeUpdate) error {
	fields := patchFields(upd)
	if len(fields) > 0 {
		res := tx.Model(&model.Server{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Updates(fields)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}
	var exists int64
	if err := tx.Model(&model.Server{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Select("id").
		Take(&exists).Error; err != nil {
		return err
	}
	return nil
}

func (s *Store) syncGroups(tx *gorm.DB, serverID int64, groupIDs *[]int64) error {
	if groupIDs == nil {
		return nil
	}
	if err := tx.Where("server_id = ?", serverID).Delete(&model.ServerGroup{}).Error; err != nil {
		return err
	}
	if len(*groupIDs) == 0 {
		return nil
	}
	groups := make([]model.ServerGroup, 0, len(*groupIDs))
	for _, gid := range *groupIDs {
		groups = append(groups, model.ServerGroup{ServerID: serverID, GroupID: gid})
	}
	return tx.Create(&groups).Error
}

func (s *Store) syncFrontNodeCache(ctx context.Context, id int64, upd NodeUpdate) error {
	snapshotPatch := upd.Name != nil || upd.DisplayOrder != nil
	snapshotReset := upd.Tags != nil
	snapshot := snapshotPatch || snapshotReset
	guest := upd.IsGuestVisible != nil
	if s.front == nil || (!snapshot && !guest) {
		return nil
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout)
	defer cancel()

	var err error
	if snapshotReset {
		err = errors.Join(err, s.front.ClearFrontMeta(cacheCtx))
	} else if snapshotPatch {
		if patchErr := s.front.PatchNodeSnapshot(cacheCtx, id, upd.Name, upd.DisplayOrder); patchErr != nil {
			err = errors.Join(err, patchErr, s.front.ClearFrontMeta(cacheCtx))
		}
	}
	if guest {
		err = errors.Join(err, s.front.ClearGuestVisibilityMeta(cacheCtx))
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFrontCacheUpdate, err)
	}
	return nil
}

func (s *Store) removeFrontNodeCache(ctx context.Context, id int64) error {
	if s.front == nil {
		return nil
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout)
	defer cancel()

	if err := s.front.RemoveNodeSnapshot(cacheCtx, id); err != nil {
		err = errors.Join(err, s.front.ClearFrontMeta(cacheCtx), s.front.ClearGuestVisibilityMeta(cacheCtx))
		return fmt.Errorf("%w: %w", ErrFrontCacheUpdate, err)
	}
	return nil
}

func (s *Store) clearFrontSnapshotCache(ctx context.Context) error {
	if s.front == nil {
		return nil
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.RedisWriteTimeout)
	defer cancel()

	if err := s.front.ClearFrontMeta(cacheCtx); err != nil {
		return fmt.Errorf("%w: %w", ErrFrontCacheUpdate, err)
	}
	return nil
}

func (s *Store) syncFrontNodeOrders(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return s.clearFrontSnapshotCache(ctx)
}

func patchFields(upd NodeUpdate) map[string]any {
	m := make(map[string]any)
	if upd.Name != nil {
		m["name"] = *upd.Name
	}
	if upd.IsGuestVisible != nil {
		m["is_guest_visible"] = *upd.IsGuestVisible
	}
	if upd.TrafficP95Enabled != nil {
		m["traffic_p95_enabled"] = *upd.TrafficP95Enabled
	}
	if upd.TrafficCycleMode != nil {
		m["traffic_cycle_mode"] = string(*upd.TrafficCycleMode)
	}
	if upd.TrafficBillingStartDay != nil {
		m["traffic_billing_start_day"] = *upd.TrafficBillingStartDay
	}
	if upd.TrafficBillingAnchorDate != nil {
		m["traffic_billing_anchor_date"] = *upd.TrafficBillingAnchorDate
	}
	if upd.TrafficBillingTimezone != nil {
		m["traffic_billing_timezone"] = *upd.TrafficBillingTimezone
	}
	if upd.DisplayOrder != nil {
		m["display_order"] = *upd.DisplayOrder
	}
	if upd.Tags != nil {
		m["tags"] = *upd.Tags
	}
	if upd.Secret != nil {
		m["secret"] = *upd.Secret
	}
	return m
}

func (s *Store) DeleteNode(ctx context.Context, id int64) error {
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

	old, err := s.loadServerMeta(tx, id)
	if err != nil {
		return err
	}

	res := tx.Model(&model.Server{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Update("is_deleted", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if err := tx.Where("server_id = ?", id).Delete(&model.ServerGroup{}).Error; err != nil {
		return err
	}

	if err := tx.Commit().Error; err != nil {
		_ = s.RefreshMetaByID(ctx, id)
		return err
	}
	committed = true

	var syncErr error
	if err := s.removeFrontNodeCache(ctx, id); err != nil {
		syncErr = errors.Join(syncErr, err)
	}
	if err := s.deleteServerMeta(ctx, id, old.Secret); err != nil {
		_ = s.RefreshMetaByID(ctx, id)
		syncErr = errors.Join(syncErr, fmt.Errorf("%w: %w", ErrServerMetaCacheUpdate, err))
	}
	return syncErr
}

func (s *Store) UpdateDisplayOrder(ctx context.Context, ids []int64) error {
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

	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate id %d", id)
		}
		seen[id] = struct{}{}
	}

	var existing []struct {
		ID int64
	}
	if err := tx.Model(&model.Server{}).
		Select("id").
		Where("is_deleted = ?", false).
		Find(&existing).Error; err != nil {
		return err
	}

	if len(existing) != len(ids) {
		return fmt.Errorf("ids must include all active nodes")
	}
	active := make(map[int64]struct{}, len(existing))
	for _, row := range existing {
		active[row.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := active[id]; !ok {
			return fmt.Errorf("id %d not found", id)
		}
	}

	stmt, args := orderUpdate(ids)
	if err := tx.Exec(stmt, args...).Error; err != nil {
		return err
	}

	var freshRows []model.Server
	if err := tx.Model(&model.Server{}).
		Select(serverMetaSelectColumns).
		Where("id IN ? AND is_deleted = ?", ids, false).
		Find(&freshRows).Error; err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		_ = s.RefreshMetaByIDs(ctx, ids)
		return err
	}
	committed = true

	var syncErr error
	if err := s.syncFrontNodeOrders(ctx, ids); err != nil {
		syncErr = errors.Join(syncErr, err)
	}
	for _, srv := range freshRows {
		if err := s.SyncServerCache(ctx, srv); err != nil {
			_ = s.RefreshMetaByIDs(ctx, ids)
			syncErr = errors.Join(syncErr, fmt.Errorf("%w: %w", ErrServerMetaCacheUpdate, err))
		}
	}
	return syncErr
}

func (s *Store) SetTrafficP95(ctx context.Context, ids []int64, enabled bool) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Server{}).
			Where("id IN ? AND is_deleted = ?", ids, false).
			Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return gorm.ErrRecordNotFound
		}

		return tx.Model(&model.Server{}).
			Where("id IN ? AND is_deleted = ?", ids, false).
			Update("traffic_p95_enabled", enabled).
			Error
	})
}

func (s *Store) GetServerIP(ctx context.Context, serverID int64) (string, bool, error) {
	if s == nil {
		return "", false, errors.New("store is nil")
	}
	if s.db == nil {
		return "", false, errors.New("store DB is nil")
	}
	if serverID <= 0 {
		return "", false, fmt.Errorf("invalid server id %d", serverID)
	}
	type row struct {
		IP *string
	}
	var out row
	if err := s.db.WithContext(ctx).
		Table("servers").
		Select("ip").
		Where("id = ?", serverID).
		Take(&out).
		Error; err != nil {
		return "", false, err
	}
	if out.IP == nil || *out.IP == "" {
		return "", false, nil
	}
	return *out.IP, true, nil
}

func orderUpdate(ids []int64) (string, []any) {
	var sb strings.Builder
	sb.Grow(64 + len(ids)*16)
	sb.WriteString("UPDATE servers SET display_order = CASE id ")

	args := make([]any, 0, len(ids)*3)
	for i, id := range ids {
		sb.WriteString("WHEN ? THEN CAST(? AS integer) ")
		args = append(args, id, int64(len(ids)-i))
	}
	sb.WriteString("END WHERE is_deleted = false AND id IN (")
	for i, id := range ids {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('?')
		args = append(args, id)
	}
	sb.WriteByte(')')
	return sb.String(), args
}
