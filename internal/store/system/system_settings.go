package system

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dash/internal/model"
	appversion "dash/internal/version"
	"github.com/jackc/pgx/v5/pgconn"
)

const systemSettingsID int16 = 1

const (
	DefaultSiteLogoURL    = "/brandlogo.svg"
	DefaultSitePageTitle  = "Ithiltir Monitor Dashboard"
	DefaultSiteTopbarText = "Ithiltir Control"
)

type DashUpdateChannel = appversion.Channel
type DashUpdateMode string

type DashUpdatePolicy struct {
	Channel DashUpdateChannel
	Mode    DashUpdateMode
}

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

// SiteBrandPatch preserves PATCH tri-state semantics: nil leaves a field
// unchanged, while an empty string resets that field to its default.
type SiteBrandPatch struct {
	LogoURL    *string
	PageTitle  *string
	TopbarText *string
}

type Settings struct {
	SiteBrand
	ActiveThemeID      string
	DashUpdateChannel  DashUpdateChannel
	DashUpdateMode     DashUpdateMode
	UptimeGuestVisible bool
	UptimeWarningSLA   float64
	UptimeErrorSLA     float64
}

// SettingsPatch updates only non-nil fields, including explicit false and zero.
type SettingsPatch struct {
	SiteBrandPatch
	ActiveThemeID      *string
	DashUpdateChannel  *DashUpdateChannel
	DashUpdateMode     *DashUpdateMode
	UptimeGuestVisible *bool
	UptimeWarningSLA   *float64
	UptimeErrorSLA     *float64
}

var ErrInvalidUptimeSLA = errors.New("uptime SLA must satisfy 0 <= error < warning <= 100")

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

func ParseDashUpdateChannel(channel DashUpdateChannel) (DashUpdateChannel, bool) {
	normalized, err := appversion.ParseChannel(string(channel))
	if err != nil {
		return DashUpdateChannelRelease, false
	}
	return normalized, true
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

func (s *Store) loadSettings(ctx context.Context) (model.SystemSetting, error) {
	var item model.SystemSetting
	err := s.db.WithContext(ctx).
		Where("id = ?", systemSettingsID).
		First(&item).Error
	if err != nil {
		return model.SystemSetting{}, fmt.Errorf("load system settings: %w", err)
	}
	return item, nil
}

func (s *Store) saveSettings(ctx context.Context, updates map[string]any) error {
	result := s.db.WithContext(ctx).
		Model(&model.SystemSetting{}).
		Where("id = ?", systemSettingsID).
		Updates(updates)
	if result.Error != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](result.Error); ok && pgErr.ConstraintName == "system_settings_uptime_sla" {
			return ErrInvalidUptimeSLA
		}
		return fmt.Errorf("save system settings: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("save system settings: singleton row is missing")
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
	return s.PatchSettings(ctx, SettingsPatch{ActiveThemeID: &id})
}

// GetDashUpdatePolicy reads the coupled update channel and mode from one
// committed system_settings row.
func (s *Store) GetDashUpdatePolicy(ctx context.Context) (DashUpdatePolicy, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DashUpdatePolicy{}, err
	}
	return dashUpdatePolicy(item)
}

func dashUpdatePolicy(item model.SystemSetting) (DashUpdatePolicy, error) {
	channel, ok := ParseDashUpdateChannel(DashUpdateChannel(item.DashUpdateChannel))
	if !ok {
		return DashUpdatePolicy{}, fmt.Errorf("invalid dash update channel %q", item.DashUpdateChannel)
	}
	mode, ok := ParseDashUpdateMode(DashUpdateMode(item.DashUpdateMode))
	if !ok {
		return DashUpdatePolicy{}, fmt.Errorf("invalid dash update mode %q", item.DashUpdateMode)
	}
	return DashUpdatePolicy{Channel: channel, Mode: mode}, nil
}

func (s *Store) GetSiteBrand(ctx context.Context) (SiteBrand, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DefaultSiteBrand(), err
	}
	return siteBrand(item)
}

func siteBrand(item model.SystemSetting) (SiteBrand, error) {
	stored := SiteBrand{
		LogoURL:    item.LogoURL,
		PageTitle:  item.PageTitle,
		TopbarText: item.TopbarText,
	}
	brand := NormalizeSiteBrand(stored)
	if brand != stored {
		return SiteBrand{}, fmt.Errorf("invalid stored site brand")
	}
	return brand, nil
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	brand, err := siteBrand(item)
	if err != nil {
		return Settings{}, err
	}
	policy, err := dashUpdatePolicy(item)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		SiteBrand:          brand,
		ActiveThemeID:      item.ActiveThemeID,
		DashUpdateChannel:  policy.Channel,
		DashUpdateMode:     policy.Mode,
		UptimeGuestVisible: item.UptimeGuestVisible,
		UptimeWarningSLA:   item.UptimeWarningSLA,
		UptimeErrorSLA:     item.UptimeErrorSLA,
	}, nil
}

func (s *Store) PatchSettings(ctx context.Context, patch SettingsPatch) error {
	updates := make(map[string]any, 9)
	if patch.ActiveThemeID != nil {
		updates["active_theme_id"] = *patch.ActiveThemeID
	}
	if patch.DashUpdateChannel != nil {
		normalized, ok := ParseDashUpdateChannel(*patch.DashUpdateChannel)
		if !ok {
			return fmt.Errorf("invalid dash update channel %q", *patch.DashUpdateChannel)
		}
		updates["dash_update_channel"] = string(normalized)
	}
	if patch.DashUpdateMode != nil {
		normalized, ok := ParseDashUpdateMode(*patch.DashUpdateMode)
		if !ok {
			return fmt.Errorf("invalid dash update mode %q", *patch.DashUpdateMode)
		}
		updates["dash_update_mode"] = string(normalized)
	}
	if patch.UptimeGuestVisible != nil {
		updates["uptime_guest_visible"] = *patch.UptimeGuestVisible
	}
	if patch.UptimeWarningSLA != nil {
		updates["uptime_warning_sla"] = *patch.UptimeWarningSLA
	}
	if patch.UptimeErrorSLA != nil {
		updates["uptime_error_sla"] = *patch.UptimeErrorSLA
	}
	defaults := DefaultSiteBrand()
	if patch.LogoURL != nil {
		updates["logo_url"] = normalizeSiteBrandField(*patch.LogoURL, defaults.LogoURL)
	}
	if patch.PageTitle != nil {
		updates["page_title"] = normalizeSiteBrandField(*patch.PageTitle, defaults.PageTitle)
	}
	if patch.TopbarText != nil {
		updates["topbar_text"] = normalizeSiteBrandField(*patch.TopbarText, defaults.TopbarText)
	}
	if len(updates) == 0 {
		return nil
	}
	// One UPDATE preserves concurrent edits to other fields and lets the database
	// check coupled SLA thresholds against the current row atomically.
	return s.saveSettings(ctx, updates)
}

func normalizeSiteBrandField(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
