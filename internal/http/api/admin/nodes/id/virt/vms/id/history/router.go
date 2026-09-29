package history

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strconv"

	"github.com/Ithildur/EiluneKit/http/response"
	"github.com/Ithildur/EiluneKit/http/routes"

	"dash/internal/http/httperr"
	"dash/internal/http/request"
	"dash/internal/nodesession"
	nodestore "dash/internal/store/node"
	"dash/internal/virt"
)

type handler struct {
	node     *nodestore.Store
	sessions *nodesession.Hub
}

func Router(node *nodestore.Store, sessions *nodesession.Hub) *routes.Blueprint {
	h := &handler{node: node, sessions: sessions}
	r := routes.NewBlueprint()
	historyRoute(r, h)
	return r
}
func historyRoute(r *routes.Blueprint, h *handler) {
	r.Get("", "Get VM history from PVE", h.historyHandler)
}
func (h *handler) historyHandler(w http.ResponseWriter, r *http.Request, rawID, rawVMID string) {
	id, err := request.ParseIDInt64(rawID)
	if err != nil {
		httperr.Write(w, 400, "invalid_id", "invalid node id")
		return
	}
	vmID, err := strconv.Atoi(rawVMID)
	q := virt.HistoryQuery{VMID: vmID, Timeframe: cmp.Or(r.URL.Query().Get("timeframe"), "hour"), Consolidation: cmp.Or(r.URL.Query().Get("consolidation"), "AVERAGE")}
	if err != nil || q.Validate() != nil {
		httperr.Write(w, 400, "invalid_query", "invalid VM history query")
		return
	}
	exists, err := h.node.NodeExists(r.Context(), id)
	if err != nil {
		httperr.Write(w, 503, "virt_unavailable", "VM query unavailable")
		return
	}
	if !exists {
		httperr.Write(w, 404, "not_found", "node not found")
		return
	}
	raw, err := h.sessions.History(r.Context(), id, q)
	if err != nil {
		code, httpStatus := "source_unavailable", http.StatusBadGateway
		switch {
		case errors.Is(err, nodesession.ErrOffline):
			code, httpStatus = "node_offline", 503
		case errors.Is(err, nodesession.ErrUnsupported):
			code, httpStatus = "unsupported", 501
		case errors.Is(err, nodesession.ErrVMNotFound):
			code, httpStatus = "vm_not_found", 404
		case errors.Is(err, nodesession.ErrBusy):
			code, httpStatus = "busy", 429
		case errors.Is(err, context.DeadlineExceeded):
			code, httpStatus = "deadline_exceeded", 504
		case errors.Is(err, context.Canceled):
			return
		}
		httperr.Write(w, httpStatus, code, code)
		return
	}
	var history virt.History
	if err := json.Unmarshal(raw, &history); err != nil || history.Source != "pve_rrd" || history.VMID != q.VMID || history.Timeframe != q.Timeframe || history.Consolidation != q.Consolidation || history.CollectedAt.IsZero() || history.Points == nil {
		httperr.Write(w, 502, "invalid_result", "invalid VM history result")
		return
	}
	response.WriteJSON(w, http.StatusOK, history)
}
