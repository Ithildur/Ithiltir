package settings

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"dash/internal/infra"
	"dash/internal/store/metricdata"
	systemstore "dash/internal/store/system"
)

type settingsView struct {
	HistoryGuestAccessMode metricdata.HistoryGuestAccessMode `json:"history_guest_access_mode"`
	LogoURL                string                            `json:"logo_url"`
	PageTitle              string                            `json:"page_title"`
	TopbarText             string                            `json:"topbar_text"`
}

type settingsInput struct {
	HistoryGuestAccessMode *metricdata.HistoryGuestAccessMode `json:"history_guest_access_mode"`
	LogoURL                *string                            `json:"logo_url"`
	PageTitle              *string                            `json:"page_title"`
	TopbarText             *string                            `json:"topbar_text"`
}

func loadSettings(ctx context.Context, metric *metricdata.Store, system *systemstore.Store) (settingsView, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (settingsView, error) {
		mode, err := metric.GetHistoryGuestAccessMode(c)
		if err != nil {
			return settingsView{}, err
		}
		brand, err := system.GetSiteBrand(c)
		if err != nil {
			return settingsView{}, err
		}
		return settingsViewFrom(mode, brand), nil
	})
}

func loadSiteBrand(ctx context.Context, st *systemstore.Store) (systemstore.SiteBrand, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (systemstore.SiteBrand, error) {
		return st.GetSiteBrand(c)
	})
}

func saveSettings(
	ctx context.Context,
	metric *metricdata.Store,
	system *systemstore.Store,
	mode *metricdata.HistoryGuestAccessMode,
	brand *systemstore.SiteBrand,
) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		if mode != nil {
			if err := metric.SetHistoryGuestAccessMode(c, *mode); err != nil {
				return struct{}{}, err
			}
		}
		if brand != nil {
			if err := system.SetSiteBrand(c, *brand); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	return err
}

func settingsViewFrom(mode metricdata.HistoryGuestAccessMode, brand systemstore.SiteBrand) settingsView {
	normalized := systemstore.NormalizeSiteBrand(brand)
	return settingsView{
		HistoryGuestAccessMode: mode,
		LogoURL:                normalized.LogoURL,
		PageTitle:              normalized.PageTitle,
		TopbarText:             normalized.TopbarText,
	}
}

func (in settingsInput) hasSiteBrandFields() bool {
	return in.LogoURL != nil || in.PageTitle != nil || in.TopbarText != nil
}

func (in settingsInput) applySiteBrand(current systemstore.SiteBrand) (systemstore.SiteBrand, error) {
	next := current
	if in.LogoURL != nil {
		next.LogoURL = *in.LogoURL
	}
	if in.PageTitle != nil {
		next.PageTitle = *in.PageTitle
	}
	if in.TopbarText != nil {
		next.TopbarText = *in.TopbarText
	}
	return validateSiteBrand(next)
}

const (
	maxPageTitleRunes  = 120
	maxTopbarTextRunes = 64
	maxLogoURLBytes    = 768 * 1024
)

var errInvalidSiteBrand = errors.New("invalid site brand")

func validateSiteBrand(brand systemstore.SiteBrand) (systemstore.SiteBrand, error) {
	normalized := systemstore.NormalizeSiteBrand(brand)
	if utf8.RuneCountInString(normalized.PageTitle) > maxPageTitleRunes {
		return systemstore.SiteBrand{}, errInvalidSiteBrand
	}
	if utf8.RuneCountInString(normalized.TopbarText) > maxTopbarTextRunes {
		return systemstore.SiteBrand{}, errInvalidSiteBrand
	}
	if len(normalized.LogoURL) > maxLogoURLBytes || !validLogoURL(normalized.LogoURL) {
		return systemstore.SiteBrand{}, errInvalidSiteBrand
	}
	return normalized, nil
}

func validLogoURL(value string) bool {
	if value == systemstore.DefaultSiteLogoURL {
		return true
	}
	if strings.HasPrefix(value, "/") {
		return !strings.HasPrefix(value, "//")
	}

	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return true
	}
	return strings.HasPrefix(lower, "data:image/") && strings.Contains(lower, ";base64,")
}
