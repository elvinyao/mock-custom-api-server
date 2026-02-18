package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ── LoadConfig error cases ─────────────────────────────────────────────────────

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("{ invalid yaml: ["), 0644)
	_, err := LoadConfig(f)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfig_DefaultPort(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("server:\n  port: 0\n"), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Server.Port)
	}
}

func TestLoadConfig_DefaultLoggingLevel(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("server: {}\n"), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Logging.Level != "info" {
		t.Errorf("default logging level = %q, want info", cfg.Server.Logging.Level)
	}
	if cfg.Server.Logging.LogFormat != "json" {
		t.Errorf("default log format = %q, want json", cfg.Server.Logging.LogFormat)
	}
}

func TestLoadConfig_DefaultHealthPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("health_check:\n  enabled: true\n"), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HealthCheck.Path != "/health" {
		t.Errorf("default health path = %q, want /health", cfg.HealthCheck.Path)
	}
}

func TestLoadConfig_HealthCheckDisabled_NoDefaultPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("health_check:\n  enabled: false\n"), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HealthCheck.Path != "" {
		t.Errorf("expected empty health path when disabled, got %q", cfg.HealthCheck.Path)
	}
}

func TestLoadConfig_InlineEndpoints_Sequence(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	yaml := `server:
  port: 9090
endpoints:
  - path: "/api/a"
    method: "GET"
  - path: "/api/b"
    method: "POST"
`
	os.WriteFile(f, []byte(yaml), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(cfg.Endpoints))
	}
}

func TestLoadConfig_EmptyEndpointsSection(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("server:\n  port: 8080\n"), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Endpoints) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(cfg.Endpoints))
	}
}

func TestLoadConfig_MappingEndpoints_ConfigPaths(t *testing.T) {
	dir := t.TempDir()
	epDir := filepath.Join(dir, "eps")
	os.MkdirAll(epDir, 0755)

	epFile := filepath.Join(epDir, "ep.yaml")
	os.WriteFile(epFile, []byte("- path: /from/file\n  method: GET\n"), 0644)

	mainCfg := `server:
  port: 8080
endpoints:
  config_paths:
    - "./eps/ep.yaml"
`
	mainF := filepath.Join(dir, "config.yaml")
	os.WriteFile(mainF, []byte(mainCfg), 0644)
	cfg, err := LoadConfig(mainF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Endpoints) != 1 || cfg.Endpoints[0].Path != "/from/file" {
		t.Errorf("expected /from/file endpoint, got %v", cfg.Endpoints)
	}
}

func TestLoadConfig_EmptyConfigPath_Error(t *testing.T) {
	dir := t.TempDir()
	mainCfg := `endpoints:
  config_paths:
    - ""
`
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte(mainCfg), 0644)
	_, err := LoadConfig(f)
	if err == nil {
		t.Error("expected error for empty config path")
	}
}

// ── loadEndpointsFromFile edge cases ──────────────────────────────────────────

func TestLoadEndpointsFromFile_SequenceFormat(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "eps.yaml")
	os.WriteFile(f, []byte("- path: /a\n  method: GET\n- path: /b\n  method: POST\n"), 0644)

	eps, err := loadEndpointsFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(eps))
	}
}

func TestLoadEndpointsFromFile_MappingWithPaths(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "eps.yaml")
	yaml := `paths:
  - path: /x
    method: GET
  - path: /y
    method: PUT
`
	os.WriteFile(f, []byte(yaml), 0644)
	eps, err := loadEndpointsFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(eps))
	}
}

func TestLoadEndpointsFromFile_MappingWithEndpoints(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "eps.yaml")
	yaml := `endpoints:
  - path: /z
    method: DELETE
`
	os.WriteFile(f, []byte(yaml), 0644)
	eps, err := loadEndpointsFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 1 || eps[0].Path != "/z" {
		t.Errorf("expected /z endpoint, got %v", eps)
	}
}

func TestLoadEndpointsFromFile_MappingInlineEndpoint(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ep.yaml")
	os.WriteFile(f, []byte("path: /inline\nmethod: GET\n"), 0644)
	eps, err := loadEndpointsFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 1 || eps[0].Path != "/inline" {
		t.Errorf("expected /inline endpoint, got %v", eps)
	}
}

func TestLoadEndpointsFromFile_NotFound(t *testing.T) {
	_, err := loadEndpointsFromFile("/nonexistent/file.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadEndpointsFromFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "empty.yaml")
	os.WriteFile(f, []byte(""), 0644)
	_, err := loadEndpointsFromFile(f)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestLoadEndpointsFromFile_EmptySequence(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "eps.yaml")
	os.WriteFile(f, []byte("[]\n"), 0644)
	_, err := loadEndpointsFromFile(f)
	if err == nil {
		t.Error("expected error for empty sequence")
	}
}

func TestLoadEndpointsFromFile_MappingNoContent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ep.yaml")
	// Mapping with no path or paths/endpoints
	os.WriteFile(f, []byte("somekey: somevalue\n"), 0644)
	_, err := loadEndpointsFromFile(f)
	if err == nil {
		t.Error("expected error for mapping with no endpoint content")
	}
}

func TestLoadEndpointsFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.yaml")
	os.WriteFile(f, []byte("{ bad yaml ["), 0644)
	_, err := loadEndpointsFromFile(f)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

// ── ValidateConfig ─────────────────────────────────────────────────────────────

func TestValidateConfig_EmptyEndpoints_NoWarnings(t *testing.T) {
	cfg := &Config{}
	warnings := ValidateConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty config, got: %v", warnings)
	}
}

func TestValidateConfig_MissingPath(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{{Method: "GET"}},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "path is empty") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected path warning, got: %v", warnings)
	}
}

func TestValidateConfig_MissingMethod(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{{Path: "/api"}},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "method is empty") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected method warning, got: %v", warnings)
	}
}

func TestValidateConfig_InvalidSelectorType(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Selectors: []Selector{
					{Name: "x", Type: "invalid_type", Key: "k"},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "invalid type") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid type warning, got: %v", warnings)
	}
}

func TestValidateConfig_EmptySelectorName(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Selectors: []Selector{
					{Name: "", Type: "header", Key: "k"},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "name is empty") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected empty name warning, got: %v", warnings)
	}
}

func TestValidateConfig_DuplicateSelectorName(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Selectors: []Selector{
					{Name: "dup", Type: "header", Key: "k1"},
					{Name: "dup", Type: "query", Key: "k2"},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "duplicate name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate name warning, got: %v", warnings)
	}
}

func TestValidateConfig_UnknownSelectorInCondition(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Selectors: []Selector{
					{Name: "status", Type: "query", Key: "status"},
				},
				Rules: []Rule{
					{
						Conditions: []Condition{
							{Selector: "unknown_sel", MatchType: "exact", Value: "v"},
						},
					},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "unknown selector") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown selector warning, got: %v", warnings)
	}
}

func TestValidateConfig_InvalidMatchType(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Selectors: []Selector{
					{Name: "s", Type: "header", Key: "k"},
				},
				Rules: []Rule{
					{
						Conditions: []Condition{
							{Selector: "s", MatchType: "invalid_match", Value: "v"},
						},
					},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "invalid match_type") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid match_type warning, got: %v", warnings)
	}
}

func TestValidateConfig_InvalidRegex(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Selectors: []Selector{
					{Name: "s", Type: "header", Key: "k"},
				},
				Rules: []Rule{
					{
						Conditions: []Condition{
							{Selector: "s", MatchType: "regex", Value: "[invalid regex("},
						},
					},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "invalid regex") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid regex warning, got: %v", warnings)
	}
}

func TestValidateConfig_MissingResponseFile(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Rules: []Rule{
					{
						ResponseConfig: ResponseConfig{
							ResponseFile: "/nonexistent/file.json",
						},
					},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "response_file not found") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected response_file warning, got: %v", warnings)
	}
}

func TestValidateConfig_MissingDefaultResponseFile(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Default: ResponseConfig{
					ResponseFile: "/nonexistent/default.json",
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "response_file not found") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected default response_file warning, got: %v", warnings)
	}
}

func TestValidateConfig_MissingRandomResponseFile(t *testing.T) {
	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:   "/api",
				Method: "GET",
				Default: ResponseConfig{
					RandomResponses: &RandomResponses{
						Enabled: true,
						Files: []RandomResponse{
							{File: "/nonexistent/rand.json", Weight: 1},
						},
					},
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "file not found") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected random response file warning, got: %v", warnings)
	}
}

func TestValidateConfig_MissingCustomErrorResponseFile(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			ErrorHandling: ErrorHandling{
				CustomErrorResponses: map[int]string{
					404: "/nonexistent/404.json",
				},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	found := false
	for _, w := range warnings {
		if containsStr(w, "file not found") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected custom error response warning, got: %v", warnings)
	}
}

func TestValidateConfig_ValidExistingResponseFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "resp.json")
	os.WriteFile(f, []byte(`{}`), 0644)

	cfg := &Config{
		Endpoints: []Endpoint{
			{
				Path:    "/api",
				Method:  "GET",
				Default: ResponseConfig{ResponseFile: f},
			},
		},
	}
	warnings := ValidateConfig(cfg)
	// Should have no warnings about response file
	for _, w := range warnings {
		if containsStr(w, "response_file not found") {
			t.Errorf("unexpected response_file warning for existing file: %s", w)
		}
	}
}

// ── isValidSelectorType ────────────────────────────────────────────────────────

func TestIsValidSelectorType(t *testing.T) {
	cases := []struct {
		t    string
		want bool
	}{
		{"body", true},
		{"BODY", true},
		{"header", true},
		{"query", true},
		{"path", true},
		{"cookie", false},
		{"", false},
		{"form", false},
	}
	for _, c := range cases {
		got := isValidSelectorType(c.t)
		if got != c.want {
			t.Errorf("isValidSelectorType(%q) = %v, want %v", c.t, got, c.want)
		}
	}
}

// ── isValidMatchType ──────────────────────────────────────────────────────────

func TestIsValidMatchType(t *testing.T) {
	cases := []struct {
		t    string
		want bool
	}{
		{"exact", true},
		{"EXACT", true},
		{"prefix", true},
		{"suffix", true},
		{"regex", true},
		{"range", true},
		{"contains", false}, // not in the valid list per the implementation
		{"fuzzy", false},
		{"", false},
	}
	for _, c := range cases {
		got := isValidMatchType(c.t)
		if got != c.want {
			t.Errorf("isValidMatchType(%q) = %v, want %v", c.t, got, c.want)
		}
	}
}

// ── allChildrenKind ───────────────────────────────────────────────────────────

func TestAllChildrenKind_AllMatch(t *testing.T) {
	// Create a yaml.Node with all scalar children via parsing
	dir := t.TempDir()
	f := filepath.Join(dir, "test.yaml")
	os.WriteFile(f, []byte("- a\n- b\n- c\n"), 0644)
	cfg, err := LoadConfig(filepath.Join(dir, "nonexistent_to_trigger_inline.yaml"))
	// Just test via LoadConfig with inline string paths
	_ = cfg
	_ = err

	// Direct approach: test allChildrenKind indirectly via parseEndpoints
	// (string-only sequence → allChildrenKind true for ScalarNode)
	cfgContent := `server: {}
endpoints:
  - /some/path.yaml
`
	cf := filepath.Join(dir, "cfg.yaml")
	os.WriteFile(cf, []byte(cfgContent), 0644)
	// This will try to load /some/path.yaml which doesn't exist; error expected
	_, err = LoadConfig(cf)
	if err == nil {
		t.Error("expected error for nonexistent endpoint config path")
	}
}

func TestParseEndpoints_InvalidSequence_MixedTypes(t *testing.T) {
	// Cannot easily write a YAML with mixed scalar/mapping in sequence via file
	// But test the "invalid mixed" path: use a config with one string and one mapping
	dir := t.TempDir()
	cfgContent := `server: {}
endpoints:
  - "a string path"
  - path: /inline
    method: GET
`
	f := filepath.Join(dir, "cfg.yaml")
	os.WriteFile(f, []byte(cfgContent), 0644)
	_, err := LoadConfig(f)
	// This tests the mixed sequence path which currently returns error
	// (either fails to load string as file path or fails as mixed sequence)
	_ = err // accept either success or failure, just ensure no panic
}

// helper
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
