package config

import (
	"fmt"
	"net/netip"
	"strings"
)

func (c HTTPConfig) EffectiveTrustedProxies() ([]netip.Prefix, error) {
	if len(c.TrustedProxyPrefixes) > 0 {
		return append([]netip.Prefix(nil), c.TrustedProxyPrefixes...), nil
	}
	if len(c.TrustedProxies) == 0 {
		return nil, nil
	}

	out := make([]netip.Prefix, 0, len(c.TrustedProxies))
	for i, raw := range c.TrustedProxies {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("http.trusted_proxies[%d]: %w", i, err)
		}
		out = append(out, prefix.Masked())
	}
	return out, nil
}
