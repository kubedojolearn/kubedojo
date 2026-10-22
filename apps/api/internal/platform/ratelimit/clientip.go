package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIPResolver determines the client address. X-Forwarded-For is honored only when the
// direct peer is a trusted proxy (the edge), and then the rightmost untrusted hop is used, so
// clients cannot spoof their address by sending the header themselves.
type ClientIPResolver struct {
	trusted []netip.Prefix
}

// NewClientIPResolver parses TRUSTED_PROXY_CIDRS (comma-separated, may be empty).
func NewClientIPResolver(cidrs string) (*ClientIPResolver, error) {
	r := &ClientIPResolver{}
	for _, c := range strings.Split(cidrs, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS: invalid CIDR %q", c)
		}
		r.trusted = append(r.trusted, p.Masked())
	}
	return r, nil
}

func (c *ClientIPResolver) isTrusted(a netip.Addr) bool {
	for _, p := range c.trusted {
		if p.Contains(a.Unmap()) {
			return true
		}
	}
	return false
}

// ClientIP returns the best-known client address for r.
func (c *ClientIPResolver) ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	if !c.isTrusted(peer) {
		return peer.Unmap().String()
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		if !c.isTrusted(a) {
			return a.Unmap().String()
		}
	}
	return peer.Unmap().String()
}
