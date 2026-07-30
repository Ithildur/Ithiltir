package settings

import (
	"context"
	"net/http"
	"time"

	"dash/internal/infra"
	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type settingsView struct {
	Enabled    bool    `json:"enabled"`
	ChannelIDs []int64 `json:"channel_ids"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

func detailRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"Get alert settings",
		routes.Func(h.detailHandler),
	)
}

func (h *handler) detailHandler(w http.ResponseWriter, r *http.Request) {
	item, err := getSettings(r.Context(), h.store)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch settings")
		return
	}

	ids, err := alertstore.DecodeChannelIDs(item.ChannelIDs)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to parse settings")
		return
	}

	response.WriteJSON(w, http.StatusOK, settingsView{
		Enabled:    item.Enabled,
		ChannelIDs: ids,
		CreatedAt:  item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  item.UpdatedAt.Format(time.RFC3339),
	})
}

func getSettings(ctx context.Context, st *alertstore.Store) (*model.AlertSetting, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (*model.AlertSetting, error) {
		return st.GetSettings(c)
	})
}
