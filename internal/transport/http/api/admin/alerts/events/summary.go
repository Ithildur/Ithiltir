package events

import (
	"context"
	"net/http"
	"time"

	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type summaryView struct {
	Items []summaryItemView `json:"items"`
}

type summaryItemView struct {
	ServerID      int64  `json:"server_id"`
	OpenCount     int64  `json:"open_count"`
	LastTriggerAt string `json:"last_trigger_at"`
	Metric        string `json:"metric"`
	RuleName      string `json:"rule_name"`
}

func summaryRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/summary",
		"List open alert event summaries",
		routes.Func(h.summaryHandler),
	)
}

func (h *handler) summaryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]alertstore.OpenEventSummary, error) {
		return h.alerts.ListOpenEventSummaries(c)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch alert summaries")
		return
	}

	response.WriteJSON(w, http.StatusOK, summaryView{Items: summaryViews(items)})
}

func summaryViews(items []alertstore.OpenEventSummary) []summaryItemView {
	if len(items) == 0 {
		return make([]summaryItemView, 0)
	}
	out := make([]summaryItemView, 0, len(items))
	for _, item := range items {
		out = append(out, summaryItemView{
			ServerID:      item.ServerID,
			OpenCount:     item.OpenCount,
			LastTriggerAt: item.LastTriggerAt.UTC().Format(time.RFC3339),
			Metric:        item.Metric,
			RuleName:      item.RuleName,
		})
	}
	return out
}
