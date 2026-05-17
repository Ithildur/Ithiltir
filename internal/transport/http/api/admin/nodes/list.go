package nodes

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"dash/internal/infra"
	nodestore "dash/internal/store/node"
	"dash/internal/transport/http/httperr"
	appversion "dash/internal/version"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type versionView struct {
	Version    string `json:"version"`
	IsOutdated bool   `json:"is_outdated"`
}

type nodeView struct {
	ID                       int64       `json:"id"`
	Name                     string      `json:"name"`
	Hostname                 string      `json:"hostname"`
	IP                       *string     `json:"ip,omitempty"`
	IsGuestVisible           bool        `json:"is_guest_visible"`
	TrafficP95Enabled        bool        `json:"traffic_p95_enabled"`
	TrafficCycleMode         string      `json:"traffic_cycle_mode"`
	TrafficBillingStartDay   int16       `json:"traffic_billing_start_day"`
	TrafficBillingAnchorDate string      `json:"traffic_billing_anchor_date"`
	TrafficBillingTimezone   string      `json:"traffic_billing_timezone"`
	Secret                   string      `json:"secret"`
	Tags                     []string    `json:"tags"`
	DisplayOrder             int         `json:"display_order"`
	GroupIDs                 []int64     `json:"group_ids"`
	Version                  versionView `json:"version"`
}

func listRoute(r *routes.Blueprint, h *handler) {
	r.Get(
		"/",
		"List nodes",
		routes.Func(h.listHandler),
	)
}

func (h *handler) listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Cache-Control", "no-store")

	nodes, err := loadNodes(ctx, h.store)
	if err != nil {
		httperr.Write(w, http.StatusServiceUnavailable, "db_error", "failed to fetch nodes")
		return
	}
	views, err := nodeViews(nodes)
	if err != nil {
		infra.WithModule("admin.nodes").Error("node tags are invalid", err)
		httperr.Write(w, http.StatusServiceUnavailable, "invalid_node_tags", "invalid node tags")
		return
	}

	response.WriteJSON(w, http.StatusOK, views)
}

func loadNodes(ctx context.Context, st *nodestore.Store) ([]nodestore.NodeItem, error) {
	return infra.WithPGReadTimeout(ctx, func(c context.Context) ([]nodestore.NodeItem, error) {
		nodes, err := st.Nodes(c)
		if err != nil || len(nodes) == 0 {
			return nodes, err
		}

		ids := make([]int64, 0, len(nodes))
		for _, n := range nodes {
			ids = append(ids, n.ID)
		}

		relations, err := st.GroupRelations(c, ids)
		if err != nil {
			return nil, err
		}

		byNode := make(map[int64][]int64, len(nodes))
		for _, rel := range relations {
			byNode[rel.ServerID] = append(byNode[rel.ServerID], rel.GroupID)
		}
		for i := range nodes {
			gids := byNode[nodes[i].ID]
			if gids == nil {
				gids = make([]int64, 0)
			}
			nodes[i].GroupIDs = gids
		}

		return nodes, nil
	})
}

func nodeViews(nodes []nodestore.NodeItem) ([]nodeView, error) {
	if len(nodes) == 0 {
		return make([]nodeView, 0), nil
	}
	out := make([]nodeView, 0, len(nodes))
	for _, n := range nodes {
		version := ""
		if n.AgentVersion != nil {
			version = strings.TrimSpace(*n.AgentVersion)
		}
		tags, err := parseNodeTags(n.Tags)
		if err != nil {
			return nil, fmt.Errorf("node %d tags: %w", n.ID, err)
		}
		out = append(out, nodeView{
			ID:                       n.ID,
			Name:                     n.Name,
			Hostname:                 n.Hostname,
			IP:                       n.IP,
			IsGuestVisible:           n.IsGuestVisible,
			TrafficP95Enabled:        n.TrafficP95Enabled,
			TrafficCycleMode:         n.TrafficCycleMode,
			TrafficBillingStartDay:   n.TrafficBillingStartDay,
			TrafficBillingAnchorDate: n.TrafficBillingAnchorDate,
			TrafficBillingTimezone:   n.TrafficBillingTimezone,
			Secret:                   n.Secret,
			Tags:                     tags,
			DisplayOrder:             n.DisplayOrder,
			GroupIDs:                 n.GroupIDs,
			Version: versionView{
				Version:    version,
				IsOutdated: isVersionOutdated(version),
			},
		})
	}
	return out, nil
}

func isVersionOutdated(version string) bool {
	v := strings.TrimSpace(version)
	if v == "" {
		return true
	}
	outdated, err := appversion.IsNodeOutdated(v)
	if err != nil {
		return true
	}
	return outdated
}
