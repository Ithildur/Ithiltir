package nodes

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/routes"
)

func createRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/",
		"Create node",
		routes.Func(h.createHandler),
	)
}

func (h *handler) createHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var secret string
	var err error
	collisions := 0
	for attempts := 0; attempts < 5; attempts++ {
		secret, err = h.store.GenerateSecret()
		if err != nil {
			break
		}

		_, err = infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
			_, err := h.store.CreateNode(c, secret)
			return struct{}{}, err
		})
		if err == nil {
			break
		}
		if errors.Is(err, nodestore.ErrDuplicateSecret) {
			collisions++
			continue
		}
		if errors.Is(err, nodestore.ErrServerMetaCacheUpdate) || errors.Is(err, nodestore.ErrFrontCacheUpdate) {
			httperr.Write(w, http.StatusServiceUnavailable, "redis_cache_error", "sync failed")
			return
		}
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to create server")
		return
	}
	if err != nil {
		if errors.Is(err, nodestore.ErrDuplicateSecret) && collisions > 0 {
			httperr.Write(w, http.StatusInternalServerError, "secret_collision_exhausted", "failed to allocate secret")
			return
		}
		httperr.Write(w, http.StatusInternalServerError, "secret_generation_failed", "failed to generate secret")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
