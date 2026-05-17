package traffic

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
	"dash/internal/store/frontcache"
	trafficstore "dash/internal/store/traffic"
	"dash/internal/transport/http/request"
)

var errTrafficGuestForbidden = errors.New("traffic guest access denied")

type trafficSettingsView struct {
	GuestAccessMode   trafficstore.GuestAccessMode  `json:"guest_access_mode"`
	UsageMode         trafficstore.UsageMode        `json:"usage_mode"`
	CycleMode         trafficstore.BillingCycleMode `json:"cycle_mode"`
	BillingStartDay   int                           `json:"billing_start_day"`
	BillingAnchorDate string                        `json:"billing_anchor_date"`
	BillingTimezone   string                        `json:"billing_timezone"`
	DirectionMode     trafficstore.DirectionMode    `json:"direction_mode"`
}

type trafficSettingsInput struct {
	GuestAccessMode   *trafficstore.GuestAccessMode  `json:"guest_access_mode"`
	UsageMode         *trafficstore.UsageMode        `json:"usage_mode"`
	CycleMode         *trafficstore.BillingCycleMode `json:"cycle_mode"`
	BillingStartDay   *int                           `json:"billing_start_day"`
	BillingAnchorDate *string                        `json:"billing_anchor_date"`
	BillingTimezone   *string                        `json:"billing_timezone"`
	DirectionMode     *trafficstore.DirectionMode    `json:"direction_mode"`
}

type trafficQueryInput struct {
	ServerID          int64
	Iface             string
	UsageMode         trafficstore.UsageMode
	CycleMode         trafficstore.BillingCycleMode
	BillingStartDay   int
	BillingAnchorDate string
	BillingTimezone   string
	DirectionMode     trafficstore.DirectionMode
	Months            int
	Period            trafficstore.TrafficPeriod
}

func (h *handler) isAuthorized(r *http.Request) bool {
	return request.HasValidBearer(r, h.auth)
}

func (h *handler) canReadTraffic(ctx context.Context, r *http.Request, serverID int64) (bool, error) {
	if h.isAuthorized(r) {
		return true, nil
	}
	settings, err := h.traffic.GetSettings(ctx)
	if err != nil {
		return false, err
	}
	if settings.GuestAccessMode != trafficstore.GuestAccessByNode {
		return false, nil
	}
	return h.isGuestVisible(ctx, serverID)
}

func (h *handler) isGuestVisible(ctx context.Context, serverID int64) (bool, error) {
	if h.front == nil || serverID <= 0 {
		return false, nil
	}
	return h.front.EnsureGuestVisible(ctx, serverID, frontcache.GuestVisibilityOptions{
		CacheTimeout: config.RedisFetchTimeout,
		BuildTimeout: config.PGReadTimeout,
	})
}

func loadSettings(ctx context.Context, st *trafficstore.Store, loc *time.Location) (trafficstore.Settings, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (trafficstore.Settings, error) {
		settings, err := st.GetSettings(c)
		if err != nil {
			return settings, err
		}
		return trafficstore.SettingsWithTimezone(settings, loc), nil
	})
}

func loadP95Enabled(ctx context.Context, st *trafficstore.Store, serverID int64) (bool, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (bool, error) {
		return st.TrafficP95Enabled(c, serverID)
	})
}

func loadEffectiveCycleSettings(ctx context.Context, st *trafficstore.Store, serverID int64, defaults trafficstore.Settings) (trafficstore.Settings, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (trafficstore.Settings, error) {
		cycle, err := st.ServerCycleSettings(c, serverID)
		if err != nil {
			return trafficstore.Settings{}, err
		}
		return trafficstore.SettingsWithServerCycleSettings(defaults, cycle), nil
	})
}

func saveSettings(ctx context.Context, st *trafficstore.Store, settings trafficstore.Settings) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.SetSettings(c, settings)
	})
	return err
}

func settingsViewFrom(settings trafficstore.Settings) trafficSettingsView {
	normalized, _ := trafficstore.NormalizeSettings(settings)
	return trafficSettingsView{
		GuestAccessMode:   normalized.GuestAccessMode,
		UsageMode:         normalized.UsageMode,
		CycleMode:         normalized.CycleMode,
		BillingStartDay:   normalized.BillingStartDay,
		BillingAnchorDate: normalized.BillingAnchorDate,
		BillingTimezone:   normalized.BillingTimezone,
		DirectionMode:     normalized.DirectionMode,
	}
}

func (in trafficSettingsInput) hasFields() bool {
	return in.GuestAccessMode != nil ||
		in.UsageMode != nil ||
		in.CycleMode != nil ||
		in.BillingStartDay != nil ||
		in.BillingAnchorDate != nil ||
		in.BillingTimezone != nil ||
		in.DirectionMode != nil
}

func (in trafficSettingsInput) apply(current trafficstore.Settings) (trafficstore.Settings, bool) {
	next := current
	if in.GuestAccessMode != nil {
		next.GuestAccessMode = *in.GuestAccessMode
	}
	if in.UsageMode != nil {
		next.UsageMode = *in.UsageMode
	}
	if in.CycleMode != nil {
		next.CycleMode = *in.CycleMode
	}
	if in.BillingStartDay != nil {
		next.BillingStartDay = *in.BillingStartDay
	}
	if in.BillingAnchorDate != nil {
		next.BillingAnchorDate = strings.TrimSpace(*in.BillingAnchorDate)
	}
	if in.BillingTimezone != nil {
		next.BillingTimezone = strings.TrimSpace(*in.BillingTimezone)
	}
	if in.DirectionMode != nil {
		next.DirectionMode = *in.DirectionMode
	}
	return trafficstore.NormalizeSettings(next)
}

func parseTrafficQuery(q url.Values, defaults trafficstore.Settings) (trafficQueryInput, error) {
	serverID, err := parseServerID(q)
	if err != nil {
		return trafficQueryInput{}, err
	}

	months := 6
	if raw := strings.TrimSpace(q.Get("months")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return trafficQueryInput{}, errors.New("invalid months")
		}
		if n > 24 {
			n = 24
		}
		months = n
	}

	iface, err := parseIface(q.Get("iface"))
	if err != nil {
		return trafficQueryInput{}, err
	}
	return trafficQueryInput{
		ServerID:          serverID,
		Iface:             iface,
		UsageMode:         defaults.UsageMode,
		CycleMode:         defaults.CycleMode,
		BillingStartDay:   defaults.BillingStartDay,
		BillingAnchorDate: defaults.BillingAnchorDate,
		BillingTimezone:   defaults.BillingTimezone,
		DirectionMode:     defaults.DirectionMode,
		Months:            months,
	}, nil
}

func (in *trafficQueryInput) applySettings(settings trafficstore.Settings) {
	in.UsageMode = settings.UsageMode
	in.CycleMode = settings.CycleMode
	in.BillingStartDay = settings.BillingStartDay
	in.BillingAnchorDate = settings.BillingAnchorDate
	in.BillingTimezone = settings.BillingTimezone
	in.DirectionMode = settings.DirectionMode
}

func parseServerID(q url.Values) (int64, error) {
	raw := strings.TrimSpace(q.Get("server_id"))
	if raw == "" {
		return 0, errors.New("server_id is required")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err == nil && id > 0 {
		return id, nil
	}
	return 0, errors.New("invalid server_id")
}

func parseIface(raw string) (string, error) {
	iface := strings.TrimSpace(raw)
	if iface == "" {
		return "", errors.New("iface is required")
	}
	if strings.EqualFold(iface, "all") {
		return "", errors.New("invalid iface")
	}
	return iface, nil
}

func parseTrafficPeriod(raw string) (trafficstore.TrafficPeriod, error) {
	switch strings.TrimSpace(raw) {
	case "", string(trafficstore.TrafficPeriodCurrent):
		return trafficstore.TrafficPeriodCurrent, nil
	case string(trafficstore.TrafficPeriodPrev):
		return trafficstore.TrafficPeriodPrev, nil
	default:
		return "", errors.New("invalid period")
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func queryFromInput(in trafficQueryInput, loc *time.Location) trafficstore.TrafficQuery {
	return trafficstore.TrafficQuery{
		ServerID:          in.ServerID,
		Iface:             in.Iface,
		UsageMode:         in.UsageMode,
		CycleMode:         in.CycleMode,
		BillingStartDay:   in.BillingStartDay,
		BillingAnchorDate: in.BillingAnchorDate,
		DirectionMode:     in.DirectionMode,
		Location:          trafficstore.SettingsLocation(trafficstore.Settings{BillingTimezone: in.BillingTimezone}, loc),
		Ref:               time.Now(),
		Period:            in.Period,
	}
}
