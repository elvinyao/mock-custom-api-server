package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// helper: build a gin context with given method, path, body, headers and query
func buildContext(t *testing.T, method, path, body string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
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
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

// ── ExtractValues ─────────────────────────────────────────────────────────────

func TestExtractValues_HeaderSelector(t *testing.T) {
	c, _ := buildContext(t, "GET", "/", "", map[string]string{"X-User-ID": "user42"})

	selectors := []Selector{
		{Name: "user_id", Type: "header", Key: "X-User-ID"},
	}
	vals := ExtractValues(c, selectors, nil)
	if vals["user_id"] != "user42" {
		t.Errorf("user_id = %q, want user42", vals["user_id"])
	}
}

func TestExtractValues_QuerySelector(t *testing.T) {
	c, _ := buildContext(t, "GET", "/?status=active&page=2", "", nil)

	selectors := []Selector{
		{Name: "status", Type: "query", Key: "status"},
		{Name: "page", Type: "query", Key: "page"},
	}
	vals := ExtractValues(c, selectors, nil)
	if vals["status"] != "active" {
		t.Errorf("status = %q, want active", vals["status"])
	}
	if vals["page"] != "2" {
		t.Errorf("page = %q, want 2", vals["page"])
	}
}

func TestExtractValues_BodySelector(t *testing.T) {
	body := `{"order_id":"ord-123","status":"pending"}`
	req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	selectors := []Selector{
		{Name: "order_id", Type: "body", Key: "order_id"},
		{Name: "status", Type: "body", Key: "status"},
	}
	vals := ExtractValues(c, selectors, nil)
	if vals["order_id"] != "ord-123" {
		t.Errorf("order_id = %q, want ord-123", vals["order_id"])
	}
	if vals["status"] != "pending" {
		t.Errorf("status = %q, want pending", vals["status"])
	}
}

func TestExtractValues_BodyReadOnce(t *testing.T) {
	// Two body selectors: body should be read once and reused
	body := `{"a":"1","b":"2"}`
	req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	selectors := []Selector{
		{Name: "a", Type: "body", Key: "a"},
		{Name: "b", Type: "body", Key: "b"},
	}
	vals := ExtractValues(c, selectors, nil)
	if vals["a"] != "1" || vals["b"] != "2" {
		t.Errorf("expected a=1 and b=2, got a=%q b=%q", vals["a"], vals["b"])
	}
}

func TestExtractValues_PathSelector_FromParamMap(t *testing.T) {
	c, _ := buildContext(t, "GET", "/users/99", "", nil)

	selectors := []Selector{
		{Name: "user_id", Type: "path", Key: "id"},
	}
	pathParams := map[string]string{"id": "99"}
	vals := ExtractValues(c, selectors, pathParams)
	if vals["user_id"] != "99" {
		t.Errorf("user_id = %q, want 99", vals["user_id"])
	}
}

func TestExtractValues_PathSelector_Fallback(t *testing.T) {
	// No pathParams map, falls back to c.Param (which returns empty for non-gin-routed context)
	c, _ := buildContext(t, "GET", "/", "", nil)
	selectors := []Selector{
		{Name: "x", Type: "path", Key: "x"},
	}
	vals := ExtractValues(c, selectors, nil)
	// Should not panic; value will be empty
	_ = vals["x"]
}

func TestExtractValues_MissingHeader_EmptyString(t *testing.T) {
	c, _ := buildContext(t, "GET", "/", "", nil)
	selectors := []Selector{
		{Name: "auth", Type: "header", Key: "Authorization"},
	}
	vals := ExtractValues(c, selectors, nil)
	if vals["auth"] != "" {
		t.Errorf("expected empty string for missing header, got %q", vals["auth"])
	}
}

func TestExtractValues_UnknownType_EmptyString(t *testing.T) {
	c, _ := buildContext(t, "GET", "/", "", nil)
	selectors := []Selector{
		{Name: "x", Type: "cookie", Key: "session"},
	}
	vals := ExtractValues(c, selectors, nil)
	if vals["x"] != "" {
		t.Errorf("expected empty string for unknown type, got %q", vals["x"])
	}
}

func TestExtractValues_EmptySelectors(t *testing.T) {
	c, _ := buildContext(t, "GET", "/", "", nil)
	vals := ExtractValues(c, nil, nil)
	if len(vals) != 0 {
		t.Errorf("expected empty map for nil selectors, got %d entries", len(vals))
	}
}

func TestExtractValues_TypeCaseInsensitive(t *testing.T) {
	c, _ := buildContext(t, "GET", "/?foo=bar", "", nil)
	selectors := []Selector{
		{Name: "f", Type: "QUERY", Key: "foo"},
	}
	vals := ExtractValues(c, selectors, nil)
	if vals["f"] != "bar" {
		t.Errorf("QUERY type case-insensitive: f = %q, want bar", vals["f"])
	}
}

// ── ConvertSelectors ──────────────────────────────────────────────────────────

func TestConvertSelectors_Basic(t *testing.T) {
	input := []struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"`
		Key  string `yaml:"key"`
	}{
		{Name: "a", Type: "header", Key: "X-A"},
		{Name: "b", Type: "query", Key: "q"},
	}
	out := ConvertSelectors(input)
	if len(out) != 2 {
		t.Fatalf("expected 2 selectors, got %d", len(out))
	}
	if out[0].Name != "a" || out[0].Type != "header" || out[0].Key != "X-A" {
		t.Errorf("selector[0] mismatch: %+v", out[0])
	}
	if out[1].Name != "b" || out[1].Type != "query" || out[1].Key != "q" {
		t.Errorf("selector[1] mismatch: %+v", out[1])
	}
}

func TestConvertSelectors_Empty(t *testing.T) {
	out := ConvertSelectors(nil)
	if len(out) != 0 {
		t.Errorf("expected empty slice for nil input, got %d", len(out))
	}
}
