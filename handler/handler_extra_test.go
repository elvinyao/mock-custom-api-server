package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ── matchPath ─────────────────────────────────────────────────────────────────

func TestMatchPath_ExactMatch(t *testing.T) {
	params, ok := matchPath("/api/users", "/api/users")
	if !ok {
		t.Fatal("expected exact match")
	}
	if len(params) != 0 {
		t.Errorf("expected no path params, got %v", params)
	}
}

func TestMatchPath_NoMatch(t *testing.T) {
	_, ok := matchPath("/api/users", "/api/orders")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestMatchPath_DifferentSegmentCount(t *testing.T) {
	_, ok := matchPath("/api/users", "/api/users/1")
	if ok {
		t.Fatal("expected no match for different segment count")
	}
}

func TestMatchPath_PathParam(t *testing.T) {
	params, ok := matchPath("/api/users/:id", "/api/users/42")
	if !ok {
		t.Fatal("expected match with path param")
	}
	if params["id"] != "42" {
		t.Errorf("expected id=42, got %v", params)
	}
}

func TestMatchPath_MultiplePathParams(t *testing.T) {
	params, ok := matchPath("/api/:resource/:id", "/api/orders/99")
	if !ok {
		t.Fatal("expected match")
	}
	if params["resource"] != "orders" || params["id"] != "99" {
		t.Errorf("unexpected params: %v", params)
	}
}

func TestMatchPath_CatchAll_Named(t *testing.T) {
	params, ok := matchPath("/static/*filepath", "/static/css/main.css")
	if !ok {
		t.Fatal("expected catch-all match")
	}
	if len(params) != 0 {
		t.Logf("params: %v (catch-all returns empty params)", params)
	}
}

func TestMatchPath_CatchAll_Root(t *testing.T) {
	_, ok := matchPath("/api/*path", "/api/")
	if !ok {
		t.Fatal("expected match for root of catch-all")
	}
}

func TestMatchPath_CatchAll_NoMatch(t *testing.T) {
	_, ok := matchPath("/api/*path", "/other/something")
	if ok {
		t.Fatal("expected no match when prefix differs")
	}
}

func TestMatchPath_StaticMismatch(t *testing.T) {
	_, ok := matchPath("/api/users/:id/orders", "/api/users/1/profile")
	if ok {
		t.Fatal("expected no match when static segment differs")
	}
}

// ── getRuleIndex ─────────────────────────────────────────────────────────────

func TestGetRuleIndex_Found(t *testing.T) {
	rules := []Rule{
		{ResponseFile: "a.json"},
		{ResponseFile: "b.json"},
		{ResponseFile: "c.json"},
	}
	idx := getRuleIndex(rules, &rules[1])
	if idx != 1 {
		t.Errorf("expected index 1, got %d", idx)
	}
}

func TestGetRuleIndex_NotFound(t *testing.T) {
	rules := []Rule{{ResponseFile: "a.json"}}
	other := Rule{ResponseFile: "other.json"}
	idx := getRuleIndex(rules, &other)
	if idx != -1 {
		t.Errorf("expected -1 for not found, got %d", idx)
	}
}

// ── handleNotFound ────────────────────────────────────────────────────────────

func TestHandleNotFound_DefaultJSON(t *testing.T) {
	h := NewMockHandler(buildTestConfigManager(nil))
	r := gin.New()
	r.NoRoute(func(c *gin.Context) {
		h.handleNotFound(c, h.configManager.GetConfig())
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/not-a-real-path", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleNotFound_CustomFile(t *testing.T) {
	tmp := writeTempFile(t, `{"error":"custom 404"}`)
	defer os.Remove(tmp)

	cfg := &config.Config{
		ErrorHandling: config.ErrorHandling{
			CustomErrorResponses: map[int]string{404: tmp},
		},
	}
	h := NewMockHandler(buildTestConfigManager(cfg))
	r := gin.New()
	r.NoRoute(func(c *gin.Context) {
		h.handleNotFound(c, cfg)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/missing", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("custom 404")) {
		t.Errorf("expected custom 404 body, got %s", w.Body.String())
	}
}

// ── handleError ───────────────────────────────────────────────────────────────

func TestHandleError_DefaultJSON(t *testing.T) {
	h := NewMockHandler(buildTestConfigManager(nil))
	r := gin.New()
	r.GET("/fail", func(c *gin.Context) {
		h.handleError(c, h.configManager.GetConfig(), io.ErrUnexpectedEOF)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fail", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleError_ShowDetails(t *testing.T) {
	cfg := &config.Config{
		ErrorHandling: config.ErrorHandling{ShowDetails: true},
	}
	h := NewMockHandler(buildTestConfigManager(cfg))
	r := gin.New()
	r.GET("/fail", func(c *gin.Context) {
		h.handleError(c, cfg, io.ErrUnexpectedEOF)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fail", nil)
	r.ServeHTTP(w, req)

	if !bytes.Contains(w.Body.Bytes(), []byte("unexpected EOF")) {
		t.Errorf("expected error details in body, got %s", w.Body.String())
	}
}

func TestHandleError_CustomFile(t *testing.T) {
	tmp := writeTempFile(t, `{"error":"custom 500"}`)
	defer os.Remove(tmp)

	cfg := &config.Config{
		ErrorHandling: config.ErrorHandling{
			CustomErrorResponses: map[int]string{500: tmp},
		},
	}
	h := NewMockHandler(buildTestConfigManager(cfg))
	r := gin.New()
	r.GET("/fail", func(c *gin.Context) {
		h.handleError(c, cfg, io.ErrUnexpectedEOF)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fail", nil)
	r.ServeHTTP(w, req)

	if !bytes.Contains(w.Body.Bytes(), []byte("custom 500")) {
		t.Errorf("expected custom 500 body, got %s", w.Body.String())
	}
}

// ── HealthHandler ─────────────────────────────────────────────────────────────

func TestHealthHandler_ReturnsHealthy(t *testing.T) {
	cm := buildTestConfigManager(&config.Config{
		Endpoints: []config.Endpoint{{Path: "/a", Method: "GET"}},
	})

	r := gin.New()
	r.GET("/health", HealthHandler(cm))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("healthy")) {
		t.Errorf("expected 'healthy' in response, got %s", w.Body.String())
	}
}

// ── MockHandler full request flow ─────────────────────────────────────────────

func TestHandleRequest_MatchesRule(t *testing.T) {
	respFile := writeTempFile(t, `{"result":"ok"}`)
	defer os.Remove(respFile)

	ep := config.Endpoint{
		Path:   "/api/test",
		Method: "GET",
		Default: config.ResponseConfig{
			ResponseFile: respFile,
			StatusCode:   200,
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("result")) {
		t.Errorf("expected response file content, got %s", w.Body.String())
	}
}

func TestHandleRequest_NoMatch_404(t *testing.T) {
	h, r := buildHandlerRouter(t, nil)
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/not-registered", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleRequest_WithSelector_MatchesRule(t *testing.T) {
	vipFile := writeTempFile(t, `{"tier":"vip"}`)
	regFile := writeTempFile(t, `{"tier":"regular"}`)
	defer os.Remove(vipFile)
	defer os.Remove(regFile)

	ep := config.Endpoint{
		Path:   "/api/users",
		Method: "GET",
		Selectors: []config.Selector{
			{Name: "user_type", Type: "header", Key: "X-User-Type"},
		},
		Rules: []config.Rule{
			{
				Conditions: []config.Condition{
					{Selector: "user_type", MatchType: "exact", Value: "vip"},
				},
				ResponseConfig: config.ResponseConfig{ResponseFile: vipFile, StatusCode: 200},
			},
		},
		Default: config.ResponseConfig{ResponseFile: regFile, StatusCode: 200},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	// VIP request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("X-User-Type", "vip")
	r.ServeHTTP(w, req)
	if !bytes.Contains(w.Body.Bytes(), []byte("vip")) {
		t.Errorf("expected vip response, got %s", w.Body.String())
	}

	// Regular request
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/api/users", nil)
	r.ServeHTTP(w2, req2)
	if !bytes.Contains(w2.Body.Bytes(), []byte("regular")) {
		t.Errorf("expected regular response, got %s", w2.Body.String())
	}
}

func TestHandleRequest_ProxyMode_UsesUpstream(t *testing.T) {
	// Start a fake upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"from":"upstream"}`))
	}))
	defer upstream.Close()

	ep := config.Endpoint{
		Path:   "/proxy/*path",
		Method: "ANY",
		Mode:   "proxy",
		Proxy: config.ProxyConfig{
			Target:      upstream.URL,
			StripPrefix: "/proxy",
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/proxy/data", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200 from proxy, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("upstream")) {
		t.Errorf("expected upstream response, got %s", w.Body.String())
	}
}

func TestHandleRequest_ProxyMode_FallbackOnError(t *testing.T) {
	fallbackFile := writeTempFile(t, `{"source":"fallback"}`)
	defer os.Remove(fallbackFile)

	ep := config.Endpoint{
		Path:   "/resilient",
		Method: "GET",
		Mode:   "proxy",
		Proxy: config.ProxyConfig{
			Target:          "http://127.0.0.1:19999", // unreachable
			FallbackOnError: true,
		},
		Default: config.ResponseConfig{
			ResponseFile: fallbackFile,
			StatusCode:   200,
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/resilient", nil)
	r.ServeHTTP(w, req)

	if !bytes.Contains(w.Body.Bytes(), []byte("fallback")) {
		t.Errorf("expected fallback response, got %s", w.Body.String())
	}
}

func TestHandleRequest_NilConfig_Returns500(t *testing.T) {
	cm := config.NewConfigManager("nonexistent.yaml")
	// Don't call SetConfig — config is nil
	h := NewMockHandler(cm)
	r := gin.New()
	r.GET("/test", h.handleRequest)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 with nil config, got %d", w.Code)
	}
}

func TestHandleRequest_ContentTypeHeader(t *testing.T) {
	respFile := writeTempFile(t, `plain text response`)
	defer os.Remove(respFile)

	ep := config.Endpoint{
		Path:   "/text",
		Method: "GET",
		Default: config.ResponseConfig{
			ResponseFile: respFile,
			StatusCode:   200,
			ContentType:  "text/plain",
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/text", nil)
	r.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if !bytes.Contains([]byte(ct), []byte("text/plain")) {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestHandleRequest_DelayApplied(t *testing.T) {
	respFile := writeTempFile(t, `{}`)
	defer os.Remove(respFile)

	ep := config.Endpoint{
		Path:   "/slow",
		Method: "GET",
		Default: config.ResponseConfig{
			ResponseFile: respFile,
			StatusCode:   200,
			DelayMs:      20,
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	start := time.Now()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/slow", nil)
	r.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if elapsed < 15*time.Millisecond {
		t.Errorf("expected >=15ms delay, got %v", elapsed)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildTestConfigManager(cfg *config.Config) *config.ConfigManager {
	cm := config.NewConfigManager("test.yaml")
	if cfg == nil {
		cfg = &config.Config{}
	}
	cm.SetConfig(cfg)
	return cm
}

func buildHandlerRouter(t *testing.T, endpoints []config.Endpoint) (*MockHandler, *gin.Engine) {
	t.Helper()
	cfg := &config.Config{Endpoints: endpoints}
	cm := buildTestConfigManager(cfg)
	h := NewMockHandler(cm)
	r := gin.New()
	return h, r
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "handler_test_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return f.Name()
}
