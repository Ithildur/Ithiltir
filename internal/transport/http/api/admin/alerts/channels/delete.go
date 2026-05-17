package channels

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/infra"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

func deleteRoute(r *routes.Blueprint, h *handler) {
	r.Delete(
		"/{id}",
		"Delete alert channel",
		routes.Func(h.deleteHandler),
	)
}

func (h *handler) deleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.DeleteChannel(c, id)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to delete channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
