package system

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dash/internal/model"
	appversion "dash/internal/version"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const systemSettingsID int16 = 1

const (
	DefaultSiteLogoURL    = "/brandlogo.svg"
	DefaultSitePageTitle  = "Ithiltir Monitor Dashboard"
	DefaultSiteTopbarText = "Ithiltir Control"
)

type DashUpdateChannel = appversion.Channel
type DashUpdateMode string

const (
	DashUpdateChannelRelease    DashUpdateChannel = appversion.ChannelRelease
	DashUpdateChannelPrerelease DashUpdateChannel = appversion.ChannelPrerelease

	DashUpdateModeManual DashUpdateMode = "manual"
	DashUpdateModeNotify DashUpdateMode = "notify"
	DashUpdateModeAuto   DashUpdateMode = "auto"
)

type SiteBrand struct {
	LogoURL    string `json:"logo_url"`
	PageTitle  string `json:"page_title"`
	TopbarText string `json:"topbar_text"`
}

func DefaultSiteBrand() SiteBrand {
	return SiteBrand{
		LogoURL:    DefaultSiteLogoURL,
		PageTitle:  DefaultSitePageTitle,
		TopbarText: DefaultSiteTopbarText,
	}
}

func NormalizeSiteBrand(brand SiteBrand) SiteBrand {
	out := SiteBrand{
		LogoURL:    strings.TrimSpace(brand.LogoURL),
		PageTitle:  strings.TrimSpace(brand.PageTitle),
		TopbarText: strings.TrimSpace(brand.TopbarText),
	}
	defaults := DefaultSiteBrand()
	if out.LogoURL == "" {
		out.LogoURL = defaults.LogoURL
	}
	if out.PageTitle == "" {
		out.PageTitle = defaults.PageTitle
	}
	if out.TopbarText == "" {
		out.TopbarText = defaults.TopbarText
	}
	return out
}

func NormalizeDashUpdateChannel(channel DashUpdateChannel) (DashUpdateChannel, bool) {
	if strings.TrimSpace(string(channel)) == "" {
		return DashUpdateChannelRelease, true
	}
	return ParseDashUpdateChannel(channel)
}

func ParseDashUpdateChannel(channel DashUpdateChannel) (DashUpdateChannel, bool) {
	normalized, err := appversion.ParseChannel(string(channel))
	if err != nil {
		return DashUpdateChannelRelease, false
	}
	return normalized, true
}

func NormalizeDashUpdateMode(mode DashUpdateMode) (DashUpdateMode, bool) {
	if strings.TrimSpace(string(mode)) == "" {
		return DashUpdateModeManual, true
	}
	return ParseDashUpdateMode(mode)
}

func ParseDashUpdateMode(mode DashUpdateMode) (DashUpdateMode, bool) {
	switch DashUpdateMode(strings.TrimSpace(string(mode))) {
	case DashUpdateModeManual:
		return DashUpdateModeManual, true
	case DashUpdateModeNotify:
		return DashUpdateModeNotify, true
	case DashUpdateModeAuto:
		return DashUpdateModeAuto, true
	default:
		return DashUpdateModeManual, false
	}
}

func defaultSystemSetting() model.SystemSetting {
	brand := DefaultSiteBrand()
	return model.SystemSetting{
		ID:                systemSettingsID,
		DashUpdateChannel: string(DashUpdateChannelRelease),
		DashUpdateMode:    string(DashUpdateModeManual),
		LogoURL:           brand.LogoURL,
		PageTitle:         brand.PageTitle,
		TopbarText:        brand.TopbarText,
	}
}

func (s *Store) loadSettings(ctx context.Context) (model.SystemSetting, error) {
	var item model.SystemSetting
	err := s.db.WithContext(ctx).
		Where("id = ?", systemSettingsID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultSystemSetting(), nil
		}
		return model.SystemSetting{}, fmt.Errorf("load system settings: %w", err)
	}
	return item, nil
}

func (s *Store) saveSettingsColumns(ctx context.Context, item model.SystemSetting, columns []string) error {
	item.ID = systemSettingsID
	err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns(columns),
		}).
		Create(&item).Error
	if err != nil {
		return fmt.Errorf("save system settings: %w", err)
	}
	return nil
}

func (s *Store) GetActiveThemeID(ctx context.Context) (string, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return "", err
	}
	return item.ActiveThemeID, nil
}

func (s *Store) SetActiveThemeID(ctx context.Context, id string) error {
	item := defaultSystemSetting()
	item.ActiveThemeID = id
	return s.saveSettingsColumns(ctx, item, []string{"active_theme_id"})
}

func (s *Store) GetDashUpdateChannel(ctx context.Context) (DashUpdateChannel, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DashUpdateChannelRelease, err
	}
	channel, ok := NormalizeDashUpdateChannel(DashUpdateChannel(item.DashUpdateChannel))
	if !ok {
		return DashUpdateChannelRelease, nil
	}
	return channel, nil
}

func (s *Store) SetDashUpdateChannel(ctx context.Context, channel DashUpdateChannel) error {
	normalized, ok := ParseDashUpdateChannel(channel)
	if !ok {
		return fmt.Errorf("invalid dash update channel %q", channel)
	}
	item := defaultSystemSetting()
	item.DashUpdateChannel = string(normalized)
	return s.saveSettingsColumns(ctx, item, []string{"dash_update_channel"})
}

func (s *Store) GetDashUpdateMode(ctx context.Context) (DashUpdateMode, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DashUpdateModeManual, err
	}
	mode, ok := NormalizeDashUpdateMode(DashUpdateMode(item.DashUpdateMode))
	if !ok {
		return DashUpdateModeManual, nil
	}
	return mode, nil
}

func (s *Store) SetDashUpdateMode(ctx context.Context, mode DashUpdateMode) error {
	normalized, ok := ParseDashUpdateMode(mode)
	if !ok {
		return fmt.Errorf("invalid dash update mode %q", mode)
	}
	item := defaultSystemSetting()
	item.DashUpdateMode = string(normalized)
	return s.saveSettingsColumns(ctx, item, []string{"dash_update_mode"})
}

func (s *Store) GetSiteBrand(ctx context.Context) (SiteBrand, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DefaultSiteBrand(), err
	}
	return NormalizeSiteBrand(SiteBrand{
		LogoURL:    item.LogoURL,
		PageTitle:  item.PageTitle,
		TopbarText: item.TopbarText,
	}), nil
}

func (s *Store) SetSiteBrand(ctx context.Context, brand SiteBrand) error {
	item := defaultSystemSetting()
	normalized := NormalizeSiteBrand(brand)
	item.LogoURL = normalized.LogoURL
	item.PageTitle = normalized.PageTitle
	item.TopbarText = normalized.TopbarText
	return s.saveSettingsColumns(ctx, item, []string{
		"logo_url",
		"page_title",
		"topbar_text",
	})
}
