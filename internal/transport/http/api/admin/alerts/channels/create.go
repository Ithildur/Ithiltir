package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/datatypes"
)

type createInput struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled"`
}

func createRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/",
		"Create alert channel",
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
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "name is required")
		return
	}
	if in.Enabled == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "enabled is required")
		return
	}

	typ, err := notify.NormalizeType(in.Type)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}
	cfg, err := notify.NormalizeConfig(typ, in.Config)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.CreateChannel(c, &model.NotifyChannel{
			Name:    name,
			Type:    typ,
			Config:  datatypes.JSON(cfg),
			Enabled: *in.Enabled,
		})
	}); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to create channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
