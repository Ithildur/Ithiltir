package events

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

const (
	defaultEventLimit = 200
	maxEventLimit     = 500
)

type listView struct {
	Items      []eventView `json:"items"`
	NextCursor *string     `json:"next_cursor"`
	HasMore    bool        `json:"has_more"`
}

type eventView struct {
	ID                 int64    `json:"id"`
	RuleID             int64    `json:"rule_id"`
	RuleGeneration     int64    `json:"rule_generation"`
	ServerID           int64    `json:"server_id"`
	ServerName         string   `json:"server_name"`
	ServerHostname     string   `json:"server_hostname"`
	ServerIP           *string  `json:"server_ip,omitempty"`
	Status             string   `json:"status"`
	Metric             string   `json:"metric"`
	RuleName           string   `json:"rule_name"`
	FirstTriggerAt     string   `json:"first_trigger_at"`
	LastTriggerAt      string   `json:"last_trigger_at"`
	ClosedAt           *string  `json:"closed_at,omitempty"`
	CurrentValue       *float64 `json:"current_value,omitempty"`
	EffectiveThreshold *float64 `json:"effective_threshold,omitempty"`
	CloseReason        *string  `json:"close_reason,omitempty"`
	Title              *string  `json:"title,omitempty"`
	Message            *string  `json:"message,omitempty"`
}

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List alert events",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	query, err := parseListQuery(r.URL.Query())
	if err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return
	}

	items, err := infra.WithPGReadTimeout(r.Context(), func(c context.Context) ([]alertstore.AlertEventItem, error) {
		storeQuery := query
		storeQuery.Limit = query.Limit + 1
		return h.alerts.ListEvents(c, storeQuery)
	})
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch alert events")
		return
	}

	hasMore := len(items) > query.Limit
	if hasMore {
		items = items[:query.Limit]
	}

	var nextCursor *string
	if hasMore && len(items) > 0 {
		cursor := eventCursor(items[len(items)-1])
		nextCursor = &cursor
	}

	response.WriteJSON(w, http.StatusOK, listView{
		Items:      eventViews(items),
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

func parseListQuery(q url.Values) (alertstore.AlertEventQuery, error) {
	status, err := parseStatus(q.Get("status"))
	if err != nil {
		return alertstore.AlertEventQuery{}, err
	}
	serverID, err := parseOptionalServerID(q.Get("server_id"))
	if err != nil {
		return alertstore.AlertEventQuery{}, err
	}
	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		return alertstore.AlertEventQuery{}, err
	}
	from, err := parseTimeParam(q.Get("from"))
	if err != nil {
		return alertstore.AlertEventQuery{}, errors.New("invalid from")
	}
	to, err := parseTimeParam(q.Get("to"))
	if err != nil {
		return alertstore.AlertEventQuery{}, errors.New("invalid to")
	}
	cursor, err := parseEventCursor(q.Get("cursor"))
	if err != nil {
		return alertstore.AlertEventQuery{}, err
	}
	if from != nil && to != nil && from.After(*to) {
		return alertstore.AlertEventQuery{}, errors.New("from must be before to")
	}

	return alertstore.AlertEventQuery{
		ServerID: serverID,
		Status:   status,
		Metric:   strings.TrimSpace(q.Get("metric")),
		From:     from,
		To:       to,
		Cursor:   cursor,
		Limit:    limit,
	}, nil
}

func parseStatus(raw string) (alertstore.EventStatus, error) {
	switch strings.TrimSpace(raw) {
	case "", string(alertstore.EventStatusOpen):
		return alertstore.EventStatusOpen, nil
	case string(alertstore.EventStatusClosed):
		return alertstore.EventStatusClosed, nil
	case string(alertstore.EventStatusAll):
		return alertstore.EventStatusAll, nil
	default:
		return "", errors.New("invalid status")
	}
}

func parseOptionalServerID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err == nil && id > 0 {
		return id, nil
	}
	return 0, errors.New("invalid server_id")
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultEventLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, errors.New("invalid limit")
	}
	if limit > maxEventLimit {
		return maxEventLimit, nil
	}
	return limit, nil
}

func parseTimeParam(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	t = t.UTC()
	return &t, nil
}

func parseEventCursor(raw string) (*alertstore.AlertEventCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	rawAt, rawID, ok := strings.Cut(raw, ",")
	if !ok {
		return nil, errors.New("invalid cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, rawAt)
	if err != nil {
		return nil, errors.New("invalid cursor")
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return nil, errors.New("invalid cursor")
	}
	return &alertstore.AlertEventCursor{LastTriggerAt: at.UTC(), ID: id}, nil
}

func eventCursor(item alertstore.AlertEventItem) string {
	return item.LastTriggerAt.UTC().Format(time.RFC3339Nano) + "," + strconv.FormatInt(item.ID, 10)
}

func eventViews(items []alertstore.AlertEventItem) []eventView {
	if len(items) == 0 {
		return make([]eventView, 0)
	}
	out := make([]eventView, 0, len(items))
	for _, item := range items {
		out = append(out, eventView{
			ID:                 item.ID,
			RuleID:             item.RuleID,
			RuleGeneration:     item.RuleGeneration,
			ServerID:           item.ServerID,
			ServerName:         item.ServerName,
			ServerHostname:     item.ServerHostname,
			ServerIP:           item.ServerIP,
			Status:             string(item.Status),
			Metric:             item.Metric,
			RuleName:           item.RuleName,
			FirstTriggerAt:     item.FirstTriggerAt.UTC().Format(time.RFC3339),
			LastTriggerAt:      item.LastTriggerAt.UTC().Format(time.RFC3339),
			ClosedAt:           formatOptionalTime(item.ClosedAt),
			CurrentValue:       item.CurrentValue,
			EffectiveThreshold: item.EffectiveThreshold,
			CloseReason:        item.CloseReason,
			Title:              item.Title,
			Message:            item.Message,
		})
	}
	return out
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	out := t.UTC().Format(time.RFC3339)
	return &out
}
