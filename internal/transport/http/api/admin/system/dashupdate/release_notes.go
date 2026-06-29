package dashupdate

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type releaseNotesView struct {
	SourceURL string `json:"source_url"`
	HTML      string `json:"html"`
}

func releaseNotesRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/release-notes",
		"Get Dash release notes document",
		routes.Func(h.releaseNotesHandler),
	)
}

func (h *handler) releaseNotesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	sourceURL, ok := releaseNotesURL(r.URL.Query().Get("lang"))
	if !ok {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "invalid lang")
		return
	}
	html, err := h.fetchReleaseNotes(r, sourceURL)
	if err != nil {
		httperr.Write(w, http.StatusBadGateway, "release_notes_fetch_failed", "failed to fetch release notes")
		return
	}

	response.WriteJSON(w, http.StatusOK, releaseNotesView{
		SourceURL: sourceURL,
		HTML:      html,
	})
}

func releaseNotesURL(lang string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh":
		return "https://www.ithiltir.dev/docs/ReleaseNotes", true
	case "en":
		return "https://www.ithiltir.dev/en/docs/ReleaseNotes", true
	default:
		return "", false
	}
}

func (h *handler) fetchReleaseNotes(r *http.Request, sourceURL string) (string, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "Ithiltir-Dash")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("release notes status: %s", resp.Status)
	}

	limited := io.LimitReader(resp.Body, releaseNotesMaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(body) > releaseNotesMaxBytes {
		return "", fmt.Errorf("release notes response too large")
	}
	return string(body), nil
}
