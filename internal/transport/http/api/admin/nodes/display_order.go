package nodes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type displayOrderInput struct {
	IDs []int64 `json:"ids"`
}

func displayOrderRoute(r *routes.Blueprint, h *handler) {
	r.Put(
		"/display-order",
		"Update node display order",
		routes.Func(h.displayOrderHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) displayOrderHandler(w http.ResponseWriter, r *http.Request) {
	var in displayOrderInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	ids, err := normalizeIDList(in.IDs)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_ids", err.Error())
		return
	}

	if _, err := infra.WithPGWriteTimeout(r.Context(), func(c context.Context) (struct{}, error) {
		return struct{}{}, h.store.UpdateDisplayOrder(c, ids)
	}); err != nil {
		if errors.Is(err, nodestore.ErrServerMetaCacheUpdate) {
			infra.WithModule("admin.nodes").Error("server cache sync failed after display order update", err,
				slog.Int("count", len(ids)),
			)
			httperr.Write(w, http.StatusServiceUnavailable, "redis_cache_error", "sync failed")
			return
		} else if errors.Is(err, nodestore.ErrFrontCacheUpdate) {
			infra.WithModule("admin.nodes").Warn("front cache sync failed after display order update", err,
				slog.Int("count", len(ids)),
			)
			httperr.Write(w, http.StatusServiceUnavailable, "redis_cache_error", "sync failed")
			return
		} else {
			httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update display order")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func normalizeIDList(in []int64) ([]int64, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("ids is required")
	}
	if len(in) > 10000 {
		return nil, fmt.Errorf("too many ids")
	}
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, id := range in {
		if id <= 0 {
			return nil, fmt.Errorf("id must be positive")
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("duplicate id %d", id)
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
