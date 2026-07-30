package system

import (
	"context"
	"fmt"
	"strings"

	"dash/internal/model"
	appversion "dash/internal/version"
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
	return s.saveSettings(ctx, map[string]any{"active_theme_id": id})
}

// GetDashUpdatePolicy reads the coupled update channel and mode from one
// committed system_settings row.
func (s *Store) GetDashUpdatePolicy(ctx context.Context) (DashUpdatePolicy, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DashUpdatePolicy{}, err
	}
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

// GetDashUpdateChannel keeps the pre-policy reader available while the update
// service is migrated in the following commit.
func (s *Store) GetDashUpdateChannel(ctx context.Context) (DashUpdateChannel, error) {
	policy, err := s.GetDashUpdatePolicy(ctx)
	return policy.Channel, err
}

// GetDashUpdateMode keeps the pre-policy reader available while the update
// service is migrated in the following commit.
func (s *Store) GetDashUpdateMode(ctx context.Context) (DashUpdateMode, error) {
	policy, err := s.GetDashUpdatePolicy(ctx)
	return policy.Mode, err
}

func (s *Store) SetDashUpdateChannel(ctx context.Context, channel DashUpdateChannel) error {
	normalized, ok := ParseDashUpdateChannel(channel)
	if !ok {
		return fmt.Errorf("invalid dash update channel %q", channel)
	}
	return s.saveSettings(ctx, map[string]any{"dash_update_channel": string(normalized)})
}

func (s *Store) SetDashUpdateMode(ctx context.Context, mode DashUpdateMode) error {
	normalized, ok := ParseDashUpdateMode(mode)
	if !ok {
		return fmt.Errorf("invalid dash update mode %q", mode)
	}
	return s.saveSettings(ctx, map[string]any{"dash_update_mode": string(normalized)})
}

func (s *Store) GetSiteBrand(ctx context.Context) (SiteBrand, error) {
	item, err := s.loadSettings(ctx)
	if err != nil {
		return DefaultSiteBrand(), err
	}
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

func (s *Store) PatchSiteBrand(ctx context.Context, patch SiteBrandPatch) error {
	updates := make(map[string]any, 3)
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
	return s.saveSettings(ctx, updates)
}

func normalizeSiteBrandField(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
