package nodeid

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"dash/internal/infra"
	"dash/internal/nodetags"
	nodestore "dash/internal/store/node"
	trafficstore "dash/internal/store/traffic"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type updateInput struct {
	Name                     *string                           `json:"name"`
	IsGuestVisible           *bool                             `json:"is_guest_visible"`
	TrafficP95Enabled        *bool                             `json:"traffic_p95_enabled"`
	TrafficCycleMode         *trafficstore.ServerCycleMode     `json:"traffic_cycle_mode"`
	TrafficBillingStartDay   *int                              `json:"traffic_billing_start_day"`
	TrafficBillingAnchorDate *string                           `json:"traffic_billing_anchor_date"`
	TrafficBillingTimezone   *string                           `json:"traffic_billing_timezone"`
	TrafficDirectionMode     *trafficstore.ServerDirectionMode `json:"traffic_direction_mode"`
	DisplayOrder             *int                              `json:"display_order"`
	Tags                     json.RawMessage                   `json:"tags"`
	Secret                   *string                           `json:"secret"`
	GroupIDs                 *[]int64                          `json:"group_ids"`
}

func updateRoute(r *routes.Blueprint, h *handler) {
	r.Patch(
		"/",
		"Update node",
		routes.Func(h.updateHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) updateHandler(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	var in updateInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}
	if err := normalizeUpdate(&in); err != nil {
		switch {
		case errors.Is(err, errInvalidNodeName):
			httperr.Write(w, http.StatusBadRequest, "invalid_name", err.Error())
		case errors.Is(err, errInvalidDisplayOrder):
			httperr.Write(w, http.StatusBadRequest, "invalid_display_order", err.Error())
		case errors.Is(err, errInvalidTrafficCycleMode):
			httperr.Write(w, http.StatusBadRequest, "invalid_traffic_cycle_mode", err.Error())
		case errors.Is(err, errIncompleteTrafficCycleSettings):
			httperr.Write(w, http.StatusBadRequest, "invalid_traffic_cycle_settings", err.Error())
		case errors.Is(err, errInvalidTrafficBillingStartDay):
			httperr.Write(w, http.StatusBadRequest, "invalid_traffic_billing_start_day", err.Error())
		case errors.Is(err, errInvalidTrafficBillingAnchor):
			httperr.Write(w, http.StatusBadRequest, "invalid_traffic_billing_anchor_date", err.Error())
		case errors.Is(err, errInvalidTrafficBillingTimezone):
			httperr.Write(w, http.StatusBadRequest, "invalid_traffic_billing_timezone", err.Error())
		case errors.Is(err, errInvalidTrafficDirectionMode):
			httperr.Write(w, http.StatusBadRequest, "invalid_traffic_direction_mode", err.Error())
		case errors.Is(err, nodetags.ErrInvalid):
			httperr.Write(w, http.StatusBadRequest, "invalid_tags", err.Error())
		default:
			httperr.Write(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}
	if in.GroupIDs != nil {
		normalized, hadDup, err := normalizeGroupIDs(*in.GroupIDs)
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, "invalid_group_ids", err.Error())
			return
		}
		if hadDup {
			infra.WithModule("admin.nodes").Warn("duplicate group ids removed", nil,
				slog.Int64("node_id", id),
				slog.Int("before", len(*in.GroupIDs)),
				slog.Int("after", len(normalized)),
			)
		}
		in.GroupIDs = &normalized
	}

	upd := updateFromInput(in)
	if !hasNodeUpdates(upd) {
		httperr.Write(w, http.StatusBadRequest, "no_fields", "no fields to update")
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.UpdateNode(c, id, upd)
	}); err != nil {
		if errors.Is(err, nodestore.ErrServerMetaCacheUpdate) {
			infra.WithModule("admin.nodes").Error("server cache sync failed after update", err,
				slog.Int64("node_id", id),
			)
			httperr.Write(w, http.StatusServiceUnavailable, "redis_cache_error", "sync failed")
			return
		} else if errors.Is(err, nodestore.ErrFrontCacheUpdate) {
			infra.WithModule("admin.nodes").Warn("front cache sync failed after node update", err,
				slog.Int64("node_id", id),
			)
			httperr.Write(w, http.StatusServiceUnavailable, "redis_cache_error", "sync failed")
			return
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "node not found")
			return
		} else if errors.Is(err, nodestore.ErrInvalidSecret) {
			httperr.Write(w, http.StatusBadRequest, "invalid_secret", "invalid secret")
			return
		} else if errors.Is(err, nodestore.ErrInvalidGroupIDs) {
			httperr.Write(w, http.StatusBadRequest, "invalid_group_ids", "invalid group ids")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update node")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var (
	errInvalidNodeName                = errors.New("name cannot be empty")
	errInvalidDisplayOrder            = errors.New("display_order must be positive")
	errInvalidTrafficCycleMode        = errors.New("traffic_cycle_mode is invalid")
	errIncompleteTrafficCycleSettings = errors.New("traffic cycle fields do not match mode")
	errInvalidTrafficBillingStartDay  = errors.New("traffic_billing_start_day must be between 1 and 31")
	errInvalidTrafficBillingAnchor    = errors.New("traffic_billing_anchor_date is invalid")
	errInvalidTrafficBillingTimezone  = errors.New("traffic_billing_timezone is invalid")
	errInvalidTrafficDirectionMode    = errors.New("traffic_direction_mode is invalid")
)

func normalizeUpdate(in *updateInput) error {
	if in == nil {
		return errors.New("request is required")
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return errInvalidNodeName
		}
		in.Name = &name
	}
	if in.DisplayOrder != nil && *in.DisplayOrder <= 0 {
		return errInvalidDisplayOrder
	}
	if err := normalizeCycleUpdate(in); err != nil {
		return err
	}
	if err := normalizeDirectionUpdate(in); err != nil {
		return err
	}
	if in.Secret != nil {
		secret := strings.TrimSpace(*in.Secret)
		in.Secret = &secret
	}
	if in.Tags != nil {
		tags, err := normalizeSubmittedTags(in.Tags)
		if err != nil {
			return err
		}
		in.Tags = json.RawMessage(tags)
	}
	return nil
}

func normalizeCycleUpdate(in *updateInput) error {
	if !hasCycleUpdate(in) {
		return nil
	}
	if in.TrafficCycleMode == nil {
		return errIncompleteTrafficCycleSettings
	}
	mode, ok := trafficstore.NormalizeServerCycleMode(*in.TrafficCycleMode)
	if !ok {
		return errInvalidTrafficCycleMode
	}
	cycle, err := cycleSettingsFromInput(mode, in)
	if err != nil {
		if errors.Is(err, errIncompleteTrafficCycleSettings) {
			return err
		}
		return nodeCycleError(err)
	}
	in.TrafficCycleMode = &cycle.Mode
	in.TrafficBillingStartDay = &cycle.BillingStartDay
	in.TrafficBillingAnchorDate = &cycle.BillingAnchorDate
	in.TrafficBillingTimezone = &cycle.BillingTimezone
	return nil
}

func hasCycleUpdate(in *updateInput) bool {
	return in.TrafficCycleMode != nil ||
		in.TrafficBillingStartDay != nil ||
		in.TrafficBillingAnchorDate != nil ||
		in.TrafficBillingTimezone != nil
}

func cycleSettingsFromInput(mode trafficstore.ServerCycleMode, in *updateInput) (trafficstore.ServerCycleSettings, error) {
	cycle := trafficstore.ServerCycleSettings{Mode: mode, BillingStartDay: 1}
	switch mode {
	case trafficstore.ServerCycleDefault:
		if in.TrafficBillingStartDay != nil || in.TrafficBillingAnchorDate != nil || in.TrafficBillingTimezone != nil {
			return cycle, errIncompleteTrafficCycleSettings
		}
	case trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth):
		if in.TrafficBillingStartDay != nil || in.TrafficBillingAnchorDate != nil || in.TrafficBillingTimezone == nil {
			return cycle, errIncompleteTrafficCycleSettings
		}
		cycle.BillingTimezone = *in.TrafficBillingTimezone
	case trafficstore.ServerCycleMode(trafficstore.CycleClampMonthEnd):
		if in.TrafficBillingStartDay == nil || in.TrafficBillingAnchorDate != nil || in.TrafficBillingTimezone == nil {
			return cycle, errIncompleteTrafficCycleSettings
		}
		cycle.BillingStartDay = *in.TrafficBillingStartDay
		cycle.BillingTimezone = *in.TrafficBillingTimezone
	case trafficstore.ServerCycleMode(trafficstore.CycleWHMCS):
		if in.TrafficBillingStartDay != nil || in.TrafficBillingAnchorDate == nil || in.TrafficBillingTimezone == nil {
			return cycle, errIncompleteTrafficCycleSettings
		}
		cycle.BillingAnchorDate = *in.TrafficBillingAnchorDate
		cycle.BillingTimezone = *in.TrafficBillingTimezone
	default:
		return cycle, errInvalidTrafficCycleMode
	}
	return trafficstore.NormalizeServerCycleSettings(cycle)
}

func normalizeDirectionUpdate(in *updateInput) error {
	if in.TrafficDirectionMode == nil {
		return nil
	}
	mode, ok := trafficstore.NormalizeServerDirectionMode(*in.TrafficDirectionMode)
	if !ok {
		return errInvalidTrafficDirectionMode
	}
	in.TrafficDirectionMode = &mode
	return nil
}

func nodeCycleError(err error) error {
	switch {
	case errors.Is(err, trafficstore.ErrInvalidServerCycleMode):
		return errInvalidTrafficCycleMode
	case errors.Is(err, trafficstore.ErrInvalidServerCycleStartDay):
		return errInvalidTrafficBillingStartDay
	case errors.Is(err, trafficstore.ErrInvalidServerCycleAnchorDate):
		return errInvalidTrafficBillingAnchor
	case errors.Is(err, trafficstore.ErrInvalidServerCycleTimezone):
		return errInvalidTrafficBillingTimezone
	default:
		return errInvalidTrafficCycleMode
	}
}

func updateFromInput(in updateInput) nodestore.NodeUpdate {
	upd := nodestore.NodeUpdate{
		Name:                     in.Name,
		IsGuestVisible:           in.IsGuestVisible,
		TrafficP95Enabled:        in.TrafficP95Enabled,
		TrafficCycleMode:         in.TrafficCycleMode,
		TrafficBillingStartDay:   in.TrafficBillingStartDay,
		TrafficBillingAnchorDate: in.TrafficBillingAnchorDate,
		TrafficBillingTimezone:   in.TrafficBillingTimezone,
		TrafficDirectionMode:     in.TrafficDirectionMode,
		DisplayOrder:             in.DisplayOrder,
		Secret:                   in.Secret,
		GroupIDs:                 in.GroupIDs,
	}
	if in.Tags != nil {
		tags := datatypes.JSON(in.Tags)
		upd.Tags = &tags
	}
	return upd
}

func hasNodeUpdates(upd nodestore.NodeUpdate) bool {
	return upd.Name != nil ||
		upd.IsGuestVisible != nil ||
		upd.TrafficP95Enabled != nil ||
		upd.TrafficCycleMode != nil ||
		upd.TrafficBillingStartDay != nil ||
		upd.TrafficBillingAnchorDate != nil ||
		upd.TrafficBillingTimezone != nil ||
		upd.TrafficDirectionMode != nil ||
		upd.DisplayOrder != nil ||
		upd.Tags != nil ||
		upd.Secret != nil ||
		upd.GroupIDs != nil
}

func normalizeGroupIDs(ids []int64) ([]int64, bool, error) {
	if len(ids) == 0 {
		return ids, false, nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	hadDup := false
	for _, id := range ids {
		if id <= 0 {
			return nil, false, errors.New("group id must be positive")
		}
		if _, ok := seen[id]; ok {
			hadDup = true
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, hadDup, nil
}
