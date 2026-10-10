package settings

import (
	"net/http"
	"time"

	"dash/internal/http/httperr"
	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
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
		h.detailHandler,
	)
	r.Get(
		"",
		"Get alert settings",
		h.detailHandler,
	)
}

func (h *handler) detailHandler(w http.ResponseWriter, r *http.Request) {
	item, err := infra.WithPGReadTimeout(r.Context(), h.store.GetSettings)
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
