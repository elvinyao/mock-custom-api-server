package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mock-api-server/config"
	"mock-api-server/metrics"
	"mock-api-server/recorder"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestHandler() (*Handler, *config.ConfigManager, *recorder.Recorder, *metrics.Store) {
	cm := config.NewConfigManager("")
	rec := recorder.New(100)
	ms := metrics.New()
	h := New(cm, rec, ms)
	return h, cm, rec, ms
}

func setupRouter(h *Handler) *gin.Engine {
	r := gin.New()
	h.RegisterRoutes(r, "/admin", config.AdminAuth{})
	return r
}

func doRequest(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ── getHealth ─────────────────────────────────────────────────────────────────

func TestGetHealth_NoConfig(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/health", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", resp["status"])
	}
}

func TestGetHealth_WithConfig(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.SetConfig(&config.Config{
		Endpoints: []config.Endpoint{{Path: "/a"}, {Path: "/b"}},
	})
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/health", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	cfgMap := resp["config"].(map[string]interface{})
	count := cfgMap["endpoints_count"].(float64)
	if count != 2 {
		t.Errorf("endpoints_count = %v, want 2", count)
	}
}

// ── getConfig ─────────────────────────────────────────────────────────────────

func TestGetConfig_NilConfig_503(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/config", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestGetConfig_WithConfig_200(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.SetConfig(&config.Config{Port: 9999})
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/config", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["port"].(float64) != 9999 {
		t.Errorf("port = %v, want 9999", resp["port"])
	}
}

// ── postConfigReload ──────────────────────────────────────────────────────────

func TestPostConfigReload_InvalidPath_500(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "POST", "/admin/config/reload", nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for invalid config path", w.Code)
	}
}

func TestPostConfigReload_ValidPath_200(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfgContent := `port: 8080
health_check:
  enabled: true
  path: /health
`
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cm := config.NewConfigManager(cfgFile)
	h := New(cm, nil, nil)
	r := gin.New()
	h.RegisterRoutes(r, "/admin", config.AdminAuth{})

	w := doRequest(r, "POST", "/admin/config/reload", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
}

// ── listEndpoints ─────────────────────────────────────────────────────────────

func TestListEndpoints_Empty(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.SetConfig(&config.Config{})
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/endpoints", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestListEndpoints_WithFileAndRuntime(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.SetConfig(&config.Config{
		Endpoints: []config.Endpoint{{Path: "/file/ep"}},
	})
	cm.AddRuntimeEndpoint(config.Endpoint{Path: "/runtime/ep"})
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/endpoints", nil)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", resp["total"])
	}
}

// ── addEndpoint ───────────────────────────────────────────────────────────────

func TestAddEndpoint_ValidPayload_201(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	ep := map[string]interface{}{
		"path":   "/new/ep",
		"method": "GET",
	}
	w := doRequest(r, "POST", "/admin/endpoints", ep)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}
}

func TestAddEndpoint_MissingPath_400(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	ep := map[string]interface{}{
		"method": "GET",
	}
	w := doRequest(r, "POST", "/admin/endpoints", ep)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAddEndpoint_MissingMethod_400(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	ep := map[string]interface{}{
		"path": "/ep",
	}
	w := doRequest(r, "POST", "/admin/endpoints", ep)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ── updateEndpoint ────────────────────────────────────────────────────────────

func TestUpdateEndpoint_ValidIndex_200(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.AddRuntimeEndpoint(config.Endpoint{Path: "/old"})
	r := setupRouter(h)
	ep := map[string]interface{}{
		"path":   "/updated",
		"method": "POST",
	}
	w := doRequest(r, "PUT", "/admin/endpoints/0", ep)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEndpoint_InvalidID_400(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "PUT", "/admin/endpoints/notanumber", map[string]interface{}{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUpdateEndpoint_OutOfRange_404(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	ep := map[string]interface{}{"path": "/x", "method": "GET"}
	w := doRequest(r, "PUT", "/admin/endpoints/99", ep)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ── deleteEndpoint ────────────────────────────────────────────────────────────

func TestDeleteEndpoint_ValidIndex_200(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.AddRuntimeEndpoint(config.Endpoint{Path: "/to-delete"})
	r := setupRouter(h)
	w := doRequest(r, "DELETE", "/admin/endpoints/0", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestDeleteEndpoint_InvalidID_400(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "DELETE", "/admin/endpoints/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDeleteEndpoint_OutOfRange_404(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "DELETE", "/admin/endpoints/0", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ── listRequests ──────────────────────────────────────────────────────────────

func TestListRequests_NilRecorder(t *testing.T) {
	cm := config.NewConfigManager("")
	h := New(cm, nil, nil)
	r := gin.New()
	h.RegisterRoutes(r, "/admin", config.AdminAuth{})

	w := doRequest(r, "GET", "/admin/requests", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestListRequests_WithRecords(t *testing.T) {
	h, _, rec, _ := newTestHandler()
	for i := 0; i < 5; i++ {
		rec.Record(&recorder.RecordedRequest{
			Method:         "GET",
			Path:           "/test",
			Timestamp:      time.Now(),
			ResponseStatus: 200,
		})
	}
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 5 {
		t.Errorf("total = %v, want 5", resp["total"])
	}
}

func TestListRequests_LimitAndOffset(t *testing.T) {
	h, _, rec, _ := newTestHandler()
	for i := 0; i < 10; i++ {
		rec.Record(&recorder.RecordedRequest{
			Method: "GET", Path: "/x",
			Timestamp: time.Now(), ResponseStatus: 200,
		})
	}
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests?limit=3&offset=2", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	requests := resp["requests"].([]interface{})
	if len(requests) != 3 {
		t.Errorf("requests count = %d, want 3", len(requests))
	}
}

// ── getRequest ────────────────────────────────────────────────────────────────

func TestGetRequest_NilRecorder_404(t *testing.T) {
	cm := config.NewConfigManager("")
	h := New(cm, nil, nil)
	r := gin.New()
	h.RegisterRoutes(r, "/admin", config.AdminAuth{})
	w := doRequest(r, "GET", "/admin/requests/req_1", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestGetRequest_NotFound_404(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests/req_99999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestGetRequest_Found_200(t *testing.T) {
	h, _, rec, _ := newTestHandler()
	entry := &recorder.RecordedRequest{
		Method:         "GET",
		Path:           "/found",
		Timestamp:      time.Now(),
		ResponseStatus: 200,
	}
	rec.Record(entry)
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests/"+entry.ID, nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ── clearRequests ─────────────────────────────────────────────────────────────

func TestClearRequests_ClearsAll(t *testing.T) {
	h, _, rec, _ := newTestHandler()
	rec.Record(&recorder.RecordedRequest{
		Method: "GET", Path: "/x",
		Timestamp: time.Now(), ResponseStatus: 200,
	})
	r := setupRouter(h)
	w := doRequest(r, "DELETE", "/admin/requests", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if rec.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", rec.Count())
	}
}

func TestClearRequests_NilRecorder_200(t *testing.T) {
	cm := config.NewConfigManager("")
	h := New(cm, nil, nil)
	r := gin.New()
	h.RegisterRoutes(r, "/admin", config.AdminAuth{})
	w := doRequest(r, "DELETE", "/admin/requests", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ── getMetrics ────────────────────────────────────────────────────────────────

func TestGetMetrics_NilStore(t *testing.T) {
	cm := config.NewConfigManager("")
	h := New(cm, nil, nil)
	r := gin.New()
	h.RegisterRoutes(r, "/admin", config.AdminAuth{})
	w := doRequest(r, "GET", "/admin/metrics", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["uptime_sec"].(float64) != 0 {
		t.Errorf("uptime_sec = %v, want 0", resp["uptime_sec"])
	}
}

func TestGetMetrics_WithData(t *testing.T) {
	h, _, _, ms := newTestHandler()
	ms.Record("GET", "/test", 200, 50)
	ms.Record("GET", "/test", 500, 100)
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/metrics", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	eps := resp["endpoints"].([]interface{})
	if len(eps) != 1 {
		t.Errorf("endpoints count = %d, want 1", len(eps))
	}
}

// ── Basic auth ────────────────────────────────────────────────────────────────

func TestBasicAuth_Required_Unauthorized(t *testing.T) {
	cm := config.NewConfigManager("")
	cm.SetConfig(&config.Config{})
	h := New(cm, nil, nil)
	r := gin.New()
	auth := config.AdminAuth{
		Enabled:  true,
		Username: "admin",
		Password: "secret",
	}
	h.RegisterRoutes(r, "/admin", auth)

	req, _ := http.NewRequest("GET", "/admin/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBasicAuth_ValidCredentials_200(t *testing.T) {
	cm := config.NewConfigManager("")
	cm.SetConfig(&config.Config{})
	h := New(cm, nil, nil)
	r := gin.New()
	auth := config.AdminAuth{
		Enabled:  true,
		Username: "admin",
		Password: "secret",
	}
	h.RegisterRoutes(r, "/admin", auth)

	req, _ := http.NewRequest("GET", "/admin/health", nil)
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ── writeEndpointToFile ───────────────────────────────────────────────────────

func TestWriteEndpointToFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ep.yaml")
	ep := config.Endpoint{
		Path:   "/test",
		Method: "GET",
	}
	if err := writeEndpointToFile(f, ep); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("could not read written file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty YAML file")
	}
}
