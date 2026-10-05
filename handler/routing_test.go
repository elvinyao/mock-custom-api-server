package handler

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"mock-api-server/config"
)

func TestMatchPath_SegmentBoundariesAndCaptures(t *testing.T) {
	for _, tt := range []struct {
		pattern, path string
		matched       bool
		params        map[string]string
	}{
		{"/api/*rest", "/apix", false, nil},
		{"/api/*rest", "/api", true, map[string]string{"rest": ""}},
		{"/api/*rest", "/api/", true, map[string]string{"rest": "/"}},
		{"/api/*rest", "/api/a/b", true, map[string]string{"rest": "/a/b"}},
		{"/org/:id/*rest", "/org/1/a", true, map[string]string{"id": "1", "rest": "/a"}},
		{"/org/:id/*rest", "/org//a", false, nil},
		{"/org/:id", "/org/1/", true, map[string]string{"id": "1"}},
	} {
		t.Run(tt.pattern+tt.path, func(t *testing.T) {
			params, matched := matchPath(tt.pattern, tt.path)
			if matched != tt.matched || !reflect.DeepEqual(params, tt.params) {
				t.Errorf("matchPath = %v, %v; want %v, %v", params, matched, tt.params, tt.matched)
			}
		})
	}
}

func TestRouting_StartupAndRuntimeSharePrecedence(t *testing.T) {
	for _, runtime := range []bool{false, true} {
		name := "startup"
		if runtime {
			name = "runtime"
		}
		t.Run(name, func(t *testing.T) {
			endpoints := []config.Endpoint{
				{Path: "/*rest", Method: "ANY", Default: config.ResponseConfig{InlineBody: "catch-all"}},
				{Path: "/org/:id/*rest", Method: "GET", Default: config.ResponseConfig{InlineBody: "nested catch-all"}},
				{Path: "/org/:id", Method: "GET", Default: config.ResponseConfig{InlineBody: "parameter"}},
				{Path: "/org/me", Method: "GET", Default: config.ResponseConfig{InlineBody: "static"}},
				{Path: "/org/:other", Method: "GET", Default: config.ResponseConfig{InlineBody: "later tie"}},
			}
			cm := buildTestConfigManager(&config.Config{})
			if !runtime {
				cm.SetConfig(&config.Config{Endpoints: endpoints})
			}
			r := gin.New()
			r.GET("/health", func(c *gin.Context) { c.String(200, "health") })
			r.GET("/mock-admin/ui/*path", func(c *gin.Context) { c.String(200, "ui") })
			r.GET("/mock-admin/config", func(c *gin.Context) { c.String(200, "admin") })
			NewMockHandler(cm).RegisterRoutes(r)
			if runtime {
				for _, ep := range endpoints {
					cm.AddRuntimeEndpoint(ep)
				}
			}
			for _, tt := range []struct{ path, want string }{
				{"/org/me", "static"}, {"/org/me/", "static"}, {"/org/7", "parameter"}, {"/org/7/a", "nested catch-all"}, {"/other", "catch-all"}, {"/health", "health"}, {"/mock-admin/ui/", "ui"}, {"/mock-admin/config", "admin"},
			} {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
				if w.Code != 200 || w.Body.String() != tt.want {
					t.Errorf("%s = %d %q; want 200 %q", tt.path, w.Code, w.Body.String(), tt.want)
				}
			}
		})
	}
}

func TestRouting_StartupRuntimeMethodCompatibility(t *testing.T) {
	for _, runtime := range []bool{false, true} {
		for _, tt := range []struct {
			configured, method string
			status             int
		}{
			{"GET", "HEAD", 404}, {"HEAD", "HEAD", 200}, {"get", "GET", 200}, {"any", "DELETE", 200}, {"ANY", "HEAD", 200}, {"ANY", "CUSTOM", 200}, {"POST", "GET", 404},
		} {
			ep := config.Endpoint{Path: "/method", Method: tt.configured, Default: config.ResponseConfig{InlineBody: "ok"}}
			cm := buildTestConfigManager(&config.Config{})
			if !runtime {
				cm.SetConfig(&config.Config{Endpoints: []config.Endpoint{ep}})
			}
			r := gin.New()
			NewMockHandler(cm).RegisterRoutes(r)
			if runtime {
				cm.AddRuntimeEndpoint(ep)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, "/method/", nil))
			if w.Code != tt.status {
				t.Errorf("runtime=%v %s/%s status=%d want=%d", runtime, tt.configured, tt.method, w.Code, tt.status)
			}
		}
	}
}

func TestRouting_LiveReplacementRemovesOldRoutes(t *testing.T) {
	cm := buildTestConfigManager(&config.Config{Endpoints: []config.Endpoint{{Path: "/old", Method: "GET", Default: config.ResponseConfig{InlineBody: "old"}}}})
	r := gin.New()
	NewMockHandler(cm).RegisterRoutes(r)
	cm.SetConfig(&config.Config{Endpoints: []config.Endpoint{{Path: "/new", Method: "GET", Default: config.ResponseConfig{InlineBody: "new"}}}})
	for _, tt := range []struct {
		path   string
		status int
	}{{"/old/", 404}, {"/new/", 200}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
		if w.Code != tt.status {
			t.Errorf("%s = %d want %d", tt.path, w.Code, tt.status)
		}
	}
}
