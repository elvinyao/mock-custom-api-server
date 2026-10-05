package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfig_RejectsInvalidEndpointBehavior(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ok.json")
	if err := os.WriteFile(file, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, field string
		change      func(*Endpoint)
	}{
		{"relative path", "path", func(e *Endpoint) { e.Path = "relative" }},
		{"nonterminal wildcard", "path", func(e *Endpoint) { e.Path = "/a/*rest/b" }},
		{"empty param", "path", func(e *Endpoint) { e.Path = "/a/:" }},
		{"duplicate param", "path", func(e *Endpoint) { e.Path = "/:id/:id" }},
		{"method", "method", func(e *Endpoint) { e.Method = "BAD METHOD" }},
		{"status", "default.status_code", func(e *Endpoint) { e.Default.StatusCode = 99 }},
		{"delay", "default.delay_ms", func(e *Endpoint) { e.Default.DelayMs = -1 }},
		{"delay overflow", "default.delay_ms", func(e *Endpoint) { e.Default.DelayMs = math.MaxInt }},
		{"rule logic", "rules[0].condition_logic", func(e *Endpoint) { e.Rules = []Rule{{ConditionLogic: "xor"}} }},
		{"group logic", "condition_groups[0].logic", func(e *Endpoint) { e.Rules = []Rule{{ConditionGroups: []ConditionGroup{{Logic: "xor"}}}} }},
		{"nested reference", "condition_groups[0].conditions[0].selector", func(e *Endpoint) {
			e.Rules = []Rule{{ConditionGroups: []ConditionGroup{{Conditions: []Condition{{Selector: "missing", MatchType: "exact"}}}}}}
		}},
		{"nested regex", "condition_groups[0].conditions[0].value", func(e *Endpoint) {
			e.Rules = []Rule{{ConditionGroups: []ConditionGroup{{Conditions: []Condition{{Selector: "q", MatchType: "regex", Value: "["}}}}}}
		}},
		{"range", "conditions[0].value", func(e *Endpoint) {
			e.Rules = []Rule{{Conditions: []Condition{{Selector: "q", MatchType: "range", Value: "[z,2]"}}}}
		}},
		{"negative weight", "weight", func(e *Endpoint) {
			e.Default.RandomResponses = &RandomResponses{Enabled: true, Files: []RandomResponse{{File: file, Weight: -1}}}
		}},
		{"weight overflow", "weight", func(e *Endpoint) {
			e.Default.RandomResponses = &RandomResponses{Enabled: true, Files: []RandomResponse{{File: file, Weight: math.MaxInt}, {File: file, Weight: 1}}}
		}},
		{"random status", "status_code", func(e *Endpoint) {
			e.Default.RandomResponses = &RandomResponses{Enabled: true, Files: []RandomResponse{{File: file, StatusCode: 600}}}
		}},
		{"random delay", "delay_ms", func(e *Endpoint) {
			e.Default.RandomResponses = &RandomResponses{Enabled: true, Files: []RandomResponse{{File: file, DelayMs: -1}}}
		}},
		{"rule random file", "random_responses", func(e *Endpoint) {
			e.Rules = []Rule{{ResponseConfig: ResponseConfig{RandomResponses: &RandomResponses{Enabled: true, Files: []RandomResponse{{File: "/missing.json"}}}}}}
		}},
		{"template syntax", "default.inline_body", func(e *Endpoint) {
			e.Default.InlineBody = "{{if}}"
			e.Default.Template = &TemplateConfig{Enabled: true, Engine: "go"}
		}},
		{"header syntax", "default.headers", func(e *Endpoint) {
			e.Default.Headers = map[string]string{"X-Test": "{{if}}"}
			e.Default.Template = &TemplateConfig{Enabled: true, Engine: "go"}
		}},
		{"template engine", "default.template.engine", func(e *Endpoint) { e.Default.Template = &TemplateConfig{Enabled: true, Engine: "typo"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ep := Endpoint{Path: "/test", Method: "GET", Selectors: []Selector{{Name: "q", Type: "query", Key: "q"}}}
			tc.change(&ep)
			got := strings.Join(ValidateConfig(&Config{Endpoints: []Endpoint{ep}}), "\n")
			if !strings.Contains(got, tc.field) {
				t.Fatalf("expected diagnostic containing %q, got %q", tc.field, got)
			}
		})
	}
}

func TestValidateConfig_ContainsAndSupportedPaths(t *testing.T) {
	for _, path := range []string{"/", "/literal", "/:id", "/files/*rest", "/*"} {
		ep := Endpoint{Path: path, Method: "GET", Selectors: []Selector{{Name: "q", Type: "query", Key: "q"}}, Rules: []Rule{{Conditions: []Condition{{Selector: "q", MatchType: "contains", Value: "yes"}}}}}
		if got := ValidateConfig(&Config{Endpoints: []Endpoint{ep}}); len(got) != 0 {
			t.Errorf("%s: unexpected diagnostics: %v", path, got)
		}
	}
}

func TestValidateConfig_EndpointSourceFile(t *testing.T) {
	dir := t.TempDir()
	epPath := filepath.Join(dir, "endpoints.yaml")
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte("endpoints: [endpoints.yaml]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(epPath, []byte("path: /test\nmethod: GET\ndefault:\n  status_code: 999\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(ValidateConfig(cfg), "\n")
	if !strings.Contains(got, epPath) || !strings.Contains(got, "default.status_code") {
		t.Fatalf("diagnostic lacks source/field: %q", got)
	}
}

func TestLoadValidatedConfig_RejectsUnknownFields(t *testing.T) {
	for _, content := range []string{
		"port: 8080\nloging: {}\n",
		"endpoints:\n  - path: /test\n    method: GET\n    default:\n      status_cdoe: 200\n",
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadValidatedConfig(path); err == nil {
			t.Fatal("unknown YAML field accepted")
		}
	}
}

func TestLoadValidatedConfig_UnknownFieldHasOriginalLocation(t *testing.T) {
	for _, tc := range []struct {
		name, content, field string
		line                 int
		external             bool
	}{
		{"inline", "# original comment\n\nport: 8080\nendpoints:\n  - path: /test\n    method: GET\n    default:\n      status_cdoe: 200\n", "endpoints[0].default.status_cdoe", 8, false},
		{"external wrapped", "# original comment\n\nendpoints:\n  - path: /test\n    method: GET\n    default:\n      status_cdoe: 200\n", "endpoints[0].default.status_cdoe", 7, true},
		{"external paths", "# original comment\n\npaths:\n  - path: /test\n    method: GET\n    rules:\n      - status_cdoe: 200\n", "paths[0].rules[0].status_cdoe", 7, true},
		{"external sequence", "# original comment\n\n- path: /test\n  method: GET\n  default:\n    status_cdoe: 200\n", "endpoints[0].default.status_cdoe", 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mainPath := filepath.Join(t.TempDir(), "config.yaml")
			sourcePath := mainPath
			if tc.external {
				sourcePath = filepath.Join(filepath.Dir(mainPath), "endpoint.yaml")
				if err := os.WriteFile(mainPath, []byte("endpoints: [endpoint.yaml]\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(sourcePath, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadValidatedConfig(mainPath)
			if err == nil {
				t.Fatal("unknown field accepted")
			}
			for _, want := range []string{sourcePath, tc.field, fmt.Sprintf("line %d", tc.line)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("diagnostic must contain %q: %v", want, err)
				}
			}
		})
	}
}

func TestLoadValidatedConfig_InvalidValueHasFieldPath(t *testing.T) {
	for _, tc := range []struct{ name, value, field string }{
		{"scalar type", "status_code: nope", "status_code"},
		{"sequence shape", "delay_ms: []", "delay_ms"},
		{"mapping shape", "delay_ms: {}", "delay_ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			content := "# original comment\n\nendpoints:\n  - path: /test\n    method: GET\n    default:\n      " + tc.value + "\n"
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadValidatedConfig(path)
			if err == nil {
				t.Fatal("invalid field value accepted")
			}
			for _, want := range []string{path, "endpoints[0].default." + tc.field, "line 7"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("diagnostic must contain %q: %v", want, err)
				}
			}
		})
	}
}

func TestValidateEndpoint_SimpleLiteralSelectorNames(t *testing.T) {
	for _, name := range []string{"user-id", "user.id", "使用者", "123", "user name", "a{b}c", "user/id:kind"} {
		t.Run(name, func(t *testing.T) {
			placeholder := "{{." + name + "}}"
			ep := Endpoint{Path: "/test", Method: "GET", Selectors: []Selector{{Name: name, Type: "query", Key: "value"}}, Default: ResponseConfig{
				InlineBody: placeholder, Headers: map[string]string{"X-Value": placeholder}, Template: &TemplateConfig{Enabled: true, Engine: "simple"},
			}}
			if err := ValidateEndpoint(ep); err != nil {
				t.Fatalf("existing literal selector name rejected: %v", err)
			}
		})
	}
}

func TestValidateEndpoint_SimpleNestedJSON(t *testing.T) {
	ep := Endpoint{Path: "/test", Method: "GET", Selectors: []Selector{{Name: "id", Type: "query", Key: "id"}}, Default: ResponseConfig{
		InlineBody: `{"data":{"id":"{{.id}}"}}`, Template: &TemplateConfig{Enabled: true, Engine: "simple"},
	}}
	if err := ValidateEndpoint(ep); err != nil {
		t.Fatalf("valid nested JSON rejected: %v", err)
	}
}
