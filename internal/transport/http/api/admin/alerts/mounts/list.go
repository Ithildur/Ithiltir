package mounts

import (
	"context"
	"net/http"

	"dash/internal/alertspec"
	"dash/internal/infra"
	"dash/internal/model"
	alertstore "dash/internal/store/alert"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type listView struct {
	Rules []ruleView `json:"rules"`
	Nodes []nodeView `json:"nodes"`
}

type ruleView struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Metric         string `json:"metric"`
	Builtin        bool   `json:"builtin"`
	Enabled        bool   `json:"enabled"`
	DefaultMounted bool   `json:"default_mounted"`
}

type nodeView struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Hostname string      `json:"hostname"`
	IP       *string     `json:"ip,omitempty"`
	GroupIDs []int64     `json:"group_ids"`
	Mounts   []mountView `json:"mounts"`
}

type mountView struct {
	RuleID  int64 `json:"rule_id"`
	Mounted bool  `json:"mounted"`
}

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List alert rule mounts",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	out, err := loadMounts(r.Context(), h.alert, h.node)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch alert mounts")
		return
	}
	response.WriteJSON(w, http.StatusOK, out)
}

func loadMounts(ctx context.Context, alert *alertstore.Store, nodeStore *nodestore.Store) (listView, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) (listView, error) {
		rules, err := alert.ListRules(c)
		if err != nil {
			return listView{}, err
		}
		nodes, err := nodeStore.Nodes(c)
		if err != nil {
			return listView{}, err
		}
		mounts, err := alert.ListRuleMounts(c)
		if err != nil {
			return listView{}, err
		}
		if len(nodes) > 0 {
			ids := make([]int64, 0, len(nodes))
			for _, item := range nodes {
				ids = append(ids, item.ID)
			}
			relations, err := nodeStore.GroupRelations(c, ids)
			if err != nil {
				return listView{}, err
			}
			applyGroups(nodes, relations)
		}
		return buildMountsView(rules, nodes, mounts), nil
	})
}

func buildMountsView(rules []alertstore.AlertRuleItem, nodes []nodestore.NodeItem, mounts []model.AlertRuleMount) listView {
	ruleViews := make([]ruleView, 0, len(rules)+len(alertspec.BuiltinRules()))
	for _, rule := range alertspec.BuiltinRules() {
		ruleViews = append(ruleViews, ruleView{
			ID:             rule.ID,
			Name:           rule.Name,
			Metric:         rule.Metric,
			Builtin:        true,
			Enabled:        true,
			DefaultMounted: true,
		})
	}
	for _, rule := range rules {
		ruleViews = append(ruleViews, ruleView{
			ID:             rule.ID,
			Name:           rule.Name,
			Metric:         rule.Metric,
			Builtin:        false,
			Enabled:        rule.Enabled,
			DefaultMounted: false,
		})
	}

	byNode := make(map[int64]map[int64]bool)
	for _, mount := range mounts {
		if byNode[mount.ServerID] == nil {
			byNode[mount.ServerID] = make(map[int64]bool)
		}
		byNode[mount.ServerID][mount.RuleID] = mount.Enabled
	}

	nodeViews := make([]nodeView, 0, len(nodes))
	for _, nodeItem := range nodes {
		states := byNode[nodeItem.ID]
		nodeMounts := make([]mountView, 0, len(ruleViews))
		for _, rule := range ruleViews {
			mounted := rule.DefaultMounted
			if enabled, ok := states[rule.ID]; ok {
				mounted = enabled
			}
			nodeMounts = append(nodeMounts, mountView{
				RuleID:  rule.ID,
				Mounted: mounted,
			})
		}
		nodeViews = append(nodeViews, nodeView{
			ID:       nodeItem.ID,
			Name:     nodeItem.Name,
			Hostname: nodeItem.Hostname,
			IP:       nodeItem.IP,
			GroupIDs: nodeItem.GroupIDs,
			Mounts:   nodeMounts,
		})
	}
	return listView{Rules: ruleViews, Nodes: nodeViews}
}

func applyGroups(nodes []nodestore.NodeItem, relations []model.ServerGroup) {
	byNode := make(map[int64][]int64, len(nodes))
	for _, rel := range relations {
		byNode[rel.ServerID] = append(byNode[rel.ServerID], rel.GroupID)
	}
	for i := range nodes {
		nodes[i].GroupIDs = byNode[nodes[i].ID]
		if nodes[i].GroupIDs == nil {
			nodes[i].GroupIDs = make([]int64, 0)
		}
	}
}
