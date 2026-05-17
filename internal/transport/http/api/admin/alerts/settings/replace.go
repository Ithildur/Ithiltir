package settings

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/infra"
	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type replaceInput struct {
	Enabled    *bool    `json:"enabled"`
	ChannelIDs *[]int64 `json:"channel_ids"`
}

func replaceRoute(r *routes.Blueprint, h *handler) {
	r.Put(
		"/",
		"Replace alert settings",
		routes.Func(h.replaceHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

var errUnknownChannelID = errors.New("channel_ids contains unknown channel")

func (h *handler) replaceHandler(w http.ResponseWriter, r *http.Request) {
	var in replaceInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	if in.Enabled == nil || in.ChannelIDs == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "enabled and channel_ids are required")
		return
	}

	ids, err := normalizeIDs(*in.ChannelIDs)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}

	if err := ensureChannels(r.Context(), h.store, ids); err != nil {
		if errors.Is(err, errUnknownChannelID) {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to validate channels")
		return
	}

	if err := saveSettings(r.Context(), h.store, *in.Enabled, ids); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update settings")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func saveSettings(ctx context.Context, st *alertstore.Store, enabled bool, ids []int64) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.UpsertSettings(c, enabled, ids)
	})
	return err
}

func normalizeIDs(ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("channel_ids cannot contain non-positive values")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func ensureChannels(ctx context.Context, st *alertstore.Store, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	items, err := infra.WithPGReadTimeout(ctx, func(c context.Context) ([]model.NotifyChannel, error) {
		return st.ListChannelsByIDs(c, ids)
	})
	if err != nil {
		return err
	}
	if len(items) != len(ids) {
		return errUnknownChannelID
	}
	return nil
}
