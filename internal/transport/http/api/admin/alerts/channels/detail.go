package channels

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

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

func (h *handler) detailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	id, err := request.ParseIDInt64(r, "id")
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	item, err := loadChannel(r.Context(), h.store, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.Write(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch channel")
		return
	}
	configView, err := notify.SanitizeConfig(item.Type, json.RawMessage(item.Config))
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to decode channel config")
		return
	}

	response.WriteJSON(w, http.StatusOK, channelView{
		ID:        item.ID,
		Name:      item.Name,
		Type:      item.Type,
		Config:    configView,
		Enabled:   item.Enabled,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	})
}

func loadChannel(ctx context.Context, st *alertstore.Store, id int64) (*model.NotifyChannel, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (*model.NotifyChannel, error) {
		return st.GetChannel(c, id)
	})
}
