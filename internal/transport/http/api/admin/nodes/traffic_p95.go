package nodes

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

type trafficP95Input struct {
	IDs     []int64 `json:"ids"`
	Enabled *bool   `json:"enabled"`
}

func trafficP95Route(r *routes.Blueprint, h *handler) {
	r.Patch(
		"/traffic-p95",
		"Update node traffic P95",
		routes.Func(h.trafficP95Handler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) trafficP95Handler(w http.ResponseWriter, r *http.Request) {
	var in trafficP95Input
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}
	if in.Enabled == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "enabled is required")
		return
	}

	ids, err := normalizeIDList(in.IDs)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_ids", err.Error())
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.SetTrafficP95(c, ids, *in.Enabled)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update traffic P95")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
