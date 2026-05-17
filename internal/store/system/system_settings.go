package system

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dash/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const systemSettingsID int16 = 1

const (
	DefaultSiteLogoURL    = "/brandlogo.svg"
	DefaultSitePageTitle  = "Ithiltir Monitor Dashboard"
	DefaultSiteTopbarText = "Ithiltir Control"
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

func defaultSystemSetting() model.SystemSetting {
	brand := DefaultSiteBrand()
	return model.SystemSetting{
		ID:         systemSettingsID,
		LogoURL:    brand.LogoURL,
		PageTitle:  brand.PageTitle,
		TopbarText: brand.TopbarText,
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
