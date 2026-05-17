package rules

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/alertspec"
	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

type updateInput struct {
	Name            *string  `json:"name"`
	Enabled         *bool    `json:"enabled"`
	Metric          *string  `json:"metric"`
	Operator        *string  `json:"operator"`
	Threshold       *float64 `json:"threshold"`
	DurationSec     *int32   `json:"duration_sec"`
	CooldownMin     *int32   `json:"cooldown_min"`
	ThresholdMode   *string  `json:"threshold_mode"`
	ThresholdOffset *float64 `json:"threshold_offset"`
}

func updateRoute(r *routes.Blueprint, h *handler) {
	r.Patch(
		"/{id}",
		"Update alert rule",
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

	patch, hasUpdates, err := patchFromInput(in)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}
	if !hasUpdates {
		httperr.Write(w, http.StatusBadRequest, "no_fields", "no fields to update")
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.PatchRule(c, id, patch)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "rule not found")
			return
		}
		if alertspec.IsValidationError(err) {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update rule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
