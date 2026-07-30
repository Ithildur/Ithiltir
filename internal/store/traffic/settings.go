package traffic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dash/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const trafficSettingsID int16 = 1

type GuestAccessMode string
type UsageMode string
type BillingCycleMode string
type ServerCycleMode string
type ServerDirectionMode string
type DirectionMode string

const (
	GuestAccessDisabled GuestAccessMode = "disabled"
	GuestAccessByNode   GuestAccessMode = "by_node"

	UsageLite    UsageMode = "lite"
	UsageBilling UsageMode = "billing"

	CycleCalendarMonth BillingCycleMode = "calendar_month"
	CycleWHMCS         BillingCycleMode = "whmcs_compatible"
	CycleClampMonthEnd BillingCycleMode = "clamp_to_month_end"

	// ServerCycleDefault is an input-only compatibility alias. Normalization
	// converts it to an explicit calendar-month cycle; it is never persisted.
	ServerCycleDefault ServerCycleMode = "default"

	ServerDirectionDefault ServerDirectionMode = "default"

	DirectionOut  DirectionMode = "out"
	DirectionBoth DirectionMode = "both"
	DirectionMax  DirectionMode = "max"
)

var (
	ErrInvalidServerCycleMode       = errors.New("invalid server cycle mode")
	ErrInvalidServerCycleStartDay   = errors.New("invalid server cycle billing start day")
	ErrInvalidServerCycleAnchorDate = errors.New("invalid server cycle billing anchor date")
	ErrInvalidServerCycleTimezone   = errors.New("invalid server cycle billing timezone")
	ErrInvalidServerDirectionMode   = errors.New("invalid server direction mode")
	ErrInvalidSettings              = errors.New("invalid traffic settings")
	ErrInvalidSettingsPatch         = errors.New("invalid traffic settings patch")
)

type Settings struct {
	GuestAccessMode   GuestAccessMode  `json:"guest_access_mode"`
	UsageMode         UsageMode        `json:"usage_mode"`
	CycleMode         BillingCycleMode `json:"cycle_mode"`
	BillingStartDay   int              `json:"billing_start_day"`
	BillingAnchorDate string           `json:"billing_anchor_date,omitempty"`
	BillingTimezone   string           `json:"billing_timezone,omitempty"`
	DirectionMode     DirectionMode    `json:"direction_mode"`
}

type SettingsPatch struct {
	GuestAccessMode *GuestAccessMode
	UsageMode       *UsageMode
	DirectionMode   *DirectionMode
}

type ServerCycleSettings struct {
	Mode              ServerCycleMode
	BillingStartDay   int
	BillingAnchorDate string
	BillingTimezone   string
}

func NormalizeGuestAccessMode(mode GuestAccessMode) (GuestAccessMode, bool) {
	switch mode {
	case GuestAccessDisabled:
		return GuestAccessDisabled, true
	case GuestAccessByNode:
		return GuestAccessByNode, true
	default:
		return GuestAccessDisabled, false
	}
}

func NormalizeUsageMode(mode UsageMode) (UsageMode, bool) {
	switch mode {
	case UsageLite:
		return UsageLite, true
	case UsageBilling:
		return UsageBilling, true
	default:
		return UsageLite, false
	}
}

func NormalizeCycleMode(mode BillingCycleMode) (BillingCycleMode, bool) {
	switch mode {
	case CycleCalendarMonth:
		return CycleCalendarMonth, true
	case CycleWHMCS:
		return CycleWHMCS, true
	case CycleClampMonthEnd:
		return CycleClampMonthEnd, true
	default:
		return CycleCalendarMonth, false
	}
}

func NormalizeServerCycleMode(mode ServerCycleMode) (ServerCycleMode, bool) {
	switch mode {
	case ServerCycleDefault:
		return ServerCycleDefault, true
	case ServerCycleMode(CycleCalendarMonth):
		return ServerCycleMode(CycleCalendarMonth), true
	case ServerCycleMode(CycleWHMCS):
		return ServerCycleMode(CycleWHMCS), true
	case ServerCycleMode(CycleClampMonthEnd):
		return ServerCycleMode(CycleClampMonthEnd), true
	default:
		return ServerCycleDefault, false
	}
}

func NormalizeServerDirectionMode(mode ServerDirectionMode) (ServerDirectionMode, bool) {
	switch mode {
	case ServerDirectionDefault:
		return ServerDirectionDefault, true
	case ServerDirectionMode(DirectionOut):
		return ServerDirectionMode(DirectionOut), true
	case ServerDirectionMode(DirectionBoth):
		return ServerDirectionMode(DirectionBoth), true
	case ServerDirectionMode(DirectionMax):
		return ServerDirectionMode(DirectionMax), true
	default:
		return ServerDirectionDefault, false
	}
}

func NormalizeServerCycleSettings(cycle ServerCycleSettings) (ServerCycleSettings, error) {
	mode, ok := NormalizeServerCycleMode(cycle.Mode)
	if !ok {
		return ServerCycleSettings{}, ErrInvalidServerCycleMode
	}

	timezone := strings.TrimSpace(cycle.BillingTimezone)
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return ServerCycleSettings{}, ErrInvalidServerCycleTimezone
		}
	}

	anchor := strings.TrimSpace(cycle.BillingAnchorDate)
	anchorDay := 0
	if mode == ServerCycleMode(CycleWHMCS) || anchor != "" {
		anchorTime, valid := parseTrafficAnchorDate(anchor, time.Local)
		if !valid {
			return ServerCycleSettings{}, ErrInvalidServerCycleAnchorDate
		}
		anchor = formatTrafficAnchorDate(anchorTime)
		anchorDay = anchorTime.Day()
	}

	if mode == ServerCycleDefault {
		return ServerCycleSettings{
			Mode:            ServerCycleMode(CycleCalendarMonth),
			BillingStartDay: 1,
		}, nil
	}

	day := cycle.BillingStartDay
	if mode == ServerCycleMode(CycleCalendarMonth) {
		day = 1
	} else if day < 1 || day > 31 {
		return ServerCycleSettings{}, ErrInvalidServerCycleStartDay
	}

	if mode == ServerCycleMode(CycleWHMCS) {
		day = anchorDay
	} else if mode != ServerCycleMode(CycleWHMCS) {
		anchor = ""
	}

	return ServerCycleSettings{
		Mode:              mode,
		BillingStartDay:   day,
		BillingAnchorDate: anchor,
		BillingTimezone:   timezone,
	}, nil
}

func SettingsWithServerCycle(settings Settings, cycle ServerCycleSettings) (Settings, error) {
	normalized, err := NormalizeSettings(settings)
	if err != nil {
		return Settings{}, err
	}
	cycle, err = NormalizeServerCycleSettings(cycle)
	if err != nil {
		return Settings{}, err
	}
	normalized.CycleMode = BillingCycleMode(cycle.Mode)
	normalized.BillingStartDay = cycle.BillingStartDay
	normalized.BillingAnchorDate = cycle.BillingAnchorDate
	normalized.BillingTimezone = cycle.BillingTimezone
	return NormalizeSettings(normalized)
}

func SettingsWithServerDirection(settings Settings, mode ServerDirectionMode) (Settings, error) {
	normalized, err := NormalizeSettings(settings)
	if err != nil {
		return Settings{}, err
	}
	mode, ok := NormalizeServerDirectionMode(mode)
	if !ok {
		return Settings{}, ErrInvalidServerDirectionMode
	}
	if mode == ServerDirectionDefault {
		return normalized, nil
	}
	normalized.DirectionMode = DirectionMode(mode)
	return NormalizeSettings(normalized)
}

func NormalizeDirectionMode(mode DirectionMode) (DirectionMode, bool) {
	switch mode {
	case DirectionOut:
		return DirectionOut, true
	case DirectionBoth:
		return DirectionBoth, true
	case DirectionMax:
		return DirectionMax, true
	default:
		return DirectionOut, false
	}
}

func NormalizeSettings(settings Settings) (Settings, error) {
	guest, valid := NormalizeGuestAccessMode(settings.GuestAccessMode)
	if !valid {
		return Settings{}, fmt.Errorf("%w: guest access mode", ErrInvalidSettings)
	}
	usage, valid := NormalizeUsageMode(settings.UsageMode)
	if !valid {
		return Settings{}, fmt.Errorf("%w: usage mode", ErrInvalidSettings)
	}
	cycle, valid := NormalizeCycleMode(settings.CycleMode)
	if !valid {
		return Settings{}, fmt.Errorf("%w: billing cycle mode", ErrInvalidSettings)
	}
	direction, valid := NormalizeDirectionMode(settings.DirectionMode)
	if !valid {
		return Settings{}, fmt.Errorf("%w: direction mode", ErrInvalidSettings)
	}
	anchor := strings.TrimSpace(settings.BillingAnchorDate)
	billingTimezone := strings.TrimSpace(settings.BillingTimezone)
	if billingTimezone != "" {
		if _, err := time.LoadLocation(billingTimezone); err != nil {
			return Settings{}, fmt.Errorf("%w: billing timezone", ErrInvalidSettings)
		}
	}
	day := settings.BillingStartDay
	if day < 1 || day > 31 {
		return Settings{}, fmt.Errorf("%w: billing start day", ErrInvalidSettings)
	}
	if cycle == CycleCalendarMonth {
		day = 1
		anchor = ""
	}
	if cycle == CycleWHMCS {
		anchorTime, valid := parseTrafficAnchorDate(anchor, time.Local)
		if !valid {
			return Settings{}, fmt.Errorf("%w: billing anchor date", ErrInvalidSettings)
		}
		anchor = formatTrafficAnchorDate(anchorTime)
		day = anchorTime.Day()
	} else if cycle != CycleWHMCS {
		anchor = ""
	}

	return Settings{
		GuestAccessMode:   guest,
		UsageMode:         usage,
		CycleMode:         cycle,
		BillingStartDay:   day,
		BillingAnchorDate: anchor,
		BillingTimezone:   billingTimezone,
		DirectionMode:     direction,
	}, nil
}

func SettingsLocation(settings Settings, fallback *time.Location) (*time.Location, error) {
	return billingLocation(settings.BillingTimezone, fallback)
}

func billingLocation(timezone string, fallback *time.Location) (*time.Location, error) {
	if fallback == nil {
		return nil, fmt.Errorf("traffic settings location is nil")
	}
	if timezone == "" {
		return fallback, nil
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load traffic billing timezone %q: %w", timezone, err)
	}
	return loc, nil
}

func SettingsWithTimezone(settings Settings, fallback *time.Location) (Settings, error) {
	normalized, err := NormalizeSettings(settings)
	if err != nil {
		return Settings{}, err
	}
	if normalized.BillingTimezone != "" {
		return normalized, nil
	}
	if fallback == nil {
		return Settings{}, fmt.Errorf("traffic settings location is nil")
	}
	normalized.BillingTimezone = fallback.String()
	return normalized, nil
}

func trafficSettingFromSettings(settings Settings) model.TrafficSetting {
	return model.TrafficSetting{
		ID:                trafficSettingsID,
		GuestAccessMode:   string(settings.GuestAccessMode),
		UsageMode:         string(settings.UsageMode),
		CycleMode:         string(settings.CycleMode),
		BillingStartDay:   int16(settings.BillingStartDay),
		BillingAnchorDate: settings.BillingAnchorDate,
		BillingTimezone:   settings.BillingTimezone,
		DirectionMode:     string(settings.DirectionMode),
	}
}

func settingsFromTrafficSetting(item model.TrafficSetting) Settings {
	return Settings{
		GuestAccessMode:   GuestAccessMode(item.GuestAccessMode),
		UsageMode:         UsageMode(item.UsageMode),
		CycleMode:         BillingCycleMode(item.CycleMode),
		BillingStartDay:   int(item.BillingStartDay),
		BillingAnchorDate: item.BillingAnchorDate,
		BillingTimezone:   item.BillingTimezone,
		DirectionMode:     DirectionMode(item.DirectionMode),
	}
}

func (patch SettingsPatch) apply(current Settings) (Settings, error) {
	next := current
	if patch.GuestAccessMode != nil {
		next.GuestAccessMode = *patch.GuestAccessMode
	}
	if patch.UsageMode != nil {
		next.UsageMode = *patch.UsageMode
	}
	if patch.DirectionMode != nil {
		next.DirectionMode = *patch.DirectionMode
	}
	return NormalizeSettings(next)
}

func loadTrafficSetting(db *gorm.DB) (model.TrafficSetting, error) {
	var item model.TrafficSetting
	err := db.
		Where("id = ?", trafficSettingsID).
		First(&item).Error
	if err != nil {
		return model.TrafficSetting{}, fmt.Errorf("load traffic settings: %w", err)
	}
	return item, nil
}

func (s *Store) loadSettings(ctx context.Context) (model.TrafficSetting, error) {
	return loadTrafficSetting(s.db.WithContext(ctx))
}

func saveTrafficSetting(db *gorm.DB, item model.TrafficSetting) error {
	item.ID = trafficSettingsID
	result := db.
		Model(&model.TrafficSetting{}).
		Where("id = ?", trafficSettingsID).
		Updates(map[string]any{
			"guest_access_mode":   item.GuestAccessMode,
			"usage_mode":          item.UsageMode,
			"cycle_mode":          item.CycleMode,
			"billing_start_day":   item.BillingStartDay,
			"billing_anchor_date": item.BillingAnchorDate,
			"billing_timezone":    item.BillingTimezone,
			"direction_mode":      item.DirectionMode,
		})
	if result.Error != nil {
		return fmt.Errorf("save traffic settings: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("save traffic settings: singleton row is missing")
	}
	return nil
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return Settings{}, err
	}

	settings := settingsFromTrafficSetting(item)
	normalized, err := NormalizeSettings(settings)
	if err != nil {
		return Settings{}, fmt.Errorf("stored traffic settings: %w", err)
	}
	return normalized, nil
}

// PatchSettingsAt merges mutable global fields against a locked committed row.
// Billing cycles are node-owned and are not part of this patch contract.
func (s *Store) PatchSettingsAt(ctx context.Context, patch SettingsPatch, ref time.Time) (Settings, error) {
	if s == nil || s.db == nil {
		return Settings{}, fmt.Errorf("store: db is nil")
	}
	if ref.IsZero() {
		ref = time.Now()
	}

	var committed Settings
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, err := loadTrafficSetting(tx.Clauses(clause.Locking{Strength: "UPDATE"}))
		if err != nil {
			return err
		}
		stored, err := NormalizeSettings(settingsFromTrafficSetting(item))
		if err != nil {
			return fmt.Errorf("stored traffic settings: %w", err)
		}
		next, err := patch.apply(stored)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidSettingsPatch, err)
		}
		if err := resetFactsProgressForBilling(tx, stored.UsageMode, next.UsageMode, ref); err != nil {
			return err
		}
		if err := saveTrafficSetting(tx, trafficSettingFromSettings(next)); err != nil {
			return err
		}
		committed = next
		return nil
	})
	if err != nil {
		return Settings{}, err
	}
	return committed, nil
}

// PrepareServerCycleChange locks the current node cycle and invalidates only
// derived data whose cycle may be reinterpreted by the immediate update. The
// caller writes the normalized server fields in the same transaction.
func PrepareServerCycleChange(tx *gorm.DB, serverID int64, next ServerCycleSettings, fallback *time.Location, ref time.Time) error {
	if tx == nil {
		return fmt.Errorf("prepare server traffic cycle: db is nil")
	}
	if serverID <= 0 {
		return fmt.Errorf("prepare server traffic cycle: invalid server id")
	}
	if fallback == nil {
		return fmt.Errorf("prepare server traffic cycle: location is nil")
	}
	if ref.IsZero() {
		ref = time.Now()
	}
	next, err := NormalizeServerCycleSettings(next)
	if err != nil {
		return err
	}

	var row struct {
		CycleMode         string `gorm:"column:traffic_cycle_mode"`
		BillingStartDay   int16  `gorm:"column:traffic_billing_start_day"`
		BillingAnchorDate string `gorm:"column:traffic_billing_anchor_date"`
		BillingTimezone   string `gorm:"column:traffic_billing_timezone"`
	}
	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Model(&model.Server{}).
		Select("traffic_cycle_mode", "traffic_billing_start_day", "traffic_billing_anchor_date", "traffic_billing_timezone").
		Where("id = ? AND is_deleted = ?", serverID, false).
		Take(&row).Error; err != nil {
		return err
	}
	current, err := NormalizeServerCycleSettings(ServerCycleSettings{
		Mode:              ServerCycleMode(row.CycleMode),
		BillingStartDay:   int(row.BillingStartDay),
		BillingAnchorDate: row.BillingAnchorDate,
		BillingTimezone:   row.BillingTimezone,
	})
	if err != nil {
		return fmt.Errorf("server %d traffic cycle settings: %w", serverID, err)
	}
	if sameServerCycle(current, next) {
		return nil
	}
	from, err := cycleRebuildStart(current, next, fallback, ref)
	if err != nil {
		return err
	}
	return resetCycleDerived(tx, serverID, from)
}

func sameServerCycle(left, right ServerCycleSettings) bool {
	return left.Mode == right.Mode &&
		left.BillingStartDay == right.BillingStartDay &&
		strings.TrimSpace(left.BillingAnchorDate) == strings.TrimSpace(right.BillingAnchorDate) &&
		strings.TrimSpace(left.BillingTimezone) == strings.TrimSpace(right.BillingTimezone)
}

func cycleRebuildStart(current, next ServerCycleSettings, fallback *time.Location, ref time.Time) (time.Time, error) {
	currentLoc, err := billingLocation(current.BillingTimezone, fallback)
	if err != nil {
		return time.Time{}, err
	}
	nextLoc, err := billingLocation(next.BillingTimezone, fallback)
	if err != nil {
		return time.Time{}, err
	}
	currentRule, err := newCycleRule(
		BillingCycleMode(current.Mode),
		current.BillingStartDay,
		current.BillingAnchorDate,
		currentLoc,
	)
	if err != nil {
		return time.Time{}, err
	}
	nextRule, err := newCycleRule(
		BillingCycleMode(next.Mode),
		next.BillingStartDay,
		next.BillingAnchorDate,
		nextLoc,
	)
	if err != nil {
		return time.Time{}, err
	}
	currentCycle, err := currentRule.at(ref)
	if err != nil {
		return time.Time{}, err
	}
	nextCycle, err := nextRule.at(ref)
	if err != nil {
		return time.Time{}, err
	}
	return minTime(currentCycle.Start, nextCycle.Start), nil
}

func resetCycleDerived(tx *gorm.DB, serverID int64, from time.Time) error {
	if _, err := lockMaterializationProgress(tx, materializationUsage); err != nil {
		return err
	}
	if err := tx.
		Where("server_id = ? AND cycle_end > ?", serverID, from.UTC()).
		Delete(&model.TrafficMonthUsage{}).Error; err != nil {
		return fmt.Errorf("delete server traffic usage after cycle change: %w", err)
	}
	if err := tx.
		Where("server_id = ? AND cycle_end > ?", serverID, from.UTC()).
		Delete(&model.TrafficMonthly{}).Error; err != nil {
		return fmt.Errorf("delete server traffic snapshots after cycle change: %w", err)
	}
	return enqueueTrafficUsageRepair(tx, serverID, from)
}

func enqueueTrafficUsageRepair(tx *gorm.DB, serverID int64, from time.Time) error {
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "server_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"scanned_until": gorm.Expr(
				"LEAST(traffic_usage_repairs.scanned_until, EXCLUDED.scanned_until)",
			),
		}),
	}).Create(&model.TrafficUsageRepair{
		ServerID:     serverID,
		ScannedUntil: from.UTC(),
	}).Error
	if err != nil {
		return fmt.Errorf("enqueue server traffic usage repair: %w", err)
	}
	return nil
}
