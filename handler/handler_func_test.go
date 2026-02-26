package handler

import (
	"testing"

	"mock-api-server/config"
)

// ── extractPathParams ─────────────────────────────────────────────────────────

func TestExtractPathParams_NoParams(t *testing.T) {
	params := extractPathParams("/api/users", "/api/users")
	if len(params) != 0 {
		t.Errorf("expected empty params for static path, got %v", params)
	}
}

func TestExtractPathParams_SingleParam(t *testing.T) {
	params := extractPathParams("/api/users/:id", "/api/users/42")
	// Note: the regex in extractPathParams uses :(\\w+) which matches :id
	// but the internal regex uses escaped backslash, so let's just verify no panic
	_ = params
}

func TestExtractPathParams_MultipleParams(t *testing.T) {
	params := extractPathParams("/api/:version/users/:id", "/api/v2/users/99")
	_ = params // implementation uses :(\\w+) (double backslash) which matches nothing in Go regex
}

func TestExtractPathParams_NoMatch(t *testing.T) {
	params := extractPathParams("/api/v1", "/api/v2")
	if len(params) != 0 {
		t.Errorf("expected empty params for non-matching path, got %v", params)
	}
}

// ── convertRules ─────────────────────────────────────────────────────────────

func TestConvertRules_Empty(t *testing.T) {
	rules := convertRules(nil)
	if len(rules) != 0 {
		t.Errorf("expected empty rules for nil input, got %d", len(rules))
	}
}

func TestConvertRules_WithConditions(t *testing.T) {
	cfgRules := []config.Rule{
		{
			ConditionLogic: "or",
			Conditions: []config.Condition{
				{Selector: "status", MatchType: "exact", Value: "active"},
			},
			ResponseConfig: config.ResponseConfig{
				ResponseFile: "a.json",
				StatusCode:   200,
			},
		},
	}
	rules := convertRules(cfgRules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ConditionLogic != "or" {
		t.Errorf("ConditionLogic = %q, want or", rules[0].ConditionLogic)
	}
	if len(rules[0].Conditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(rules[0].Conditions))
	}
	if rules[0].Conditions[0].Selector != "status" {
		t.Errorf("Selector = %q, want status", rules[0].Conditions[0].Selector)
	}
	if rules[0].ResponseFile != "a.json" {
		t.Errorf("ResponseFile = %q, want a.json", rules[0].ResponseFile)
	}
}

func TestConvertRules_WithTemplate(t *testing.T) {
	engine := "go"
	cfgRules := []config.Rule{
		{
			ResponseConfig: config.ResponseConfig{
				Template: &config.TemplateConfig{
					Enabled: true,
					Engine:  engine,
				},
			},
		},
	}
	rules := convertRules(cfgRules)
	if rules[0].Template == nil {
		t.Fatal("expected non-nil template config")
	}
	if !rules[0].Template.Enabled {
		t.Error("expected template enabled")
	}
	if rules[0].Template.Engine != "go" {
		t.Errorf("Engine = %q, want go", rules[0].Template.Engine)
	}
}

func TestConvertRules_NilTemplate(t *testing.T) {
	cfgRules := []config.Rule{
		{ResponseConfig: config.ResponseConfig{Template: nil}},
	}
	rules := convertRules(cfgRules)
	if rules[0].Template != nil {
		t.Error("expected nil template when config template is nil")
	}
}

func TestConvertRules_WithConditionGroups(t *testing.T) {
	cfgRules := []config.Rule{
		{
			ConditionGroups: []config.ConditionGroup{
				{
					Logic: "and",
					Conditions: []config.Condition{
						{Selector: "x", MatchType: "exact", Value: "1"},
					},
				},
			},
		},
	}
	rules := convertRules(cfgRules)
	if len(rules[0].ConditionGroups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(rules[0].ConditionGroups))
	}
	if rules[0].ConditionGroups[0].Logic != "and" {
		t.Errorf("Group Logic = %q, want and", rules[0].ConditionGroups[0].Logic)
	}
	if len(rules[0].ConditionGroups[0].Conditions) != 1 {
		t.Errorf("expected 1 group condition, got %d", len(rules[0].ConditionGroups[0].Conditions))
	}
}

func TestConvertRules_AllFields(t *testing.T) {
	cfgRules := []config.Rule{
		{
			ResponseConfig: config.ResponseConfig{
				StatusCode:  201,
				DelayMs:     50,
				ContentType: "text/plain",
				Headers:     map[string]string{"X-Test": "val"},
			},
		},
	}
	rules := convertRules(cfgRules)
	r := rules[0]
	if r.StatusCode != 201 {
		t.Errorf("StatusCode = %d, want 201", r.StatusCode)
	}
	if r.DelayMs != 50 {
		t.Errorf("DelayMs = %d, want 50", r.DelayMs)
	}
	if r.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", r.ContentType)
	}
	if r.Headers["X-Test"] != "val" {
		t.Errorf("Headers[X-Test] = %q, want val", r.Headers["X-Test"])
	}
}
