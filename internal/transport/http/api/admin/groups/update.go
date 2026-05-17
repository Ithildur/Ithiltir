package groups

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

type updateInput struct {
	Name   *string `json:"name"`
	Remark *string `json:"remark"`
}

func updateRoute(r *routes.Blueprint, h *handler) {
	r.Patch(
		"/{id}",
		"Update group",
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

	updates, hasUpdates, validationErr := updatesFromInput(in)
	if validationErr != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", validationErr.Error())
		return
	}
	if !hasUpdates {
		httperr.Write(w, http.StatusBadRequest, "no_fields", "no fields to update")
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.UpdateGroup(c, id, updates)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "group not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update group")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updatesFromInput(in updateInput) (map[string]any, bool, error) {
	updates := make(map[string]any)
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, false, errors.New("name cannot be empty")
		}
		updates["name"] = name
	}
	if in.Remark != nil {
		updates["remark"] = strings.TrimSpace(*in.Remark)
	}
	return updates, len(updates) > 0, nil
}
