package rules

import (
	"context"
	"net/http"

	"dash/internal/alertspec"
	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type createInput struct {
	Name            string   `json:"name"`
	Enabled         *bool    `json:"enabled"`
	Metric          string   `json:"metric"`
	Operator        string   `json:"operator"`
	Threshold       *float64 `json:"threshold"`
	DurationSec     *int32   `json:"duration_sec"`
	CooldownMin     *int32   `json:"cooldown_min"`
	ThresholdMode   *string  `json:"threshold_mode"`
	ThresholdOffset *float64 `json:"threshold_offset"`
}

func createRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/",
		"Create alert rule",
		routes.Func(h.createHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) createHandler(w http.ResponseWriter, r *http.Request) {
	var in createInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	rule, err := ruleFromInput(in)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.CreateRule(c, rule)
	}); err != nil {
		if alertspec.IsValidationError(err) {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to create rule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
