package groups

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/infra"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

func deleteRoute(r *routes.Blueprint, h *handler) {
	r.Delete(
		"/{id}",
		"Delete group",
		h.deleteHandler,
	)
}

func (h *handler) deleteHandler(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := request.ParseIDInt64(rawID)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	if id == 1 {
		httperr.Write(w, http.StatusConflict, "default_group_not_deletable", "默认分组无法删除")
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.DeleteGroup(c, id)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "group not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to delete group")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
