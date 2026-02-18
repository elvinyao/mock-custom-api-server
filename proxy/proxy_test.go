package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// helper: create gin context with a real HTTP request
func buildProxyContext(t *testing.T, method, path, body string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

// ── ProxyRequest – empty target ───────────────────────────────────────────────

func TestProxyRequest_EmptyTarget_ReturnsFalse(t *testing.T) {
	h := New()
	c, _ := buildProxyContext(t, "GET", "/", "", nil)
	ep := config.Endpoint{Proxy: config.ProxyConfig{Target: ""}}
	result := h.ProxyRequest(c, ep)
	if result {
		t.Error("expected false for empty target")
	}
}

// ── ProxyRequest – real upstream ──────────────────────────────────────────────

func TestProxyRequest_ForwardsToUpstream(t *testing.T) {
	// Start a test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"upstream":"ok"}`))
	}))
	defer upstream.Close()

	h := New()
	c, w := buildProxyContext(t, "GET", "/api/test", "", nil)
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{
			Target:    upstream.URL,
			TimeoutMs: 5000,
		},
	}
	result := h.ProxyRequest(c, ep)
	if !result {
		t.Fatal("expected true (request handled)")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "upstream") {
		t.Errorf("expected upstream response, got %q", w.Body.String())
	}
}

func TestProxyRequest_StripPrefix(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h := New()
	c, _ := buildProxyContext(t, "GET", "/api/v1/users", "", nil)
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{
			Target:      upstream.URL,
			StripPrefix: "/api/v1",
		},
	}
	h.ProxyRequest(c, ep)
	if receivedPath != "/users" {
		t.Errorf("receivedPath = %q, want /users", receivedPath)
	}
}

func TestProxyRequest_InjectHeaders(t *testing.T) {
	var receivedHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h := New()
	c, _ := buildProxyContext(t, "GET", "/", "", nil)
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{
			Target: upstream.URL,
			Headers: map[string]string{
				"X-Custom-Header": "injected-value",
			},
		},
	}
	h.ProxyRequest(c, ep)
	if receivedHeader != "injected-value" {
		t.Errorf("X-Custom-Header = %q, want injected-value", receivedHeader)
	}
}

func TestProxyRequest_ForwardsRequestBody(t *testing.T) {
	var receivedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(strings.Builder)
		for {
			b := make([]byte, 512)
			n, err := r.Body.Read(b)
			buf.Write(b[:n])
			if err != nil {
				break
			}
		}
		receivedBody = buf.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h := New()
	c, _ := buildProxyContext(t, "POST", "/", `{"key":"value"}`, map[string]string{"Content-Type": "application/json"})
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{Target: upstream.URL},
	}
	h.ProxyRequest(c, ep)
	if !strings.Contains(receivedBody, "key") {
		t.Errorf("request body not forwarded: %q", receivedBody)
	}
}

func TestProxyRequest_ForwardsOriginalHeaders(t *testing.T) {
	var receivedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h := New()
	c, _ := buildProxyContext(t, "GET", "/", "", map[string]string{"Authorization": "Bearer token123"})
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{Target: upstream.URL},
	}
	h.ProxyRequest(c, ep)
	if receivedAuth != "Bearer token123" {
		t.Errorf("Authorization = %q, want Bearer token123", receivedAuth)
	}
}

func TestProxyRequest_FallbackOnError(t *testing.T) {
	// Use a port that refuses connections
	h := New()
	c, _ := buildProxyContext(t, "GET", "/", "", nil)
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{
			Target:          "http://127.0.0.1:19999",
			FallbackOnError: true,
			TimeoutMs:       100,
		},
	}
	result := h.ProxyRequest(c, ep)
	if result {
		t.Error("expected false (fallback on error)")
	}
}

func TestProxyRequest_NoFallback_Returns502(t *testing.T) {
	h := New()
	c, w := buildProxyContext(t, "GET", "/", "", nil)
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{
			Target:          "http://127.0.0.1:19999",
			FallbackOnError: false,
			TimeoutMs:       100,
		},
	}
	result := h.ProxyRequest(c, ep)
	if !result {
		t.Error("expected true (error handled, not fallback)")
	}
	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestProxyRequest_RecordsStub(t *testing.T) {
	dir := t.TempDir()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"recorded":true}`))
	}))
	defer upstream.Close()

	h := New()
	c, _ := buildProxyContext(t, "GET", "/record/test", "", nil)
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{
			Target:    upstream.URL,
			Record:    true,
			RecordDir: dir,
		},
	}
	h.ProxyRequest(c, ep)

	// Check that stub files were created
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("could not read record dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected stub files to be created in record dir")
	}
}

func TestProxyRequest_StripPrefixToRoot(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h := New()
	c, _ := buildProxyContext(t, "GET", "/api", "", nil)
	ep := config.Endpoint{
		Proxy: config.ProxyConfig{
			Target:      upstream.URL,
			StripPrefix: "/api",
		},
	}
	h.ProxyRequest(c, ep)
	if receivedPath != "/" {
		t.Errorf("receivedPath = %q, want / (strip to root)", receivedPath)
	}
}

// ── NewReverseProxy ───────────────────────────────────────────────────────────

func TestNewReverseProxy_ValidURL(t *testing.T) {
	rp, err := NewReverseProxy("http://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp == nil {
		t.Error("expected non-nil reverse proxy")
	}
}

func TestNewReverseProxy_InvalidURL(t *testing.T) {
	_, err := NewReverseProxy("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// ── StubWriter ────────────────────────────────────────────────────────────────

func TestStubWriter_WriteStub_CreatesFiles(t *testing.T) {
	dir := t.TempDir()

	req, _ := http.NewRequest("POST", "/api/orders", strings.NewReader(`{"id":1}`))
	resp := &http.Response{
		StatusCode: 201,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	sw := StubWriter{}
	sw.WriteStub(dir, req, resp, []byte(`{"id":1}`), []byte(`{"created":true}`))

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("could not read dir: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 files (yaml + json), got %d", len(entries))
	}

	// Verify one yaml and one json
	var hasYAML, hasJSON bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			hasYAML = true
		}
		if strings.HasSuffix(e.Name(), ".json") {
			hasJSON = true
		}
	}
	if !hasYAML {
		t.Error("expected .yaml stub file")
	}
	if !hasJSON {
		t.Error("expected .json response file")
	}
}

func TestStubWriter_WriteStub_RootPath(t *testing.T) {
	dir := t.TempDir()

	req, _ := http.NewRequest("GET", "/", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
	}

	sw := StubWriter{}
	sw.WriteStub(dir, req, resp, nil, []byte(`{}`))

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Error("expected files to be created for root path")
	}

	// Should use "root" for the path part
	for _, e := range entries {
		if strings.Contains(e.Name(), "root") {
			return
		}
	}
	t.Error("expected 'root' in filename for / path")
}

func TestStubWriter_WriteStub_InvalidDir(t *testing.T) {
	// Should not panic on invalid dir (e.g., file path instead of dir)
	f := filepath.Join(t.TempDir(), "blocking_file")
	os.WriteFile(f, []byte("x"), 0444)
	// Make a dir that we can't write to by using the file as the dir
	sw := StubWriter{}

	req, _ := http.NewRequest("GET", "/test", nil)
	resp := &http.Response{StatusCode: 200, Header: http.Header{}}
	// Should not panic
	sw.WriteStub(f, req, resp, nil, nil)
}

func TestStubWriter_WriteStub_YAMLContent(t *testing.T) {
	dir := t.TempDir()

	req, _ := http.NewRequest("GET", "/api/users", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	sw := StubWriter{}
	sw.WriteStub(dir, req, resp, nil, []byte(`[{"id":1}]`))

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			if !strings.Contains(content, "/api/users") {
				t.Errorf("YAML stub doesn't contain path /api/users: %q", content)
			}
			return
		}
	}
	t.Error("no .yaml file found")
}
