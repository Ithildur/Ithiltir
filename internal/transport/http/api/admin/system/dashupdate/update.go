package dashupdate

import (
	"errors"
	"net/http"

	updater "dash/internal/dashupdate"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type updateRunInput struct {
	Action  updater.Action  `json:"action"`
	Channel updater.Channel `json:"channel"`
	Lang    string          `json:"lang"`
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
	w.Header().Set("Cache-Control", "no-store")
	response.WriteJSON(w, http.StatusOK, h.runner.Status(r.Context()))
}

func (h *handler) checkHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	channel := updater.ChannelRelease
	if raw := r.URL.Query().Get("channel"); raw != "" {
		normalized, ok := updater.ParseChannel(updater.Channel(raw))
		if !ok {
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid channel")
			return
		}
		channel = normalized
	}

	out, err := h.runner.Check(r.Context(), channel)
	if err != nil {
		if errors.Is(err, updater.ErrUnavailable) {
			httperr.Write(w, http.StatusServiceUnavailable, "dash_update_unavailable", err.Error())
			return
		}
		httperr.Write(w, http.StatusBadGateway, "dash_update_check_failed", "failed to check dash update")
		return
	}

	response.WriteJSON(w, http.StatusOK, out)
}

func (h *handler) runHandler(w http.ResponseWriter, r *http.Request) {
	var in updateRunInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}

	status, err := h.runner.Start(r.Context(), updater.RunInput{
		Action:  in.Action,
		Channel: in.Channel,
		Lang:    in.Lang,
	})
	if err != nil {
		switch {
		case errors.Is(err, updater.ErrRunning):
			response.WriteJSON(w, http.StatusConflict, status)
		case errors.Is(err, updater.ErrUnavailable):
			httperr.Write(w, http.StatusServiceUnavailable, "dash_update_unavailable", err.Error())
		case errors.Is(err, updater.ErrInvalidRequest):
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		default:
			httperr.Write(w, http.StatusServiceUnavailable, "dash_update_failed", "failed to start dash update")
		}
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	response.WriteJSON(w, http.StatusAccepted, status)
}
