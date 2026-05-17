package mounts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"dash/internal/alertspec"
	"dash/internal/config"
	"dash/internal/infra"
	alertstore "dash/internal/store/alert"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	"github.com/Ithildur/EiluneKit/contextutil"
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
		routes.Func(h.replaceHandler),
		routes.Use(middleware.RequireJSONBody),
	)
}

var (
	errUnknownRule   = errors.New("rule_ids contains unknown rule")
	errUnknownServer = errors.New("server_ids contains unknown node")
)

func (h *handler) replaceHandler(w http.ResponseWriter, r *http.Request) {
	const warningCode = "alert_reconcile_delayed"

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
	warning := ""
	if err := markDirty(r.Context(), h.alert, serverIDs); err != nil {
		infra.WithModule("admin.alerts").Warn("alert reconcile queue update failed after mount replace", err,
			slog.Int("rule_count", len(ruleIDs)),
			slog.Int("server_count", len(serverIDs)),
		)
		if fallbackErr := enqueueFullReconcile(r.Context(), h.alert); fallbackErr != nil {
			infra.WithModule("admin.alerts").Warn("alert full reconcile fallback enqueue failed after mount replace", fallbackErr,
				slog.Int("rule_count", len(ruleIDs)),
				slog.Int("server_count", len(serverIDs)),
			)
		}
		warning = warningCode
	}
	if warning != "" {
		httperr.WriteWarningHeader(w, warning)
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
	_, err := infra.WithPGReadTimeout(ctx, func(c context.Context) (struct{}, error) {
		rules, err := alert.ListRules(c)
		if err != nil {
			return struct{}{}, err
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
				return struct{}{}, errUnknownRule
			}
		}

		nodes, err := node.Nodes(c)
		if err != nil {
			return struct{}{}, err
		}
		knownServers := make(map[int64]struct{}, len(nodes))
		for _, node := range nodes {
			knownServers[node.ID] = struct{}{}
		}
		for _, id := range serverIDs {
			if _, ok := knownServers[id]; !ok {
				return struct{}{}, errUnknownServer
			}
		}
		return struct{}{}, nil
	})
	return err
}

func save(ctx context.Context, st *alertstore.Store, ruleIDs, serverIDs []int64, mounted bool) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.SetRuleMounts(c, ruleIDs, serverIDs, mounted)
	})
	return err
}

func markDirty(ctx context.Context, st *alertstore.Store, serverIDs []int64) error {
	_, err := contextutil.WithTimeout(ctx, config.RedisWriteTimeout, func(c context.Context) (struct{}, error) {
		for _, id := range serverIDs {
			if err := st.MarkServerDirty(c, id); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	return err
}

func enqueueFullReconcile(ctx context.Context, st *alertstore.Store) error {
	_, err := infra.WithPGWriteTimeout(ctx, func(c context.Context) (struct{}, error) {
		return struct{}{}, st.EnqueueFullReconcileTask(c, "full_reconcile:global")
	})
	return err
}
