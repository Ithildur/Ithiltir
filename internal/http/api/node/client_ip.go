package node

import (
	"net/http"
	"net/netip"

	"dash/internal/http/request"
)

func nodeClientIP(r *http.Request) (netip.Addr, bool) { return request.NodeIP(r) }
