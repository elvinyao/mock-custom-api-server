package handler

import (
	"net/http/httptest"
	"os"
	"testing"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newEndpointWithFile(f, path, method string) config.Endpoint {
	return config.Endpoint{
		Path:   path,
		Method: method,
		Default: config.ResponseConfig{
			ResponseFile: f,
			StatusCode:   200,
		},
	}
}

func buildDefaultEndpoint(f, path, method string) config.Endpoint {
	return config.Endpoint{
		Path:   path,
		Method: method,
		Default: config.ResponseConfig{
			ResponseFile: f,
			StatusCode:   200,
		},
	}
}

func writeTempFileHelper(t *testing.T, dir, content string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString(content)
	return f.Name()
}

// ── matchCondition – default/unknown type ─────────────────────────────────────

func TestMatchCondition_UnknownType_FallsBackToExact(t *testing.T) {
	cond := Condition{Selector: "x", MatchType: "fuzzy", Value: "hello"}
	if !matchCondition("hello", cond) {
		t.Error("unknown type should default to exact match (true for equal values)")
	}
	if matchCondition("world", cond) {
		t.Error("unknown type should default to exact match (false for unequal values)")
	}
}

// ── matchRange – edge cases ───────────────────────────────────────────────────

func TestMatchRange_NonNumericTarget(t *testing.T) {
	if matchRange("notanumber", "[1, 100]") {
		t.Error("expected false for non-numeric target")
	}
}

func TestMatchRange_TooShortRangeString(t *testing.T) {
	if matchRange("50", "[1]") {
		t.Error("expected false for too short range string")
	}
}

func TestMatchRange_InvalidMinValue(t *testing.T) {
	if matchRange("50", "[abc, 100]") {
		t.Error("expected false for invalid min value")
	}
}

func TestMatchRange_InvalidMaxValue(t *testing.T) {
	if matchRange("50", "[1, xyz]") {
		t.Error("expected false for invalid max value")
	}
}

func TestMatchRange_ExclusiveBounds(t *testing.T) {
	if !matchRange("50", "(0, 100)") {
		t.Error("expected true: 50 is between 0 and 100 (exclusive)")
	}
	if matchRange("0", "(0, 100)") {
		t.Error("expected false: 0 is not > 0 (exclusive)")
	}
	if matchRange("100", "(0, 100)") {
		t.Error("expected false: 100 is not < 100 (exclusive)")
	}
}

func TestMatchRange_MixedBounds(t *testing.T) {
	if !matchRange("0", "[0, 100)") {
		t.Error("expected true: 0 >= 0 (inclusive)")
	}
	if matchRange("100", "[0, 100)") {
		t.Error("expected false: 100 is not < 100 (exclusive)")
	}
}

func TestMatchRange_InvalidPartsCount(t *testing.T) {
	if matchRange("50", "[1 100]") {
		t.Error("expected false for range without comma")
	}
}

func TestMatchRange_InRange_Inclusive(t *testing.T) {
	if !matchRange("1", "[1, 100]") {
		t.Error("expected true: 1 == min (inclusive)")
	}
	if !matchRange("100", "[1, 100]") {
		t.Error("expected true: 100 == max (inclusive)")
	}
}

func TestMatchRange_OutOfRange(t *testing.T) {
	if matchRange("0", "[1, 100]") {
		t.Error("expected false: 0 < min")
	}
	if matchRange("101", "[1, 100]") {
		t.Error("expected false: 101 > max")
	}
}

// ── extractPathParams – ensure no panic ───────────────────────────────────────

func TestExtractPathParams_StaticPath_NoMatch(t *testing.T) {
	params := extractPathParams("/api/v1", "/api/v2")
	if len(params) != 0 {
		t.Errorf("expected empty map for no match, got %v", params)
	}
}

func TestExtractPathParams_WithColonParam(t *testing.T) {
	params := extractPathParams("/api/users/:id", "/api/users/42")
	_ = params // just verify no panic
}

func TestExtractPathParams_SamePattern(t *testing.T) {
	params := extractPathParams("/api/v1", "/api/v1")
	_ = params // just verify no panic
}

// ── selectRandomResponse – single entry always returns it ────────────────────

func TestSelectRandomResponse_AllWeightedOneEntry(t *testing.T) {
	responses := []RandomResponseConfig{
		{File: "single.json", Weight: 100, StatusCode: 200},
	}
	r := selectRandomResponse(responses)
	if r.File != "single.json" {
		t.Errorf("expected single.json, got %q", r.File)
	}
}

// ── Build – template engine empty defaults to simple ─────────────────────────

func TestBuild_TemplateNoEngine_DefaultsToSimple(t *testing.T) {
	dir := t.TempDir()
	f := writeTempFileHelper(t, dir, `{"x":"{{.myval}}"}`)

	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{
		ResponseFile:    f,
		TemplateEnabled: true,
		TemplateEngine:  "", // should default to simple
	}, map[string]string{"myval": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res.Body) != `{"x":"hello"}` {
		t.Errorf("Body = %q, want {\"x\":\"hello\"}", string(res.Body))
	}
}

// ── registerEndpoints – TRACE via default case ───────────────────────────────

func TestRegisterEndpoints_DefaultCaseMethod(t *testing.T) {
	f := writeTempFile(t, `{"method":"trace"}`)
	defer os.Remove(f)

	ep := newEndpointWithFile(f, "/trace-ep", "TRACE")
	h, r := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("TRACE", "/trace-ep", nil)
	r.ServeHTTP(w, req)
	_ = w
}

// ── handleRequest – no matching endpoint returns 404 ─────────────────────────

func TestHandleRequest_NoMatchingEndpoint_404(t *testing.T) {
	f := writeTempFile(t, `{"ok":true}`)
	defer os.Remove(f)

	ep := buildDefaultEndpoint(f, "/known", "GET")
	hh := NewMockHandler(buildTestConfigManager(&config.Config{Endpoints: []config.Endpoint{ep}}))
	r2 := gin.New()
	hh.RegisterRoutes(r2)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/unknown-path", nil)
	r2.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("unmatched path status = %d, want 404", w.Code)
	}
}
