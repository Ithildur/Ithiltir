package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"dash/internal/infra"
	"dash/internal/model"
	"dash/internal/notify"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type channelView struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Type      model.NotifyType `json:"type"`
	Config    any              `json:"config"`
	Enabled   bool             `json:"enabled"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at"`
}

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List alert channels",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]model.NotifyChannel, error) {
		return h.store.ListChannels(c)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch channels")
		return
	}

	out := make([]channelView, 0, len(items))
	for _, item := range items {
		configView, err := notify.SanitizeConfig(item.Type, json.RawMessage(item.Config))
		if err != nil {
			httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to decode channel config")
			return
		}
		out = append(out, channelView{
			ID:        item.ID,
			Name:      item.Name,
			Type:      item.Type,
			Config:    configView,
			Enabled:   item.Enabled,
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
			UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
		})
	}

	response.WriteJSON(w, http.StatusOK, out)
}
