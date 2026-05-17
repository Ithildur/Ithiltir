package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"dash/internal/infra"
	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"gorm.io/gorm"
)

const (
	defaultEnabled = true
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

var defaultIDs = []int64{}

func (h *handler) detailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	item, err := getSettings(r.Context(), h.store)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := saveSettings(r.Context(), h.store, defaultEnabled, defaultIDs); err != nil {
				httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to initialize settings")
				return
			}
			item, err = getSettings(r.Context(), h.store)
		}
	}
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch settings")
		return
	}

	ids, err := decodeIDs(item.ChannelIDs)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to parse settings")
		return
	}
	ids, err = filterIDs(r.Context(), h.store, ids)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to validate channels")
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

func decodeIDs(raw []byte) ([]int64, error) {
	if len(raw) == 0 {
		return []int64{}, nil
	}
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, err
	}
	if ids == nil {
		return []int64{}, nil
	}
	return ids, nil
}

func filterIDs(ctx context.Context, st *alertstore.Store, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}
	items, err := infra.WithPGReadTimeout(ctx, func(c context.Context) ([]model.NotifyChannel, error) {
		return st.ListChannelsByIDs(c, ids)
	})
	if err != nil {
		return nil, err
	}
	known := make(map[int64]struct{}, len(items))
	for _, item := range items {
		known[item.ID] = struct{}{}
	}
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
