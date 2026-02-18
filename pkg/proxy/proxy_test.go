package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestWhitelist_ExactDomain(t *testing.T) {
	wl := NewWhitelist()
	wl.Add("example.com")

	if !wl.Allowed("example.com", "/") {
		t.Error("expected example.com to be allowed")
	}
	if wl.Allowed("other.com", "/") {
		t.Error("expected other.com to be denied")
	}
}

func TestWhitelist_WildcardSubdomain(t *testing.T) {
	wl := NewWhitelist()
	wl.Add("*.example.com")

	if !wl.Allowed("api.example.com", "/") {
		t.Error("expected api.example.com to be allowed")
	}
	if !wl.Allowed("deep.sub.example.com", "/") {
		t.Error("expected deep.sub.example.com to be allowed")
	}
	if !wl.Allowed("example.com", "/") {
		t.Error("expected example.com itself to be allowed with wildcard")
	}
	if wl.Allowed("notexample.com", "/") {
		t.Error("expected notexample.com to be denied")
	}
}

func TestWhitelist_PathPrefix(t *testing.T) {
	wl := NewWhitelist()
	wl.Add("api.example.com/v1/*")

	if !wl.Allowed("api.example.com", "/v1/users") {
		t.Error("expected /v1/users to be allowed")
	}
	if !wl.Allowed("api.example.com", "/v1/") {
		t.Error("expected /v1/ to be allowed")
	}
	if wl.Allowed("api.example.com", "/v2/users") {
		t.Error("expected /v2/users to be denied")
	}
}

func TestWhitelist_PortMatching(t *testing.T) {
	wl := NewWhitelist()
	wl.Add("api.example.com:8443")

	if !wl.Allowed("api.example.com:8443", "/") {
		t.Error("expected port 8443 to be allowed")
	}
	if wl.Allowed("api.example.com:443", "/") {
		t.Error("expected port 443 to be denied")
	}
	// No port in request — pattern has port, request doesn't — no match
	if wl.Allowed("api.example.com", "/") {
		t.Error("expected no-port request to be denied when pattern has port")
	}
}

func TestWhitelist_PrivateIPBypass(t *testing.T) {
	wl := NewWhitelist() // empty whitelist

	cases := []string{
		"localhost",
		"127.0.0.1",
		"10.0.0.5",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.1.1",
		"::1",
	}
	for _, host := range cases {
		if !wl.Allowed(host, "/") {
			t.Errorf("expected %s to be allowed (private)", host)
		}
	}
}

func TestWhitelist_PublicIPDenied(t *testing.T) {
	wl := NewWhitelist()
	if wl.Allowed("8.8.8.8", "/") {
		t.Error("expected 8.8.8.8 to be denied on empty whitelist")
	}
}

func TestWhitelist_SchemeStripped(t *testing.T) {
	wl := NewWhitelist()
	wl.Add("https://example.com/api/*")

	if !wl.Allowed("example.com", "/api/test") {
		t.Error("expected pattern with scheme to work after stripping")
	}
}

func TestProxy_HTTP_Allowed(t *testing.T) {
	// Backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)

	wl := NewWhitelist()
	wl.Add(backendURL.Host)

	p := New(wl)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop(nil)

	proxyURL, _ := url.Parse(p.Addr())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get(backend.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProxy_HTTP_Denied(t *testing.T) {
	wl := NewWhitelist() // empty — deny all non-private

	p := New(wl)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop(nil)

	proxyURL, _ := url.Parse(p.Addr())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get("http://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestProxy_CONNECT_Allowed(t *testing.T) {
	// TLS backend
	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("TLS OK"))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)

	wl := NewWhitelist()
	wl.Add(backendURL.Host)

	p := New(wl)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop(nil)

	proxyURL, _ := url.Parse(p.Addr())
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(backend.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProxy_CONNECT_Denied(t *testing.T) {
	wl := NewWhitelist() // empty

	p := New(wl)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop(nil)

	proxyURL, _ := url.Parse(p.Addr())
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	_, err := client.Get("https://example.com/")
	if err == nil {
		t.Error("expected error for denied CONNECT")
	}
}

func TestProxy_LocalhostBypass(t *testing.T) {
	// Backend on localhost
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "local")
	}))
	defer backend.Close()

	wl := NewWhitelist() // empty — but localhost is always allowed

	p := New(wl)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop(nil)

	proxyURL, _ := url.Parse(p.Addr())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for localhost, got %d", resp.StatusCode)
	}
}
