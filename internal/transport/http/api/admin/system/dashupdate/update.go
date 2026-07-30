package dashupdate

import (
	"context"
	"errors"
	"net/http"
	"time"

	updater "dash/internal/dashupdate"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

const (
	updateCheckLimit = 12 * time.Second
	updateStartLimit = 15 * time.Second
	updateRunLimit   = 30 * time.Second
)

type updateRunInput struct {
	Action                  updater.Action  `json:"action"`
	Channel                 updater.Channel `json:"channel"`
	Lang                    string          `json:"lang"`
	TargetVersion           string          `json:"target_version"`
	ExpectedCurrentVersion  string          `json:"expected_current_version"`
	ExpectedInstallRevision string          `json:"expected_install_revision"`
}

func statusRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/status",
		"Get Dash update status",
		routes.Func(h.statusHandler),
	)
}

func checkRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/check",
		"Check Dash update version",
		routes.Func(h.checkHandler),
	)
}

func runRoute(r *routes.Blueprint, h *handler) {
	r.Post(
		"/run",
		"Run Dash update",
		routes.Func(h.runHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

func (h *handler) statusHandler(w http.ResponseWriter, r *http.Request) {
	response.WriteJSON(w, http.StatusOK, h.runner.Status(r.Context()))
}

func (h *handler) checkHandler(w http.ResponseWriter, r *http.Request) {
	channel := updater.ChannelRelease
	if raw := r.URL.Query().Get("channel"); raw != "" {
		normalized, ok := updater.ParseChannel(updater.Channel(raw))
		if !ok {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid channel")
			return
		}
		channel = normalized
	}

	ctx, cancel := context.WithTimeout(r.Context(), updateCheckLimit)
	defer cancel()
	out, err := h.runner.Check(ctx, channel)
	if err != nil {
		if errors.Is(err, updater.ErrUnavailable) && ctx.Err() == nil {
			httperr.Write(w, http.StatusServiceUnavailable, "dash_update_unavailable", err.Error())
			return
		}
		httperr.Write(w, http.StatusBadGateway, "dash_update_check_failed", "failed to check dash update")
		return
	}

	response.WriteJSON(w, http.StatusOK, out)
}

func (h *handler) runHandler(w http.ResponseWriter, r *http.Request) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(updateRunLimit)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		httperr.Write(w, http.StatusServiceUnavailable, "dash_update_unavailable", "failed to establish dash update response deadline")
		return
	}
	var in updateRunInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	input := updater.RunInput{
		Action:                  in.Action,
		Channel:                 in.Channel,
		Lang:                    in.Lang,
		TargetVersion:           in.TargetVersion,
		ExpectedCurrentVersion:  in.ExpectedCurrentVersion,
		ExpectedInstallRevision: in.ExpectedInstallRevision,
	}
	prepareCtx, cancelPrepare := context.WithTimeout(r.Context(), updateCheckLimit)
	plan, err := h.runner.Prepare(prepareCtx, input)
	cancelPrepare()
	if err != nil {
		writeRunError(w, updater.State{}, err)
		return
	}

	startCtx, cancelStart := context.WithTimeout(r.Context(), updateStartLimit)
	status, err := h.runner.StartPrepared(startCtx, plan)
	cancelStart()
	if err != nil {
		writeRunError(w, status, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, status)
}

func writeRunError(w http.ResponseWriter, status updater.State, err error) {
	switch {
	case errors.Is(err, updater.ErrRunning):
		response.WriteJSON(w, http.StatusConflict, status)
	case errors.Is(err, updater.ErrAlreadyCurrent):
		httperr.Write(w, http.StatusConflict, "dash_update_current", err.Error())
	case errors.Is(err, updater.ErrUnavailable):
		httperr.Write(w, http.StatusServiceUnavailable, "dash_update_unavailable", err.Error())
	case errors.Is(err, updater.ErrInvalidRequest):
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
	default:
		httperr.Write(w, http.StatusServiceUnavailable, "dash_update_failed", "failed to start dash update")
	}
}
