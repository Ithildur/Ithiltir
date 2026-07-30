package channels

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

func detailRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/{id}",
		"Get alert channel",
		routes.Func(h.detailHandler),
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
	configView, err := notify.SanitizeConfig(item.Channel.Type, json.RawMessage(item.Channel.Config))
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to decode channel config")
		return
	}

	response.WriteJSON(w, http.StatusOK, viewFromDelivery(item, configView))
}

func loadChannelDelivery(ctx context.Context, st *alertstore.Store, id int64) (alertstore.ChannelDelivery, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (alertstore.ChannelDelivery, error) {
		return st.GetChannelDelivery(c, id)
	})
}

func loadChannel(ctx context.Context, st *alertstore.Store, id int64) (*model.NotifyChannel, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (*model.NotifyChannel, error) {
		return st.GetChannel(c, id)
	})
}
