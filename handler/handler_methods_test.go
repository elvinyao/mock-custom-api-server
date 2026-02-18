package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

// These tests cover registerEndpoints paths for PATCH, OPTIONS, HEAD, ANY and default method

func TestRegisterEndpoints_PatchMethod(t *testing.T) {
	f := writeTempFile(t, `{"method":"patch"}`)
	defer os.Remove(f)

	ep := config.Endpoint{
		Path:   "/patch-ep",
		Method: "PATCH",
		Default: config.ResponseConfig{ResponseFile: f, StatusCode: 200},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/patch-ep", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PATCH status = %d, want 200", w.Code)
	}
}

func TestRegisterEndpoints_OptionsMethod(t *testing.T) {
	f := writeTempFile(t, `{"method":"options"}`)
	defer os.Remove(f)

	ep := config.Endpoint{
		Path:   "/options-ep",
		Method: "OPTIONS",
		Default: config.ResponseConfig{ResponseFile: f, StatusCode: 200},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/options-ep", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS status = %d, want 200", w.Code)
	}
}

func TestRegisterEndpoints_HeadMethod(t *testing.T) {
	f := writeTempFile(t, `{}`)
	defer os.Remove(f)

	ep := config.Endpoint{
		Path:   "/head-ep",
		Method: "HEAD",
		Default: config.ResponseConfig{ResponseFile: f, StatusCode: 200},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("HEAD", "/head-ep", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("HEAD status = %d, want 200", w.Code)
	}
}

func TestRegisterEndpoints_AnyMethod(t *testing.T) {
	f := writeTempFile(t, `{"method":"any"}`)
	defer os.Remove(f)

	ep := config.Endpoint{
		Path:   "/any-ep",
		Method: "ANY",
		Default: config.ResponseConfig{ResponseFile: f, StatusCode: 200},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/any-ep", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("ANY method %s: status = %d, want 200", method, w.Code)
		}
	}
}

func TestRegisterEndpoints_PutMethod(t *testing.T) {
	f := writeTempFile(t, `{"method":"put"}`)
	defer os.Remove(f)

	ep := config.Endpoint{
		Path:   "/put-ep",
		Method: "PUT",
		Default: config.ResponseConfig{ResponseFile: f, StatusCode: 200},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/put-ep", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT status = %d, want 200", w.Code)
	}
}

func TestRegisterEndpoints_DeleteMethod(t *testing.T) {
	f := writeTempFile(t, `{"deleted":true}`)
	defer os.Remove(f)

	ep := config.Endpoint{
		Path:   "/delete-ep",
		Method: "DELETE",
		Default: config.ResponseConfig{ResponseFile: f, StatusCode: 200},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/delete-ep", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("DELETE status = %d, want 200", w.Code)
	}
}

// ── findEndpoint – ANY method + method mismatch ───────────────────────────────

func TestFindEndpoint_AnyMethod_MatchesAll(t *testing.T) {
	h := NewMockHandler(buildTestConfigManager(nil))
	endpoints := []config.Endpoint{
		{Path: "/api/anything", Method: "ANY"},
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		ep, _ := h.findEndpoint(endpoints, "/api/anything", method)
		if ep == nil {
			t.Errorf("findEndpoint ANY should match method %s", method)
		}
	}
}

func TestFindEndpoint_MethodMismatch(t *testing.T) {
	h := NewMockHandler(buildTestConfigManager(nil))
	endpoints := []config.Endpoint{
		{Path: "/api/ep", Method: "GET"},
	}
	ep, _ := h.findEndpoint(endpoints, "/api/ep", "POST")
	if ep != nil {
		t.Error("expected nil for method mismatch")
	}
}

func TestFindEndpoint_CaseInsensitiveMethod(t *testing.T) {
	h := NewMockHandler(buildTestConfigManager(nil))
	endpoints := []config.Endpoint{
		{Path: "/api/ep", Method: "get"}, // lowercase in config
	}
	ep, _ := h.findEndpoint(endpoints, "/api/ep", "GET")
	if ep == nil {
		t.Error("expected match for case-insensitive method")
	}
}

// ── handleRequest – runtime endpoint + response file error ────────────────────

func TestHandleRequest_RuntimeEndpoint(t *testing.T) {
	f := writeTempFile(t, `{"runtime":true}`)
	defer os.Remove(f)

	cm := config.NewConfigManager("")
	cm.SetConfig(&config.Config{
		Endpoints: []config.Endpoint{
			{
				Path:   "/runtime/ep",
				Method: "GET",
				Default: config.ResponseConfig{
					ResponseFile: f,
					StatusCode:   200,
				},
			},
		},
	})

	h := NewMockHandler(cm)
	r := gin.New()
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/runtime/ep", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("runtime")) {
		t.Errorf("expected runtime response, got %s", w.Body.String())
	}
}

func TestHandleRequest_ResponseFile_NotFound_500(t *testing.T) {
	ep := config.Endpoint{
		Path:   "/broken",
		Method: "GET",
		Default: config.ResponseConfig{
			ResponseFile: "/nonexistent/file.json",
			StatusCode:   200,
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/broken", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for missing response file", w.Code)
	}
}

// ── handleRequest – random responses ─────────────────────────────────────────

func TestHandleRequest_RandomResponses(t *testing.T) {
	f1 := writeTempFile(t, `{"r":"1"}`)
	f2 := writeTempFile(t, `{"r":"2"}`)
	defer os.Remove(f1)
	defer os.Remove(f2)

	ep := config.Endpoint{
		Path:   "/random",
		Method: "GET",
		Default: config.ResponseConfig{
			StatusCode: 200,
			RandomResponses: &config.RandomResponses{
				Enabled: true,
				Files: []config.RandomResponse{
					{File: f1, Weight: 1, StatusCode: 200},
					{File: f2, Weight: 1, StatusCode: 201},
				},
			},
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/random", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 && w.Code != 201 {
		t.Errorf("random response status = %d, want 200 or 201", w.Code)
	}
}

// ── handleRequest – template substitution ────────────────────────────────────

func TestHandleRequest_WithTemplate(t *testing.T) {
	f := writeTempFile(t, `{"user":"{{.uid}}"}`)
	defer os.Remove(f)

	ep := config.Endpoint{
		Path:   "/tmpl",
		Method: "GET",
		Selectors: []config.Selector{
			{Name: "uid", Type: "query", Key: "uid"},
		},
		Default: config.ResponseConfig{
			ResponseFile: f,
			StatusCode:   200,
			Template:     &config.TemplateConfig{Enabled: true, Engine: "simple"},
		},
	}
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/tmpl?uid=testuser", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("testuser")) {
		t.Errorf("expected template substitution, got %s", w.Body.String())
	}
}
