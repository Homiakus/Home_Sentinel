package netpolicy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

type Guard struct {
	Allowed  []netip.Prefix
	Resolver *net.Resolver
}

func New(cidrs []string) (Guard, error) {
	g := Guard{Resolver: net.DefaultResolver}
	for _, raw := range cidrs {
		p, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return Guard{}, fmt.Errorf("parse allowed CIDR %q: %w", raw, err)
		}
		g.Allowed = append(g.Allowed, p.Masked())
	}
	if len(g.Allowed) == 0 {
		return Guard{}, errors.New("at least one allowed CIDR required")
	}
	return g, nil
}
func (g Guard) ResolveAllowed(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("host required")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !g.allowed(ip) {
			return nil, fmt.Errorf("address %s is outside allowed camera networks", ip)
		}
		return []netip.Addr{ip}, nil
	}
	r := g.Resolver
	if r == nil {
		r = net.DefaultResolver
	}
	ips, err := r.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("host resolved to no addresses")
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		ip = ip.Unmap()
		if !g.allowed(ip) {
			return nil, fmt.Errorf("resolved address %s is outside allowed camera networks", ip)
		}
		out = append(out, ip)
	}
	return out, nil
}
func (g Guard) allowed(ip netip.Addr) bool {
	if !ip.IsValid() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, p := range g.Allowed {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
func (g Guard) ValidateURL(ctx context.Context, raw string, schemes ...string) (*url.URL, []netip.Addr, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, nil, err
	}
	ok := len(schemes) == 0
	for _, s := range schemes {
		if strings.EqualFold(u.Scheme, s) {
			ok = true
			break
		}
	}
	if !ok {
		return nil, nil, fmt.Errorf("URL scheme %q not allowed", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, nil, errors.New("URL host required")
	}
	ips, err := g.ResolveAllowed(ctx, u.Hostname())
	if err != nil {
		return nil, nil, err
	}
	return u, ips, nil
}

// PinURL validates DNS once and rewrites the URL host to the first allowed IP.
// It is intended for local RTSP/HTTP camera probes to avoid a second DNS lookup.
func (g Guard) PinURL(ctx context.Context, raw string, schemes ...string) (string, error) {
	u, ips, err := g.ValidateURL(ctx, raw, schemes...)
	if err != nil {
		return "", err
	}
	port := u.Port()
	host := ips[0].String()
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if ips[0].Is6() {
		host = "[" + host + "]"
	}
	u.Host = host
	return u.String(), nil
}

// DialContext returns a net.Dial-compatible function that resolves the
// requested host through the same allowlist immediately before dialing. It
// preserves the original HTTP Host/TLS server name while pinning the socket to
// an allowed address, avoiding a separate unchecked DNS resolution.
func (g Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := g.ResolveAllowed(ctx, host)
	if err != nil {
		return nil, err
	}
	var last error
	d := net.Dialer{}
	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)
		conn, err := d.DialContext(ctx, network, target)
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = errors.New("no allowed addresses")
	}
	return nil, last
}
