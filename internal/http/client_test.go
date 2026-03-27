// Generated with Cursor
package http

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
)

func TestNewHTTPClient(t *testing.T) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS13}
	ua := "test-agent/1.0"
	c := NewHTTPClient(tlsCfg, ua)

	if c.userAgent != ua {
		t.Fatalf("userAgent: got %q want %q", c.userAgent, ua)
	}
	if c.Retries != 0 {
		t.Fatalf("Retries: got %d want 0", c.Retries)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type: got %T", c.Transport)
	}
	if tr.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil")
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("TLS MinVersion: got %v want TLS 1.3", tr.TLSClientConfig.MinVersion)
	}
}

func TestClient_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if got := r.Header.Get("User-Agent"); got != "ua-test" {
			t.Errorf("User-Agent: got %q want ua-test", got)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	c := NewHTTPClient(&tls.Config{}, "ua-test")
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("body: got %q", body)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body err=%v", err)
	}
}

func TestClient_Post(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s want POST", r.Method)
		}
		if got := r.Header.Get("User-Agent"); got != "ua-post" {
			t.Errorf("User-Agent: got %q", got)
		}
		if got := r.Header.Get("X-Custom"); got != "trimmed" {
			t.Errorf("X-Custom after trim: got %q want trimmed", got)
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != "payload" {
			t.Errorf("body: got %q", b)
		}
		_, _ = w.Write([]byte("done"))
	}))
	t.Cleanup(srv.Close)

	c := NewHTTPClient(&tls.Config{}, "ua-post")
	resp, err := c.Post(srv.URL, map[string]string{"X-Custom": "  trimmed  "}, []byte("payload"))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if err = resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body err=%v", err)
	}
}

func TestClient_Do_retries_on_timeout(t *testing.T) {
	var calls int
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: context.DeadlineExceeded}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})

	c := NewHTTPClient(&tls.Config{}, "ua")
	c.Transport = rt
	c.Retries = 1

	req, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if calls != 2 {
		t.Fatalf("round trips: got %d want 2", calls)
	}

	if err = resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body err=%v", err)
	}
}

func TestClient_Do_too_many_retries_after_503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	c := NewHTTPClient(&tls.Config{}, "ua")
	c.Retries = 0

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Do(req)
	if err == nil || !strings.Contains(err.Error(), "too many retries") {
		t.Fatalf("expected too many retries error, got %v", err)
	}
}

func TestClient_Do_retry_after_503_then_success(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	c := NewHTTPClient(&tls.Config{}, "ua")
	c.Retries = 2

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if n != 2 {
		t.Fatalf("handler calls: got %d want 2", n)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d", resp.StatusCode)
	}

	if err = resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body err=%v", err)
	}
}

func TestClient_Do_invalid_retry_after(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "not-a-duration")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	c := NewHTTPClient(&tls.Config{}, "ua")
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Do(req)
	if err == nil || !strings.Contains(err.Error(), "cannot parse Retry-After") {
		t.Fatalf("expected Retry-After parse error, got %v", err)
	}
}

func TestClient_GetPlaybook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("playbook-yaml"))
	}))
	t.Cleanup(srv.Close)

	c := NewHTTPClient(&tls.Config{}, "ua")
	got, err := c.GetPlaybook(srv.URL)
	if err != nil {
		t.Fatalf("GetPlaybook: %v", err)
	}
	if string(got) != "playbook-yaml" {
		t.Fatalf("content: got %q", got)
	}
}

func TestClient_PostResults_invalid_url(t *testing.T) {
	c := NewHTTPClient(&tls.Config{}, "ua")
	_, _, _, err := c.PostResults("://bad", nil, nil)
	if err == nil {
		t.Fatal("expected error for bad URL")
	}
}

func TestClient_PostResults_missing_scheme(t *testing.T) {
	c := NewHTTPClient(&tls.Config{}, "ua")
	_, _, _, err := c.PostResults("example.com/path", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no scheme") {
		t.Fatalf("expected no scheme error, got %v", err)
	}
}

func TestClient_PostResults_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Reply", "1")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	orig := config.DefaultConfig
	t.Cleanup(func() { config.DefaultConfig = orig })

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	config.DefaultConfig.DataHost = ""

	c := NewHTTPClient(&tls.Config{}, "ua")
	code, meta, body, err := c.PostResults(srv.URL, map[string]string{"X-Req": "a"}, []byte("{}"))
	if err != nil {
		t.Fatalf("PostResults: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("code: got %d", code)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("body: got %s", body)
	}
	if meta["X-Reply"] != "1" {
		t.Fatalf("metadata X-Reply: got %#v", meta)
	}
	if u.Host == "" {
		t.Fatal("parsed URL host empty")
	}
}

func TestClient_PostResults_data_host_override(t *testing.T) {
	var seenHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	orig := config.DefaultConfig
	t.Cleanup(func() { config.DefaultConfig = orig })

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	config.DefaultConfig.DataHost = u.Host

	c := NewHTTPClient(&tls.Config{}, "ua")
	// URL host is different from DataHost target — PostResults forces DataHost.
	code, _, _, err := c.PostResults("http://other.example.com/submit", nil, []byte("{}"))
	if err != nil {
		t.Fatalf("PostResults: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("code: got %d", code)
	}
	if seenHost != u.Host {
		t.Fatalf("request Host: got %q want %q (DataHost override)", seenHost, u.Host)
	}
}

func TestClient_PostResults_response_metadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("A", "1")
		w.Header().Set("B", "2")
	}))
	t.Cleanup(srv.Close)

	orig := config.DefaultConfig
	t.Cleanup(func() { config.DefaultConfig = orig })
	config.DefaultConfig.DataHost = ""

	c := NewHTTPClient(&tls.Config{}, "ua")
	_, meta, _, err := c.PostResults(srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta["A"] != "1" || meta["B"] != "2" {
		t.Fatalf("metadata A/B: got %#v", meta)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
