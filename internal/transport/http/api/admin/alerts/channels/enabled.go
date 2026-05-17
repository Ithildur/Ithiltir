package channels

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

type enabledInput struct {
	Enabled *bool `json:"enabled"`
}

func enabledRoute(r *routes.Blueprint, h *handler) {
	r.Put(
		"/{id}/enabled",
		"Update alert channel enabled status",
		routes.Func(h.enabledHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) enabledHandler(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	var in enabledInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}
	if in.Enabled == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "enabled is required")
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.ReplaceChannel(c, id, map[string]any{
			"enabled": *in.Enabled,
		})
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
