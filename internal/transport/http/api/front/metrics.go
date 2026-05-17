package front

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
	"dash/internal/metrics"
	"dash/internal/store/frontcache"
	nodestore "dash/internal/store/node"
	systemstore "dash/internal/store/system"
	"dash/internal/transport/http/httperr"
	"dash/internal/transport/http/request"
	authjwt "github.com/Ithildur/EiluneKit/auth/jwt"
	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"
)

type listMode int

const (
	listModeFull listMode = iota
	listModePaged
)

type listInput struct {
	Mode   listMode
	Limit  int
	Offset int
}

type handler struct {
	front         *frontcache.Store
	node          *nodestore.Store
	system        *systemstore.Store
	staleAfterSec int
	auth          *authjwt.Manager
}

func newHandler(front *frontcache.Store, node *nodestore.Store, system *systemstore.Store, offlineThreshold time.Duration, auth *authjwt.Manager) *handler {
	if offlineThreshold <= 0 {
		offlineThreshold = config.DefaultNodeOfflineThreshold
	}
	return &handler{
		front:         front,
		node:          node,
		system:        system,
		staleAfterSec: metrics.DurationSecondsCeil(offlineThreshold),
		auth:          auth,
	}
}

func (h *handler) metricsRoute(r *routes.Blueprint) {
	r.Get(
		"/metrics",
		"List front metrics",
		routes.Func(h.metricsHandler),
		routes.Tags("front"),
		routes.Auth(routes.AuthOptional),
	)
}

func (h *handler) metricsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authorized := h.isAuthorized(r)

	in, err := parseList(r)
	if err != nil {
		httperr.TryWrite(w, httperr.InvalidRequest(err))
		return
	}

	nodes, err := h.fetchNodes(ctx, authorized, in)
	if err != nil {
		httperr.TryWrite(w, httperr.ServiceUnavailable(err))
		return
	}

	if nodes == nil {
		nodes = make([]metrics.NodeView, 0)
	}
	response.WriteJSON(w, http.StatusOK, nodes)
}

func (h *handler) fetchNodes(ctx context.Context, authorized bool, in listInput) ([]metrics.NodeView, error) {
	switch in.Mode {
	case listModeFull:
		nodes, err := h.fetchSnapshot(ctx, authorized)
		if err != nil {
			return nil, err
		}
		sortByOrder(nodes)
		return nodes, nil

	case listModePaged:
		return h.fetchPage(ctx, in.Limit, in.Offset, authorized)

	default:
		return nil, errors.New("invalid front list mode")
	}
}

func (h *handler) fetchSnapshot(ctx context.Context, authorized bool) ([]metrics.NodeView, error) {
	nodes, err := h.front.EnsureSnapshot(ctx, frontcache.FrontSnapshotOptions{
		CacheTimeout:  config.RedisFetchTimeout,
		BuildTimeout:  config.PGReadTimeout,
		StaleAfterSec: h.staleAfterSec,
	})

	if err != nil {
		logCacheWarn("cache ensure", err)
		return nil, err
	}

	if authorized {
		return nodes, nil
	}

	ids, err := nodeIDs(nodes)
	if err != nil {
		return nil, err
	}
	allowed, err := h.front.EnsureGuestVisibleIDs(ctx, ids, frontcache.GuestVisibilityOptions{
		CacheTimeout: config.RedisFetchTimeout,
		BuildTimeout: config.PGReadTimeout,
	})
	if err != nil {
		logCacheWarn("guest visibility cache ensure", err)
		return nil, err
	}
	return filterGuestNodes(nodes, ids, allowed), nil
}

func (h *handler) fetchPage(ctx context.Context, limit, offset int, authorized bool) ([]metrics.NodeView, error) {
	return infra.WithPGReadTimeout(ctx, func(dbCtx context.Context) ([]metrics.NodeView, error) {
		return h.front.FetchFrontNodes(dbCtx, h.staleAfterSec, limit, offset, authorized)
	})
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return 0, errors.New("limit must be a positive integer")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	if n > config.FrontMaxLimit {
		return config.FrontMaxLimit, nil
	}
	return n, nil
}

func nodeIDs(nodes []metrics.NodeView) ([]int64, error) {
	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		id, ok := metrics.ParseNodeID(node.Node.ID)
		if !ok {
			return nil, fmt.Errorf("invalid front node id %q", node.Node.ID)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func filterGuestNodes(nodes []metrics.NodeView, ids []int64, allowed map[int64]struct{}) []metrics.NodeView {
	if len(nodes) == 0 || len(allowed) == 0 {
		return []metrics.NodeView{}
	}
	out := make([]metrics.NodeView, 0, len(nodes))
	for i, id := range ids {
		if _, ok := allowed[id]; ok {
			out = append(out, nodes[i])
		}
	}
	return out
}

func parseOffset(raw string) (int, error) {
	if raw == "" {
		return 0, errors.New("offset must be a non-negative integer")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errors.New("offset must be a non-negative integer")
	}
	if n > 100000 {
		return 100000, nil
	}
	return n, nil
}

func parseList(r *http.Request) (listInput, error) {
	if r == nil {
		return listInput{}, errors.New("request is required")
	}
	q := r.URL.Query()
	rawLimit := q.Get("limit")
	rawOffset := q.Get("offset")
	_, hasLimit := q["limit"]
	_, hasOffset := q["offset"]

	if !hasLimit && !hasOffset {
		return listInput{Mode: listModeFull}, nil
	}

	if hasOffset && !hasLimit {
		return listInput{}, errors.New("limit is required when offset is provided")
	}

	limit, err := parseLimit(rawLimit)
	if err != nil {
		return listInput{}, err
	}

	offset := 0
	if hasOffset {
		offset, err = parseOffset(rawOffset)
		if err != nil {
			return listInput{}, err
		}
	}

	return listInput{
		Mode:   listModePaged,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func sortByOrder(nodes []metrics.NodeView) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Node.Order == nodes[j].Node.Order {
			if nodes[i].Node.Title == nodes[j].Node.Title {
				idI, okI := metrics.ParseNodeID(nodes[i].Node.ID)
				idJ, okJ := metrics.ParseNodeID(nodes[j].Node.ID)
				if okI && okJ {
					return idI < idJ
				}
				return nodes[i].Node.ID < nodes[j].Node.ID
			}
			return nodes[i].Node.Title < nodes[j].Node.Title
		}
		return nodes[i].Node.Order > nodes[j].Node.Order
	})
}

func (h *handler) isAuthorized(r *http.Request) bool {
	return request.HasValidBearer(r, h.auth)
}

func logCacheWarn(action string, err error) {
	if err == nil {
		return
	}
	infra.WithModule("front").Warn("cache issue", err, slog.String("action", action))
}
