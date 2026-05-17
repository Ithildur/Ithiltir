package rules

import (
	"context"
	"net/http"
	"time"

	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type ruleView struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Enabled         bool    `json:"enabled"`
	Metric          string  `json:"metric"`
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	DurationSec     int32   `json:"duration_sec"`
	CooldownMin     int32   `json:"cooldown_min"`
	ThresholdMode   string  `json:"threshold_mode"`
	ThresholdOffset float64 `json:"threshold_offset"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List alert rules",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]alertstore.AlertRuleItem, error) {
		return h.store.ListRules(c)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch rules")
		return
	}

	response.WriteJSON(w, http.StatusOK, buildList(items))
}

func buildList(items []alertstore.AlertRuleItem) []ruleView {
	if len(items) == 0 {
		return make([]ruleView, 0)
	}
	out := make([]ruleView, 0, len(items))
	for _, item := range items {
		out = append(out, ruleView{
			ID:              item.ID,
			Name:            item.Name,
			Enabled:         item.Enabled,
			Metric:          item.Metric,
			Operator:        item.Operator,
			Threshold:       item.Threshold,
			DurationSec:     item.DurationSec,
			CooldownMin:     item.CooldownMin,
			ThresholdMode:   item.ThresholdMode,
			ThresholdOffset: item.ThresholdOffset,
			CreatedAt:       item.CreatedAt.Format(time.RFC3339),
			UpdatedAt:       item.UpdatedAt.Format(time.RFC3339),
		})
	}
	return out
}
