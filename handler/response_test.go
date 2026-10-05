package handler

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuild_FixtureEditsAreVisibleImmediately(t *testing.T) {
	f := filepath.Join(t.TempDir(), "response.json")
	rb := NewResponseBuilder()
	for _, body := range []string{`{"v":1}`, `{"v":2}`} {
		if err := os.WriteFile(f, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		res, err := rb.Build(ResponseBuildConfig{ResponseFile: f}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(res.Body) != body {
			t.Fatalf("body = %q, want edited fixture %q", res.Body, body)
		}
	}
}

func TestBuild_TemplateErrorsAreReturned(t *testing.T) {
	for _, tc := range []struct {
		name, body, engine string
		headers            map[string]string
	}{
		{"parse", "{{if}}", "go", nil},
		{"execute", "{{add .value 1}}", "go", nil},
		{"missing", "{{.missing}}", "go", nil},
		{"simple malformed", "{{.value", "simple", nil},
		{"unknown engine", "body", "typo", nil},
		{"header", "ok", "go", map[string]string{"X-Test": "{{if}}"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewResponseBuilder().Build(ResponseBuildConfig{InlineBody: tc.body, TemplateEnabled: true, TemplateEngine: tc.engine, Headers: tc.headers}, map[string]string{"value": "text"})
			if err == nil {
				t.Fatal("expected template error")
			}
		})
	}
}

func TestBuild_BuiltinValuesSharedByBodyAndHeaders(t *testing.T) {
	for _, engine := range []string{"simple", "go"} {
		t.Run(engine, func(t *testing.T) {
			res, err := NewResponseBuilder().Build(ResponseBuildConfig{InlineBody: "{{.uuid}}/{{.request_id}}/{{.timestamp}}", TemplateEnabled: true, TemplateEngine: engine, Headers: map[string]string{"X-Identity": "{{.uuid}}/{{.request_id}}/{{.timestamp}}"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if string(res.Body) != res.Headers["X-Identity"] {
				t.Fatalf("body and header used different builtins: %s / %s", res.Body, res.Headers["X-Identity"])
			}
		})
	}
}

func TestBuild_ReportsSelectedResponseFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "selected.json")
	if err := os.WriteFile(f, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := NewResponseBuilder().Build(ResponseBuildConfig{ResponseFile: "ignored", RandomResponses: []RandomResponseConfig{{File: f, Weight: 1}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ResponseFile != f {
		t.Fatalf("result must report actual selected response file %q", f)
	}
}

// ── Build ─────────────────────────────────────────────────────────────────────

func TestBuild_NoFile_DefaultStatus200(t *testing.T) {
	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", res.StatusCode)
	}
	if res.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want application/json", res.ContentType)
	}
}

func TestBuild_CustomStatus(t *testing.T) {
	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{StatusCode: 404}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", res.StatusCode)
	}
}

func TestBuild_CustomContentType(t *testing.T) {
	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{ContentType: "text/plain"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", res.ContentType)
	}
	if res.Headers["Content-Type"] != "text/plain" {
		t.Errorf("Content-Type header = %q, want text/plain", res.Headers["Content-Type"])
	}
}

func TestBuild_ReadsResponseFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "resp.json")
	content := `{"hello":"world"}`
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{ResponseFile: f}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res.Body) != content {
		t.Errorf("Body = %q, want %q", string(res.Body), content)
	}
}

func TestBuild_FileNotFound_Error(t *testing.T) {
	rb := NewResponseBuilder()
	_, err := rb.Build(ResponseBuildConfig{ResponseFile: "/nonexistent/file.json"}, nil)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestBuild_TemplateSimpleEngine(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "resp.json")
	if err := os.WriteFile(f, []byte(`{"id":"{{.user_id}}"}`), 0644); err != nil {
		t.Fatal(err)
	}

	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{
		ResponseFile:    f,
		TemplateEnabled: true,
		TemplateEngine:  "simple",
	}, map[string]string{"user_id": "abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res.Body) != `{"id":"abc123"}` {
		t.Errorf("Body = %q, want {\"id\":\"abc123\"}", string(res.Body))
	}
}

func TestBuild_SimpleLiteralNamesInBodyAndHeaders(t *testing.T) {
	content := "{{.user-id}}/{{.user.id}}/{{.使用者}}"
	result, err := NewResponseBuilder().Build(ResponseBuildConfig{
		InlineBody:      content,
		Headers:         map[string]string{"X-User": content},
		TemplateEnabled: true,
		TemplateEngine:  "simple",
	}, map[string]string{"user-id": "one", "user.id": "two", "使用者": "three"})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Body) != "one/two/three" || result.Headers["X-User"] != "one/two/three" {
		t.Fatalf("literal selector names not rendered: %#v", result)
	}
}

func TestBuild_TemplateGoEngine(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "resp.json")
	if err := os.WriteFile(f, []byte(`{"id":"{{index . "user_id"}}"}`), 0644); err != nil {
		t.Fatal(err)
	}

	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{
		ResponseFile:    f,
		TemplateEnabled: true,
		TemplateEngine:  "go",
	}, map[string]string{"user_id": "xyz789"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res.Body) != `{"id":"xyz789"}` {
		t.Errorf("Body = %q, want {\"id\":\"xyz789\"}", string(res.Body))
	}
}

func TestBuild_MergesHeaders(t *testing.T) {
	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{
		Headers: map[string]string{
			"X-Custom": "myval",
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Headers["X-Custom"] != "myval" {
		t.Errorf("X-Custom header = %q, want myval", res.Headers["X-Custom"])
	}
}

func TestBuild_HeaderWithTemplate(t *testing.T) {
	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{
		TemplateEnabled: true,
		TemplateEngine:  "simple",
		Headers: map[string]string{
			"X-User-ID": "{{.uid}}",
		},
	}, map[string]string{"uid": "u-99"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Headers["X-User-ID"] != "u-99" {
		t.Errorf("X-User-ID header = %q, want u-99", res.Headers["X-User-ID"])
	}
}

func TestBuild_DelayMs(t *testing.T) {
	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{DelayMs: 42}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DelayMs != 42 {
		t.Errorf("DelayMs = %d, want 42", res.DelayMs)
	}
}

func TestBuild_RandomResponses_SelectsOne(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.json")
	f2 := filepath.Join(dir, "b.json")
	_ = os.WriteFile(f1, []byte(`{"r":"a"}`), 0644)
	_ = os.WriteFile(f2, []byte(`{"r":"b"}`), 0644)

	rb := NewResponseBuilder()
	res, err := rb.Build(ResponseBuildConfig{
		RandomResponses: []RandomResponseConfig{
			{File: f1, Weight: 1, StatusCode: 200},
			{File: f2, Weight: 1, StatusCode: 201},
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := string(res.Body)
	if body != `{"r":"a"}` && body != `{"r":"b"}` {
		t.Errorf("unexpected body %q", body)
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		t.Errorf("unexpected status %d", res.StatusCode)
	}
}

// ── selectRandomResponse ───────────────────────────────────────────────────────

func TestSelectRandomResponse_Empty(t *testing.T) {
	r := selectRandomResponse(nil)
	if r.File != "" {
		t.Errorf("expected empty RandomResponseConfig for nil input")
	}
}

func TestSelectRandomResponse_ZeroWeights_UsesRand(t *testing.T) {
	responses := []RandomResponseConfig{
		{File: "a.json", Weight: 0},
		{File: "b.json", Weight: 0},
	}
	// Should not panic; should return one of them
	r := selectRandomResponse(responses)
	if r.File != "a.json" && r.File != "b.json" {
		t.Errorf("unexpected file %q", r.File)
	}
}

func TestSelectRandomResponse_WeightedSelection(t *testing.T) {
	// Run many times; file "a" has 100% weight so must always be chosen
	responses := []RandomResponseConfig{
		{File: "a.json", Weight: 100},
		{File: "b.json", Weight: 0},
	}
	for i := 0; i < 50; i++ {
		r := selectRandomResponse(responses)
		if r.File != "a.json" {
			t.Errorf("expected a.json with weight 100, got %q", r.File)
			break
		}
	}
}

func TestSelectRandomResponse_SingleEntry(t *testing.T) {
	responses := []RandomResponseConfig{
		{File: "only.json", Weight: 5, StatusCode: 200},
	}
	r := selectRandomResponse(responses)
	if r.File != "only.json" {
		t.Errorf("expected only.json, got %q", r.File)
	}
}

// ── ApplyDelay ────────────────────────────────────────────────────────────────

func TestApplyDelay_Zero(t *testing.T) {
	start := time.Now()
	ApplyDelay(0)
	if time.Since(start) > 50*time.Millisecond {
		t.Error("ApplyDelay(0) took too long, should not sleep")
	}
}

func TestApplyDelay_Negative(t *testing.T) {
	start := time.Now()
	ApplyDelay(-10)
	if time.Since(start) > 50*time.Millisecond {
		t.Error("ApplyDelay(-10) should not sleep")
	}
}

func TestApplyDelay_Positive(t *testing.T) {
	start := time.Now()
	ApplyDelay(20)
	elapsed := time.Since(start)
	if elapsed < 15*time.Millisecond {
		t.Errorf("ApplyDelay(20) elapsed = %v, expected >= 15ms", elapsed)
	}
}
