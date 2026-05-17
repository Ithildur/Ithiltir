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
type DirectionMode string

const (
	GuestAccessDisabled GuestAccessMode = "disabled"
	GuestAccessByNode   GuestAccessMode = "by_node"

	UsageLite    UsageMode = "lite"
	UsageBilling UsageMode = "billing"

	CycleCalendarMonth BillingCycleMode = "calendar_month"
	CycleWHMCS         BillingCycleMode = "whmcs_compatible"
	CycleClampMonthEnd BillingCycleMode = "clamp_to_month_end"

	ServerCycleDefault ServerCycleMode = "default"

	DirectionOut  DirectionMode = "out"
	DirectionBoth DirectionMode = "both"
	DirectionMax  DirectionMode = "max"
)

var (
	ErrInvalidServerCycleMode       = errors.New("invalid server cycle mode")
	ErrInvalidServerCycleStartDay   = errors.New("invalid server cycle billing start day")
	ErrInvalidServerCycleAnchorDate = errors.New("invalid server cycle billing anchor date")
	ErrInvalidServerCycleTimezone   = errors.New("invalid server cycle billing timezone")
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

type ServerCycleSettings struct {
	Mode              ServerCycleMode
	BillingStartDay   int
	BillingAnchorDate string
	BillingTimezone   string
}

func DefaultSettings() Settings {
	return Settings{
		GuestAccessMode: GuestAccessDisabled,
		UsageMode:       UsageLite,
		CycleMode:       CycleCalendarMonth,
		BillingStartDay: 1,
		DirectionMode:   DirectionOut,
	}
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
	case "", UsageLite:
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
	case "", ServerCycleDefault:
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

func NormalizeServerCycleSettings(cycle ServerCycleSettings) (ServerCycleSettings, error) {
	mode, ok := NormalizeServerCycleMode(cycle.Mode)
	if !ok {
		return ServerCycleSettings{Mode: ServerCycleDefault}, ErrInvalidServerCycleMode
	}

	timezone := strings.TrimSpace(cycle.BillingTimezone)
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return ServerCycleSettings{Mode: ServerCycleDefault}, ErrInvalidServerCycleTimezone
		}
	}

	anchor := strings.TrimSpace(cycle.BillingAnchorDate)
	anchorDay := 0
	if anchor != "" {
		anchorTime, valid := parseTrafficAnchorDate(anchor, time.Local)
		if !valid {
			return ServerCycleSettings{Mode: ServerCycleDefault}, ErrInvalidServerCycleAnchorDate
		}
		anchor = formatTrafficAnchorDate(anchorTime)
		anchorDay = anchorTime.Day()
	}

	if mode == ServerCycleDefault {
		return ServerCycleSettings{Mode: ServerCycleDefault, BillingStartDay: 1}, nil
	}

	day := cycle.BillingStartDay
	if mode == ServerCycleMode(CycleCalendarMonth) {
		day = 1
	} else if day < 1 || day > 31 {
		return ServerCycleSettings{Mode: ServerCycleDefault}, ErrInvalidServerCycleStartDay
	}

	if mode == ServerCycleMode(CycleWHMCS) && anchor != "" {
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

func SettingsWithServerCycleSettings(settings Settings, cycle ServerCycleSettings) Settings {
	normalized, _ := NormalizeSettings(settings)
	cycle, err := NormalizeServerCycleSettings(cycle)
	if err != nil || cycle.Mode == ServerCycleDefault {
		return normalized
	}
	normalized.CycleMode = BillingCycleMode(cycle.Mode)
	normalized.BillingStartDay = cycle.BillingStartDay
	normalized.BillingAnchorDate = cycle.BillingAnchorDate
	normalized.BillingTimezone = cycle.BillingTimezone
	next, _ := NormalizeSettings(normalized)
	return next
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

func NormalizeSettings(settings Settings) (Settings, bool) {
	defaults := DefaultSettings()
	ok := true

	guest, valid := NormalizeGuestAccessMode(settings.GuestAccessMode)
	if !valid {
		guest = defaults.GuestAccessMode
		ok = false
	}
	usage, valid := NormalizeUsageMode(settings.UsageMode)
	if !valid {
		usage = defaults.UsageMode
		ok = false
	}
	cycle, valid := NormalizeCycleMode(settings.CycleMode)
	if !valid {
		cycle = defaults.CycleMode
		ok = false
	}
	direction, valid := NormalizeDirectionMode(settings.DirectionMode)
	if !valid {
		direction = defaults.DirectionMode
		ok = false
	}
	anchor := strings.TrimSpace(settings.BillingAnchorDate)
	billingTimezone := strings.TrimSpace(settings.BillingTimezone)
	if billingTimezone != "" {
		if _, err := time.LoadLocation(billingTimezone); err != nil {
			billingTimezone = ""
			ok = false
		}
	}
	day := settings.BillingStartDay
	if day < 1 || day > 31 {
		day = defaults.BillingStartDay
		ok = false
	}
	if cycle == CycleCalendarMonth {
		day = 1
		anchor = ""
	}
	if cycle == CycleWHMCS && anchor != "" {
		anchorTime, valid := parseTrafficAnchorDate(anchor, time.Local)
		if !valid {
			anchor = ""
			ok = false
		} else {
			anchor = formatTrafficAnchorDate(anchorTime)
			day = anchorTime.Day()
		}
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
	}, ok
}

func SettingsLocation(settings Settings, fallback *time.Location) *time.Location {
	if fallback == nil {
		fallback = time.Local
	}
	if settings.BillingTimezone == "" {
		return fallback
	}
	loc, err := time.LoadLocation(settings.BillingTimezone)
	if err != nil {
		return fallback
	}
	return loc
}

func SettingsWithTimezone(settings Settings, fallback *time.Location) Settings {
	normalized, _ := NormalizeSettings(settings)
	if normalized.BillingTimezone != "" {
		return normalized
	}
	if fallback == nil {
		fallback = time.Local
	}
	normalized.BillingTimezone = fallback.String()
	return normalized
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

func defaultTrafficSetting() model.TrafficSetting {
	return trafficSettingFromSettings(DefaultSettings())
}

func (s *Store) loadSettings(ctx context.Context) (model.TrafficSetting, error) {
	var item model.TrafficSetting
	err := s.db.WithContext(ctx).
		Where("id = ?", trafficSettingsID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultTrafficSetting(), nil
		}
		return model.TrafficSetting{}, fmt.Errorf("load traffic settings: %w", err)
	}
	return item, nil
}

func (s *Store) saveSettings(ctx context.Context, item model.TrafficSetting) error {
	item.ID = trafficSettingsID
	err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"guest_access_mode",
				"usage_mode",
				"cycle_mode",
				"billing_start_day",
				"billing_anchor_date",
				"billing_timezone",
				"direction_mode",
			}),
		}).
		Create(&item).Error
	if err != nil {
		return fmt.Errorf("save traffic settings: %w", err)
	}
	return nil
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DefaultSettings(), err
	}

	settings := settingsFromTrafficSetting(item)
	normalized, _ := NormalizeSettings(settings)
	if normalized != settings {
		return DefaultSettings(), fmt.Errorf("invalid traffic settings")
	}
	return normalized, nil
}

func (s *Store) SetSettings(ctx context.Context, settings Settings) error {
	normalized, ok := NormalizeSettings(settings)
	if !ok {
		return fmt.Errorf("invalid traffic settings")
	}
	return s.saveSettings(ctx, trafficSettingFromSettings(normalized))
}
