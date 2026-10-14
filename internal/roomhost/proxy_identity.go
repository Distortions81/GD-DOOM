package roomhost

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func parseTrustedProxies(values []string) (map[netip.Addr]struct{}, error) {
	if len(values) > 16 {
		return nil, fmt.Errorf("at most 16 trusted proxy addresses are supported")
	}
	trusted := make(map[netip.Addr]struct{}, len(values))
	for _, value := range values {
		ip, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil || ip.Zone() != "" || !ip.IsLoopback() {
			return nil, fmt.Errorf("trusted proxy must be a literal loopback IP address")
		}
		trusted[ip.Unmap()] = struct{}{}
	}
	return trusted, nil
}

// remoteAddress trusts a single forwarded IP only when the socket itself came
// from an explicitly configured local gateway. The gateway must replace, not
// append to, caller-supplied forwarding headers; chains are never accepted.
func (m *Manager) remoteAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || peer.Zone() != "" {
		return host
	}
	peer = peer.Unmap()
	if _, trusted := m.trustedProxies[peer]; !trusted {
		return peer.String()
	}
	values := r.Header.Values("X-Forwarded-For")
	if len(values) != 1 {
		return peer.String()
	}
	forwarded, err := netip.ParseAddr(strings.TrimSpace(values[0]))
	if err != nil || forwarded.Zone() != "" {
		return peer.String()
	}
	return forwarded.Unmap().String()
}
