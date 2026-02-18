package ui

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRegisterRoutes_RedirectBarePrefix(t *testing.T) {
	r := gin.New()
	RegisterRoutes(r, "/ui")

	req, _ := http.NewRequest("GET", "/ui", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want 302 (redirect)", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/ui/" {
		t.Errorf("Location = %q, want /ui/", loc)
	}
}

func TestRegisterRoutes_ServeIndexHTML(t *testing.T) {
	r := gin.New()
	RegisterRoutes(r, "/ui")

	req, _ := http.NewRequest("GET", "/ui/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct == "" {
		t.Error("Content-Type should not be empty for index.html")
	}
}

func TestRegisterRoutes_ServeIndexHTML_EmptyFilepath(t *testing.T) {
	// Some routers match "" as the root param
	r := gin.New()
	RegisterRoutes(r, "/ui")

	// Access /ui/ which maps to filepath=""
	req, _ := http.NewRequest("GET", "/ui/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRegisterRoutes_ServeStaticAsset(t *testing.T) {
	r := gin.New()
	RegisterRoutes(r, "/ui")

	// Request a static file that doesn't exist → 404
	req, _ := http.NewRequest("GET", "/ui/nonexistent.js", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should not be 200 since the file doesn't exist
	if w.Code == http.StatusOK {
		t.Error("expected non-200 for nonexistent asset")
	}
}

func TestRegisterRoutes_DifferentPrefix(t *testing.T) {
	r := gin.New()
	RegisterRoutes(r, "/mock-admin/ui")

	req, _ := http.NewRequest("GET", "/mock-admin/ui", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", w.Code)
	}
}

// TestRegisterRoutes_IndexNotFound tests the StatusNotFound branch by temporarily
// replacing staticFiles with an empty embed.FS so ReadFile("static/index.html") fails.
func TestRegisterRoutes_IndexNotFound(t *testing.T) {
	orig := staticFiles
	defer func() { staticFiles = orig }()
	// empty embed.FS has no files; fs.Sub succeeds but ReadFile will fail
	staticFiles = embed.FS{}

	r := gin.New()
	RegisterRoutes(r, "/ui")

	req, _ := http.NewRequest("GET", "/ui/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (index.html not found in empty FS)", w.Code)
	}
}
