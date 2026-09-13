package node

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func nodeClientIP(r *http.Request) (netip.Addr, bool) {
	if r == nil {
		return netip.Addr{}, false
	}
	raw := r.Header.Get("X-Forwarded-For")
	if raw == "" {
		return nodeRemoteIP(r.RemoteAddr)
	}
	first, _, _ := strings.Cut(raw, ",")
	first = strings.TrimSpace(first)
	if first == "" {
		return netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(first)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip, true
}

func nodeRemoteIP(remoteAddr string) (netip.Addr, bool) {
	remote := strings.TrimSpace(remoteAddr)
	if remote == "" {
		return netip.Addr{}, false
	}
	if ip, err := netip.ParseAddr(remote); err == nil {
		return ip, true
	}
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip, true
}
