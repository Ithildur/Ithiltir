package groups

import (
	"context"
	"net/http"
	"strings"

	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type createInput struct {
	Name   string  `json:"name"`
	Remark *string `json:"remark"`
}

func createRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/",
		"Create group",
		routes.Func(h.createHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) createHandler(w http.ResponseWriter, r *http.Request) {
	var in createInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		httperr.Write(w, http.StatusBadRequest, "invalid_name", "name is required")
		return
	}

	remark := ""
	if in.Remark != nil {
		remark = strings.TrimSpace(*in.Remark)
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		_, err := h.store.CreateGroup(c, name, remark)
		return struct{}{}, err
	}); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to create group")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
