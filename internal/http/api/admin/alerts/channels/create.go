package channels

import (
	"context"
	"encoding/json"
	"net/http"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/model"
	"dash/internal/notify"
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
		h.createHandler,
		routes.Use(middleware.RequireJSONBody),
	)
	r.Post(
		"",
		"Create alert channel",
		h.createHandler,
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) createHandler(w http.ResponseWriter, r *http.Request) {
	var in createInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	name, err := normalizeChannelName(in.Name)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
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

	dbCtx, cancel := context.WithTimeout(r.Context(), config.PGWriteTimeout)
	defer cancel()
	err = h.store.CreateChannel(dbCtx, &model.NotifyChannel{
		Name:    name,
		Type:    typ,
		Config:  datatypes.JSON(cfg),
		Enabled: *in.Enabled,
	})
	cancel()
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to create channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
