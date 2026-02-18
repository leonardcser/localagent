package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"localagent/pkg/logger"
)

type Proxy struct {
	whitelist *Whitelist
	listener  net.Listener
	server    *http.Server
	direct    *http.Transport
}

func New(wl *Whitelist) *Proxy {
	p := &Proxy{
		whitelist: wl,
		direct: &http.Transport{
			Proxy:                 nil, // no proxy — avoids loop
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	p.server = &http.Server{Handler: p}
	return p
}

func (p *Proxy) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("proxy listen: %w", err)
	}
	p.listener = ln
	go p.server.Serve(ln)
	logger.Info("proxy started on %s", p.Addr())
	return nil
}

func (p *Proxy) Addr() string {
	if p.listener == nil {
		return ""
	}
	return "http://" + p.listener.Addr().String()
}

func (p *Proxy) Whitelist() *Whitelist {
	return p.whitelist
}

func (p *Proxy) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return p.server.Shutdown(ctx)
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if !p.whitelist.Allowed(host, "") {
		logger.Info("proxy CONNECT denied: %s", host)
		http.Error(w, "Forbidden by domain whitelist", http.StatusForbidden)
		return
	}

	// Ensure host has a port
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "443")
	}

	target, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		http.Error(w, fmt.Sprintf("dial target: %v", err), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		target.Close()
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		target.Close()
		return
	}

	go transfer(target, client)
	go transfer(client, target)
}

func transfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	path := r.URL.Path

	if !p.whitelist.Allowed(host, path) {
		logger.Info("proxy HTTP denied: %s%s", host, path)
		http.Error(w, "Forbidden by domain whitelist", http.StatusForbidden)
		return
	}

	// Build outgoing request
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("build request: %v", err), http.StatusInternalServerError)
		return
	}
	outReq.Header = r.Header.Clone()

	resp, err := p.direct.RoundTrip(outReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
