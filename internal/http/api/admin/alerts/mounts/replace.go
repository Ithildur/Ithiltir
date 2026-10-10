package mounts

import (
	"context"
	"errors"
	"net/http"

	"dash/internal/alertspec"
	"dash/internal/config"
	"dash/internal/http/httperr"
	"dash/internal/http/request"
	alertstore "dash/internal/store/alert"
	nodestore "dash/internal/store/node"
	"github.com/Ithildur/EiluneKit/http/middleware"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type replaceInput struct {
	RuleIDs   []int64 `json:"rule_ids"`
	ServerIDs []int64 `json:"server_ids"`
	Mounted   *bool   `json:"mounted"`
}

func replaceRoute(r *routes.Blueprint, h *handler) {
	r.Put(
		"/",
		"Set alert rule mounts",
		h.replaceHandler,
		routes.Use(middleware.RequireJSONBody),
	)
	r.Put(
		"",
		"Set alert rule mounts",
		h.replaceHandler,
		routes.Use(middleware.RequireJSONBody),
	)
}

var (
	errUnknownRule   = errors.New("rule_ids contains unknown rule")
	errUnknownServer = errors.New("server_ids contains unknown node")
)

func (h *handler) replaceHandler(w http.ResponseWriter, r *http.Request) {
	var in replaceInput
	if ok := request.DecodeJSONOrWriteError(w, r, &in); !ok {
		return
	}
	if in.Mounted == nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", "mounted is required")
		return
	}
	ruleIDs, err := normalizeRuleIDs(in.RuleIDs)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}
	serverIDs, err := normalizeServerIDs(in.ServerIDs)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}
	if err := ensureKnown(r.Context(), h.alert, h.node, ruleIDs, serverIDs); err != nil {
		switch {
		case errors.Is(err, errUnknownRule), errors.Is(err, errUnknownServer):
			httperr.Write(w, http.StatusBadRequest, "invalid_fields", err.Error())
		default:
			httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to validate alert mounts")
		}
		return
	}
	if err := save(r.Context(), h.alert, ruleIDs, serverIDs, *in.Mounted); err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to update alert mounts")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeRuleIDs(ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return nil, errors.New("rule_ids cannot contain zero")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, errors.New("rule_ids is required")
	}
	return out, nil
}

func normalizeServerIDs(ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("server_ids cannot contain non-positive values")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, errors.New("server_ids is required")
	}
	return out, nil
}

func ensureKnown(ctx context.Context, alert *alertstore.Store, node *nodestore.Store, ruleIDs, serverIDs []int64) error {
	dbCtx, cancel := context.WithTimeout(ctx, config.PGReadTimeout)
	defer cancel()
	rules, err := alert.ListRules(dbCtx)
	if err != nil {
		return err
	}
	knownRules := make(map[int64]struct{}, len(rules)+len(alertspec.BuiltinRules()))
	for _, id := range alertspec.BuiltinRuleIDs() {
		knownRules[id] = struct{}{}
	}
	for _, rule := range rules {
		knownRules[rule.ID] = struct{}{}
	}
	for _, id := range ruleIDs {
		if _, ok := knownRules[id]; !ok {
			return errUnknownRule
		}
	}

	nodes, err := node.Nodes(dbCtx)
	if err != nil {
		return err
	}
	knownServers := make(map[int64]struct{}, len(nodes))
	for _, node := range nodes {
		knownServers[node.ID] = struct{}{}
	}
	for _, id := range serverIDs {
		if _, ok := knownServers[id]; !ok {
			return errUnknownServer
		}
	}
	return nil
}

func save(ctx context.Context, st *alertstore.Store, ruleIDs, serverIDs []int64, mounted bool) error {
	dbCtx, cancel := context.WithTimeout(ctx, config.PGWriteTimeout)
	defer cancel()
	return st.SetRuleMounts(dbCtx, ruleIDs, serverIDs, mounted)
}
