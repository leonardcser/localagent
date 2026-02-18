package proxy

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

type Pattern struct {
	domain   string // lowercase, port stripped
	port     string // empty = any port
	wildcard bool   // *.example.com
	pathPfx  string // path prefix, empty = any
}

type Whitelist struct {
	mu       sync.RWMutex
	patterns []Pattern
}

func NewWhitelist() *Whitelist {
	return &Whitelist{}
}

func (wl *Whitelist) Add(patterns ...string) {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	for _, raw := range patterns {
		if raw == "" {
			continue
		}
		wl.patterns = append(wl.patterns, parsePattern(raw))
	}
}

func parsePattern(raw string) Pattern {
	p := Pattern{}
	raw = strings.ToLower(strings.TrimSpace(raw))

	// Strip scheme if present
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}

	// Split host from path
	hostPart := raw
	if i := strings.Index(raw, "/"); i >= 0 {
		hostPart = raw[:i]
		pathPart := raw[i:]
		// Strip trailing wildcard from path prefix: /v1/* -> /v1/
		p.pathPfx = strings.TrimSuffix(pathPart, "*")
	}

	// Check wildcard
	if strings.HasPrefix(hostPart, "*.") {
		p.wildcard = true
		hostPart = hostPart[2:]
	}

	// Split host:port
	if host, port, err := net.SplitHostPort(hostPart); err == nil {
		p.domain = host
		p.port = port
	} else {
		p.domain = hostPart
	}

	return p
}

func (wl *Whitelist) Allowed(host, path string) bool {
	host = strings.ToLower(host)

	// Split host:port
	hostOnly, port, err := net.SplitHostPort(host)
	if err != nil {
		hostOnly = host
		port = ""
	}

	// Localhost and private IPs are always allowed
	if isPrivate(hostOnly) {
		return true
	}

	wl.mu.RLock()
	defer wl.mu.RUnlock()

	for _, p := range wl.patterns {
		if !matchDomain(hostOnly, p) {
			continue
		}
		if p.port != "" && p.port != port {
			continue
		}
		if p.pathPfx != "" && !strings.HasPrefix(path, p.pathPfx) {
			continue
		}
		return true
	}

	return false
}

func matchDomain(host string, p Pattern) bool {
	if p.wildcard {
		return host == p.domain || strings.HasSuffix(host, "."+p.domain)
	}
	return host == p.domain
}

func isPrivate(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	privateRanges := []struct {
		network string
		mask    int
	}{
		{"10.0.0.0", 8},
		{"172.16.0.0", 12},
		{"192.168.0.0", 16},
	}
	for _, r := range privateRanges {
		_, cidr, _ := net.ParseCIDR(fmt.Sprintf("%s/%d", r.network, r.mask))
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func (wl *Whitelist) List() []string {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	out := make([]string, 0, len(wl.patterns))
	for _, p := range wl.patterns {
		s := ""
		if p.wildcard {
			s = "*."
		}
		s += p.domain
		if p.port != "" {
			s += ":" + p.port
		}
		if p.pathPfx != "" {
			s += p.pathPfx + "*"
		}
		out = append(out, s)
	}
	return out
}
