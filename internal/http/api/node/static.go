package node

import (
	"errors"
	"net/http"

	"github.com/Ithildur/EiluneKit/http/decoder"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/http/httperr"
	"dash/internal/infra"
	"dash/internal/metrics"
)

func (h *handler) staticRoute(r *routes.Blueprint) {
	r.Post("/static", "Push node static metrics", h.staticHandler, routes.Use(ingestMiddleware()...), routes.Tags("node"))
}
func (h *handler) staticHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	logger := infra.WithModule("node")
	secret, server, err := h.authenticate(r.Context(), r)
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	var snapshot metrics.StaticMetrics
	if err := decoder.DecodeJSONBody(r, &snapshot); err != nil {
		if errors.Is(err, decoder.ErrBodyTooLarge) {
			err = httperr.BodyTooLarge(err)
		} else {
			err = httperr.InvalidRequest(err)
		}
		h.writeError(w, r, logger, err)
		return
	}
	ip, _ := nodeClientIP(r)
	if err := h.ingest.Static(r.Context(), secret, server.ID, snapshot, ip); err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
