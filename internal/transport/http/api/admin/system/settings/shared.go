package settings

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"dash/internal/infra"
	"dash/internal/store/metricdata"
	systemstore "dash/internal/store/system"
)

type settingsView struct {
	HistoryGuestAccessMode metricdata.HistoryGuestAccessMode `json:"history_guest_access_mode"`
	DashUpdateChannel      systemstore.DashUpdateChannel     `json:"dash_update_channel"`
	DashUpdateMode         systemstore.DashUpdateMode        `json:"dash_update_mode"`
	LogoURL                string                            `json:"logo_url"`
	PageTitle              string                            `json:"page_title"`
	TopbarText             string                            `json:"topbar_text"`
}

type settingsInput struct {
	HistoryGuestAccessMode *metricdata.HistoryGuestAccessMode `json:"history_guest_access_mode"`
	DashUpdateChannel      *systemstore.DashUpdateChannel     `json:"dash_update_channel"`
	DashUpdateMode         *systemstore.DashUpdateMode        `json:"dash_update_mode"`
	LogoURL                *string                            `json:"logo_url"`
	PageTitle              *string                            `json:"page_title"`
	TopbarText             *string                            `json:"topbar_text"`
}

type settingsTx interface {
	WithSettingsTx(context.Context, func(*metricdata.Store, *systemstore.Store) error) error
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
		policy, err := system.GetDashUpdatePolicy(c)
		if err != nil {
			return settingsView{}, err
		}
		return settingsViewFrom(mode, policy.Channel, policy.Mode, brand), nil
	})
}

func saveSettingsPatch(
	ctx context.Context,
	tx settingsTx,
	mode *metricdata.HistoryGuestAccessMode,
	channel *systemstore.DashUpdateChannel,
	updateMode *systemstore.DashUpdateMode,
	brand *systemstore.SiteBrandPatch,
) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, tx.WithSettingsTx(c, func(metric *metricdata.Store, system *systemstore.Store) error {
			if mode != nil {
				if err := metric.SetHistoryGuestAccessMode(c, *mode); err != nil {
					return err
				}
			}
			if channel != nil {
				if err := system.SetDashUpdateChannel(c, *channel); err != nil {
					return err
				}
			}
			if updateMode != nil {
				if err := system.SetDashUpdateMode(c, *updateMode); err != nil {
					return err
				}
			}
			if brand != nil {
				if err := system.PatchSiteBrand(c, *brand); err != nil {
					return err
				}
			}
			return nil
		})
	})
	return err
}

func saveSettingsDoc(ctx context.Context, tx settingsTx, doc settingsView) error {
	brand := fullSiteBrandPatch(doc.siteBrand())
	return saveSettingsPatch(
		ctx,
		tx,
		&doc.HistoryGuestAccessMode,
		&doc.DashUpdateChannel,
		&doc.DashUpdateMode,
		&brand,
	)
}

func settingsViewFrom(
	mode metricdata.HistoryGuestAccessMode,
	channel systemstore.DashUpdateChannel,
	updateMode systemstore.DashUpdateMode,
	brand systemstore.SiteBrand,
) settingsView {
	normalized := systemstore.NormalizeSiteBrand(brand)
	return settingsView{
		HistoryGuestAccessMode: mode,
		DashUpdateChannel:      channel,
		DashUpdateMode:         updateMode,
		LogoURL:                normalized.LogoURL,
		PageTitle:              normalized.PageTitle,
		TopbarText:             normalized.TopbarText,
	}
}

func (v settingsView) siteBrand() systemstore.SiteBrand {
	return systemstore.SiteBrand{
		LogoURL:    v.LogoURL,
		PageTitle:  v.PageTitle,
		TopbarText: v.TopbarText,
	}
}

func fullSiteBrandPatch(brand systemstore.SiteBrand) systemstore.SiteBrandPatch {
	return systemstore.SiteBrandPatch{
		LogoURL:    &brand.LogoURL,
		PageTitle:  &brand.PageTitle,
		TopbarText: &brand.TopbarText,
	}
}

func (in settingsInput) hasSiteBrandFields() bool {
	return in.LogoURL != nil || in.PageTitle != nil || in.TopbarText != nil
}

func (in settingsInput) siteBrandPatch() (systemstore.SiteBrandPatch, error) {
	var patch systemstore.SiteBrandPatch
	if in.LogoURL != nil {
		value, err := normalizeLogoURL(*in.LogoURL)
		if err != nil {
			return systemstore.SiteBrandPatch{}, err
		}
		patch.LogoURL = &value
	}
	if in.PageTitle != nil {
		value, err := normalizePageTitle(*in.PageTitle)
		if err != nil {
			return systemstore.SiteBrandPatch{}, err
		}
		patch.PageTitle = &value
	}
	if in.TopbarText != nil {
		value, err := normalizeTopbarText(*in.TopbarText)
		if err != nil {
			return systemstore.SiteBrandPatch{}, err
		}
		patch.TopbarText = &value
	}
	return patch, nil
}

func (in settingsInput) requiredSiteBrand() (systemstore.SiteBrand, error) {
	if in.LogoURL == nil || in.PageTitle == nil || in.TopbarText == nil {
		return systemstore.SiteBrand{}, errInvalidSiteBrand
	}
	return validateSiteBrand(systemstore.SiteBrand{
		LogoURL:    *in.LogoURL,
		PageTitle:  *in.PageTitle,
		TopbarText: *in.TopbarText,
	})
}

const (
	maxPageTitleRunes  = 120
	maxTopbarTextRunes = 64
	maxLogoURLBytes    = 768 * 1024
)

var errInvalidSiteBrand = errors.New("invalid site brand")

func validateSiteBrand(brand systemstore.SiteBrand) (systemstore.SiteBrand, error) {
	logoURL, err := normalizeLogoURL(brand.LogoURL)
	if err != nil {
		return systemstore.SiteBrand{}, err
	}
	pageTitle, err := normalizePageTitle(brand.PageTitle)
	if err != nil {
		return systemstore.SiteBrand{}, err
	}
	topbarText, err := normalizeTopbarText(brand.TopbarText)
	if err != nil {
		return systemstore.SiteBrand{}, err
	}
	return systemstore.SiteBrand{LogoURL: logoURL, PageTitle: pageTitle, TopbarText: topbarText}, nil
}

func normalizeLogoURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = systemstore.DefaultSiteLogoURL
	}
	if len(value) > maxLogoURLBytes || !validLogoURL(value) {
		return "", errInvalidSiteBrand
	}
	return value, nil
}

func normalizePageTitle(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = systemstore.DefaultSitePageTitle
	}
	if utf8.RuneCountInString(value) > maxPageTitleRunes {
		return "", errInvalidSiteBrand
	}
	return value, nil
}

func normalizeTopbarText(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = systemstore.DefaultSiteTopbarText
	}
	if utf8.RuneCountInString(value) > maxTopbarTextRunes {
		return "", errInvalidSiteBrand
	}
	return value, nil
}

func validLogoURL(value string) bool {
	if value == systemstore.DefaultSiteLogoURL {
		return true
	}
	if strings.HasPrefix(value, "/") {
		if strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\\r\n\t") {
			return false
		}
		parsed, err := url.ParseRequestURI(value)
		return err == nil && parsed.Host == "" && !parsed.IsAbs()
	}

	parsed, err := url.ParseRequestURI(value)
	if err == nil && parsed.Scheme == "https" {
		return parsed.Hostname() != "" && parsed.User == nil
	}

	media, payload, ok := strings.Cut(value, ",")
	if !ok {
		return false
	}
	switch strings.ToLower(media) {
	case "data:image/svg+xml;base64",
		"data:image/png;base64",
		"data:image/jpeg;base64",
		"data:image/gif;base64",
		"data:image/webp;base64",
		"data:image/ico;base64",
		"data:image/x-icon;base64",
		"data:image/vnd.microsoft.icon;base64":
		_, err := base64.StdEncoding.Strict().DecodeString(payload)
		return err == nil
	default:
		return false
	}
}
