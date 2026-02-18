package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mock-api-server/config"
	"mock-api-server/metrics"
	"mock-api-server/recorder"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func doRequest(router *gin.Engine, method, path string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	router.ServeHTTP(w, req)
	return w
}

func doRequestWithHeaders(router *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	router.ServeHTTP(w, req)
	return w
}

// ── CORS ─────────────────────────────────────────────────────────────────────

func newCORSRouter(cfg config.CORSConfig) *gin.Engine {
	r := gin.New()
	r.Use(CORS(cfg))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })
	r.OPTIONS("/test", func(c *gin.Context) { c.Status(200) })
	return r
}

func TestCORS_Disabled_NoHeaders(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: false})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS header when disabled")
	}
}

func TestCORS_NoOriginHeader_NoHeaders(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}})
	w := doRequest(r, "GET", "/test", nil)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS header when no Origin present")
	}
}

func TestCORS_WildcardOrigin_NoCredentials(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
}

func TestCORS_WildcardOrigin_WithCredentials_EchoesOrigin(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://example.com" {
		t.Errorf("Allow-Origin = %q, want http://example.com (echoed)", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
}

func TestCORS_SpecificOrigin_Matches(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"http://allowed.com"},
	})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://allowed.com"})
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://allowed.com" {
		t.Errorf("Allow-Origin = %q, want http://allowed.com", got)
	}
}

func TestCORS_SpecificOrigin_NoMatch_NoHeaders(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"http://allowed.com"},
	})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://evil.com"})
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin for unallowed origin, got %q", got)
	}
	if w.Code != 200 {
		t.Errorf("expected 200 (request continues), got %d", w.Code)
	}
}

func TestCORS_Preflight_Returns204(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}})
	w := doRequestWithHeaders(r, "OPTIONS", "/test", map[string]string{"Origin": "http://example.com"})
	if w.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", w.Code)
	}
}

func TestCORS_ExposedHeaders(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		ExposedHeaders: []string{"X-Custom-Header", "X-Other"},
	})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	exposed := w.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposed, "X-Custom-Header") {
		t.Errorf("Expose-Headers = %q, should contain X-Custom-Header", exposed)
	}
}

func TestCORS_NoExposedHeaders_NoExposedHeader(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	if got := w.Header().Get("Access-Control-Expose-Headers"); got != "" {
		t.Errorf("expected no Expose-Headers header, got %q", got)
	}
}

func TestCORS_MaxAge(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		MaxAgeSeconds:  3600,
	})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	if got := w.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("Max-Age = %q, want 3600", got)
	}
}

func TestCORS_ZeroMaxAge_NoMaxAgeHeader(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}, MaxAgeSeconds: 0})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	if got := w.Header().Get("Access-Control-Max-Age"); got != "" {
		t.Errorf("expected no Max-Age header, got %q", got)
	}
}

func TestCORS_DefaultMethods_WhenEmpty(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}, AllowedMethods: nil})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "GET") {
		t.Errorf("default methods should include GET, got %q", methods)
	}
}

func TestCORS_DefaultHeaders_WhenEmpty(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{Enabled: true, AllowedOrigins: []string{"*"}, AllowedHeaders: nil})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	headers := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") {
		t.Errorf("default headers should include Content-Type, got %q", headers)
	}
}

func TestCORS_AllowMethodsHeader(t *testing.T) {
	r := newCORSRouter(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	})
	w := doRequestWithHeaders(r, "GET", "/test", map[string]string{"Origin": "http://example.com"})
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods != "GET, POST" {
		t.Errorf("Allow-Methods = %q, want GET, POST", methods)
	}
}

// ── Metrics ───────────────────────────────────────────────────────────────────

func newMetricsRouter(store *metrics.Store, excludes []string, statusCode int) *gin.Engine {
	r := gin.New()
	r.Use(Metrics(store, excludes))
	r.GET("/test", func(c *gin.Context) { c.Status(statusCode) })
	r.GET("/health", func(c *gin.Context) { c.Status(200) })
	return r
}

func TestMetrics_NilStore_NoOp(t *testing.T) {
	r := newMetricsRouter(nil, nil, 200)
	w := doRequest(r, "GET", "/test", nil)
	if w.Code != 200 {
		t.Errorf("expected 200 with nil store, got %d", w.Code)
	}
}

func TestMetrics_ExcludedPath_NotRecorded(t *testing.T) {
	store := metrics.New()
	r := newMetricsRouter(store, []string{"/health"}, 200)
	doRequest(r, "GET", "/health", nil)
	stats := store.GetAll()
	if len(stats) != 0 {
		t.Errorf("excluded path should not be recorded, got %d stats", len(stats))
	}
}

func TestMetrics_NonExcludedPath_Recorded(t *testing.T) {
	store := metrics.New()
	r := newMetricsRouter(store, []string{"/health"}, 200)
	doRequest(r, "GET", "/test", nil)
	stats := store.GetAll()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Path != "/test" {
		t.Errorf("stat path = %q, want /test", stats[0].Path)
	}
}

func TestMetrics_500Response_ErrorCounted(t *testing.T) {
	store := metrics.New()
	r := newMetricsRouter(store, nil, 500)
	doRequest(r, "GET", "/test", nil)
	stats := store.GetAll()
	if len(stats) != 1 || stats[0].ErrorCount != 1 {
		t.Errorf("expected 1 error for 500 response")
	}
}

func TestMetrics_200Response_NoError(t *testing.T) {
	store := metrics.New()
	r := newMetricsRouter(store, nil, 200)
	doRequest(r, "GET", "/test", nil)
	stats := store.GetAll()
	if len(stats) != 1 || stats[0].ErrorCount != 0 {
		t.Errorf("expected 0 errors for 200 response")
	}
}

// ── RequestRecorder ───────────────────────────────────────────────────────────

func newRecorderRouter(rec *recorder.Recorder, recordBody bool, maxBodyBytes int, excludes []string) *gin.Engine {
	r := gin.New()
	r.Use(RequestRecorder(rec, recordBody, maxBodyBytes, excludes))
	r.POST("/test", func(c *gin.Context) {
		c.Set("matched_rule", "rule_0")
		c.Data(200, "application/json", []byte(`{"ok":true}`))
	})
	r.GET("/excluded", func(c *gin.Context) { c.Status(200) })
	r.GET("/search", func(c *gin.Context) { c.Status(200) })
	return r
}

func TestRequestRecorder_NilRecorder_NoOp(t *testing.T) {
	r := newRecorderRouter(nil, true, 65536, nil)
	w := doRequest(r, "GET", "/search", nil)
	if w.Code != 200 {
		t.Errorf("expected 200 with nil recorder, got %d", w.Code)
	}
}

func TestRequestRecorder_ExcludedPath_NotRecorded(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 65536, []string{"/excluded"})
	doRequest(r, "GET", "/excluded", nil)
	if rec.Count() != 0 {
		t.Errorf("excluded path should not be recorded")
	}
}

func TestRequestRecorder_RecordBody_True_CapturesBody(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 65536, nil)
	body := `{"key":"value"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if rec.Count() != 1 {
		t.Fatalf("expected 1 recorded entry")
	}
	entries := rec.List(0, 0)
	if entries[0].RequestBody != body {
		t.Errorf("RequestBody = %q, want %q", entries[0].RequestBody, body)
	}
}

func TestRequestRecorder_RecordBody_False_NoBody(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, false, 65536, nil)
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := rec.List(0, 0)
	if len(entries) == 0 {
		t.Fatal("expected entry to be recorded")
	}
	if entries[0].RequestBody != "" {
		t.Errorf("expected empty RequestBody when recordBody=false, got %q", entries[0].RequestBody)
	}
}

func TestRequestRecorder_RecordBody_Truncated(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 10, nil) // max 10 bytes
	body := "0123456789EXTRA"                  // 15 bytes
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := rec.List(0, 0)
	if len(entries) == 0 {
		t.Fatal("expected entry")
	}
	if !strings.HasSuffix(entries[0].RequestBody, "...[truncated]") {
		t.Errorf("expected truncation marker, got %q", entries[0].RequestBody)
	}
}

func TestRequestRecorder_NoBody_NilBody(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 65536, nil)
	doRequest(r, "GET", "/search", nil)
	entries := rec.List(0, 0)
	if len(entries) == 0 {
		t.Fatal("expected entry")
	}
	if entries[0].RequestBody != "" {
		t.Errorf("expected empty body for GET, got %q", entries[0].RequestBody)
	}
}

func TestRequestRecorder_CapturesResponseBody(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 65536, nil)
	req := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := rec.List(0, 0)
	if entries[0].ResponseBody != `{"ok":true}` {
		t.Errorf("ResponseBody = %q, want {\"ok\":true}", entries[0].ResponseBody)
	}
}

func TestRequestRecorder_ResponseBodyTruncated(t *testing.T) {
	rec := recorder.New(100)
	r := gin.New()
	r.Use(RequestRecorder(rec, true, 5, nil)) // max 5 bytes
	r.GET("/big", func(c *gin.Context) {
		c.Data(200, "text/plain", []byte("0123456789"))
	})
	req := httptest.NewRequest("GET", "/big", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := rec.List(0, 0)
	if !strings.HasSuffix(entries[0].ResponseBody, "...[truncated]") {
		t.Errorf("expected response body truncated, got %q", entries[0].ResponseBody)
	}
}

func TestRequestRecorder_CapturesMatchedRule(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 65536, nil)
	req := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := rec.List(0, 0)
	if entries[0].MatchedRule != "rule_0" {
		t.Errorf("MatchedRule = %q, want rule_0", entries[0].MatchedRule)
	}
}

func TestRequestRecorder_CapturesRequestHeaders(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 65536, nil)
	req := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
	req.Header.Set("X-Custom", "myvalue")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	entries := rec.List(0, 0)
	if entries[0].RequestHeaders["X-Custom"] != "myvalue" {
		t.Errorf("X-Custom header not captured, got %v", entries[0].RequestHeaders)
	}
}

func TestRequestRecorder_CapturesQueryString(t *testing.T) {
	rec := recorder.New(100)
	r := newRecorderRouter(rec, true, 65536, nil)
	doRequest(r, "GET", "/search?q=hello&page=2", nil)

	entries := rec.List(0, 0)
	if entries[0].Query != "q=hello&page=2" {
		t.Errorf("Query = %q, want q=hello&page=2", entries[0].Query)
	}
}

func TestRequestRecorder_DefaultMaxBodyBytes(t *testing.T) {
	rec := recorder.New(100)
	r := gin.New()
	r.Use(RequestRecorder(rec, true, 0, nil)) // 0 → default 65536
	r.POST("/test", func(c *gin.Context) { c.Status(200) })
	body := bytes.Repeat([]byte("x"), 100)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	entries := rec.List(0, 0)
	if len(entries) == 0 {
		t.Fatal("expected entry")
	}
	// Small body should not be truncated
	if strings.Contains(entries[0].RequestBody, "[truncated]") {
		t.Errorf("100 byte body should not be truncated with default 65536 limit")
	}
}

func TestRequestRecorder_RecordBody_False_BodyStillForwardedDownstream(t *testing.T) {
	rec := recorder.New(100)
	r := gin.New()
	var capturedBody []byte
	r.Use(RequestRecorder(rec, false, 65536, nil))
	r.POST("/test", func(c *gin.Context) {
		capturedBody, _ = io.ReadAll(c.Request.Body)
		c.Status(200)
	})
	body := `{"hello":"world"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if string(capturedBody) != body {
		t.Errorf("body not forwarded downstream when recordBody=false, got %q", capturedBody)
	}
}

func TestBodyWriter_Write_BothBuffers(t *testing.T) {
	var buf bytes.Buffer
	bw := &bodyWriter{
		body: &bytes.Buffer{},
	}
	_ = bw // just verify it compiles; actual write tested via middleware integration
	_ = buf
}
