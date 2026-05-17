package channels

import (
	"context"
	"encoding/json"
	"errors"
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
	"gorm.io/gorm"
)

type replaceInput struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled"`
}

func replaceRoute(r *routes.Blueprint, h *handler) {
	r.Put(
		"/{id}",
		"Replace alert channel",
		routes.Func(h.replaceHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) replaceHandler(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	var in replaceInput
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

	existing, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) (*model.NotifyChannel, error) {
		return h.store.GetChannel(c, id)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch channel")
		return
	}

	cfg, err := notify.NormalizeConfigForUpdate(typ, in.Config, existing.Type, json.RawMessage(existing.Config))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}

	updates := map[string]any{
		"name":    name,
		"type":    typ,
		"config":  datatypes.JSON(cfg),
		"enabled": *in.Enabled,
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.ReplaceChannel(c, id, updates)
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
