package virt

import (
	"context"
	"errors"
	"net/http"
	"time"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/nodeingest"
	"dash/internal/virt"
	"github.com/Ithildur/EiluneKit/http/decoder"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type handler struct {
	ingest     *nodeingest.Receiver
	failedAuth http.Handler
}

func Router(ingest *nodeingest.Receiver, failedAuth http.Handler) *routes.Blueprint {
	h := &handler{ingest: ingest, failedAuth: failedAuth}
	r := routes.NewBlueprint(routes.DefaultMiddleware(middleware.LimitBody(virt.MaxBytes)))
	virtRoute(r, h)
	return r
}

func virtRoute(r *routes.Blueprint, h *handler) {
	r.Post("", "Push VM snapshot", h.virtHandler)
}

func (h *handler) virtHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	now := time.Now().UTC()
	ctx, cancel := context.WithDeadline(r.Context(), now.Add(config.PGWriteTimeout))
	defer cancel()
	secret := r.Header.Get(request.NodeSecretHeader)
	server, err := h.ingest.Authenticate(ctx, secret)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var snapshot virt.Snapshot
	if err := decoder.DecodeJSONBody(r, &snapshot); err != nil {
		if errors.Is(err, decoder.ErrBodyTooLarge) {
			httperr.Write(w, http.StatusRequestEntityTooLarge, "body_too_large", "VM snapshot exceeds limit")
		} else {
			httperr.Write(w, http.StatusBadRequest, "invalid_virt", "invalid VM snapshot")
		}
		return
	}
	if err := h.ingest.Virt(ctx, server.ID, secret, snapshot, now); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	if failure, ok := errors.AsType[*nodeingest.Error](err); ok {
		switch failure.Code {
		case "unauthorized":
			h.failedAuth.ServeHTTP(w, r)
			return
		case "invalid_virt":
			httperr.Write(w, http.StatusBadRequest, "invalid_virt", "invalid VM snapshot")
			return
		}
	}
	httperr.Write(w, http.StatusServiceUnavailable, "virt_unavailable", "VM snapshot storage unavailable")
}
