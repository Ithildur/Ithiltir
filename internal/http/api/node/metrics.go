package node

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Ithildur/EiluneKit/http/decoder"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/nodeingest"
)

type handler struct {
	ingest     *nodeingest.Receiver
	failedAuth http.Handler
}

func (h *handler) metricsRoute(r *routes.Blueprint) {
	r.Post("/metrics", "Push node metrics", h.metricsHandler, routes.Use(ingestMiddleware()...), routes.Tags("node"))
}
func (h *handler) metricsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	receivedAt := time.Now().UTC()
	ctx, cancel := context.WithDeadline(r.Context(), receivedAt.Add(config.PGWriteTimeout))
	defer cancel()
	logger := infra.WithModule("node")
	secret, server, err := h.authenticate(ctx, r)
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	var report metrics.NodeReport
	if err := decoder.DecodeJSONBody(r, &report); err != nil {
		if errors.Is(err, decoder.ErrBodyTooLarge) {
			err = httperr.BodyTooLarge(err)
		} else {
			err = httperr.InvalidRequest(err)
		}
		h.writeError(w, r, logger, err)
		return
	}
	ip, _ := request.NodeIP(r)
	result, err := h.ingest.Metrics(ctx, secret, server, report, receivedAt, ip)
	if err != nil {
		h.writeError(w, r, logger, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, result)
}
