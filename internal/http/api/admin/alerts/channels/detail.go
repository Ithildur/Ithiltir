package channels

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

func detailRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/{id}",
		"Get alert channel",
		h.detailHandler,
	)
}

func (h *handler) detailHandler(w http.ResponseWriter, r *http.Request, rawID string) {

	id, err := request.ParseIDInt64(rawID)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	item, err := loadChannelDelivery(r.Context(), h.store, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch channel")
		return
	}

	response.WriteJSON(w, http.StatusOK, viewFromDelivery(item))
}

func loadChannelDelivery(ctx context.Context, st *alertstore.Store, id int64) (alertstore.ChannelDelivery, error) {
	dbCtx, cancel := context.WithTimeout(ctx, config.PGReadTimeout)
	defer cancel()
	return st.GetChannelDelivery(dbCtx, id)
}

func loadChannel(ctx context.Context, st *alertstore.Store, id int64) (*model.NotifyChannel, error) {
	dbCtx, cancel := context.WithTimeout(ctx, config.PGReadTimeout)
	defer cancel()
	return st.GetChannel(dbCtx, id)
}
