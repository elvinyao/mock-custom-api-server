package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"mock-api-server/config"
)

func captureMatchTrace(t *testing.T, ep config.Endpoint, path string) (map[string]any, string, *httptest.ResponseRecorder) {
	t.Helper()
	r := gin.New()
	var trace map[string]any
	var responseFile string
	r.Use(func(c *gin.Context) {
		c.Next()
		value, ok := c.Get("match_trace")
		if !ok {
			t.Error("missing match_trace")
			return
		}
		raw, err := json.Marshal(value)
		if err != nil {
			t.Error(err)
			return
		}
		if err = json.Unmarshal(raw, &trace); err != nil {
			t.Error(err)
		}
		responseFile = c.GetString("response_file")
	})
	h, _ := buildHandlerRouter(t, []config.Endpoint{ep})
	h.RegisterRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return trace, responseFile, w
}

func TestMatchTrace_ExplainsConditionsAndGroups(t *testing.T) {
	ep := config.Endpoint{Path: "/org/:id/*rest", Method: "GET", Selectors: []config.Selector{{Name: "id", Type: "path", Key: "id"}, {Name: "rest", Type: "path", Key: "rest"}, {Name: "q", Type: "query", Key: "q"}}, Rules: []config.Rule{
		{Conditions: []config.Condition{{Selector: "q", MatchType: "exact", Value: "no"}}, ResponseConfig: config.ResponseConfig{InlineBody: "wrong"}},
		{ConditionLogic: "OR", Conditions: []config.Condition{{Selector: "id", MatchType: "range", Value: "[1,9]"}, {Selector: "q", MatchType: "exact", Value: "no"}}, ConditionGroups: []config.ConditionGroup{{Logic: "OR", Conditions: []config.Condition{{Selector: "rest", MatchType: "prefix", Value: "/a"}, {Selector: "q", MatchType: "exact", Value: "no"}}}}, ResponseConfig: config.ResponseConfig{InlineBody: "selected"}},
		{ResponseConfig: config.ResponseConfig{InlineBody: "later"}},
	}, Default: config.ResponseConfig{InlineBody: "default"}}
	trace, _, w := captureMatchTrace(t, ep, "/org/1/a?q=yes")
	if trace == nil {
		return
	}
	if w.Body.String() != "selected" || trace["selected_rule"] != "rule_1" || trace["response_source"] != "inline" {
		t.Fatalf("response=%q trace=%v", w.Body.String(), trace)
	}
	selectors := trace["selectors"].(map[string]any)
	if selectors["id"] != "1" || selectors["rest"] != "/a" {
		t.Errorf("selectors=%v", selectors)
	}
	rules := trace["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("rules=%v", rules)
	}
	for i, want := range []bool{false, true, true} {
		if rules[i].(map[string]any)["matched"] != want {
			t.Errorf("rule %d=%v", i, rules[i])
		}
	}
	selected := rules[1].(map[string]any)
	conditions := selected["conditions"].([]any)
	condition := conditions[0].(map[string]any)
	for key, want := range map[string]any{"selector": "id", "actual": "1", "expected": "[1,9]", "match_type": "range", "matched": true} {
		if condition[key] != want {
			t.Errorf("condition %s=%v want %v", key, condition[key], want)
		}
	}
	group := selected["groups"].([]any)[0].(map[string]any)
	if group["logic"] != "or" || group["matched"] != true {
		t.Errorf("group=%v", group)
	}
	values := map[string]string{"id": "1", "rest": "/a", "q": "yes"}
	converted := convertRules(ep.Rules)
	if getRuleIndex(converted, MatchRules(values, converted)) != 1 {
		t.Error("trace diverged from MatchRules")
	}
}

func TestMatchTrace_RecordsSelectedRandomFile(t *testing.T) {
	f := writeTempFile(t, "random")
	defer os.Remove(f)
	ep := config.Endpoint{Path: "/random", Method: "GET", Default: config.ResponseConfig{RandomResponses: &config.RandomResponses{Enabled: true, Files: []config.RandomResponse{{File: f, Weight: 1, StatusCode: 201}}}}}
	trace, file, w := captureMatchTrace(t, ep, "/random")
	if trace == nil {
		return
	}
	if file != f || trace["response_source"] != "random_file:"+f || trace["selected_rule"] != "default" || w.Code != 201 {
		t.Errorf("file=%q trace=%v status=%d", file, trace, w.Code)
	}
}

func TestMatchTrace_ProxyAndFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("upstream")) }))
	defer upstream.Close()
	ep := config.Endpoint{Path: "/proxy", Method: "GET", Mode: "proxy", Proxy: config.ProxyConfig{Target: upstream.URL}, Default: config.ResponseConfig{InlineBody: "fallback"}}
	trace, _, w := captureMatchTrace(t, ep, "/proxy")
	if trace == nil {
		return
	}
	if w.Body.String() != "upstream" || trace["response_source"] != "proxy:"+upstream.URL {
		t.Errorf("trace=%v body=%q", trace, w.Body.String())
	}
	upstream.Close()
	ep.Proxy.FallbackOnError = true
	trace, _, w = captureMatchTrace(t, ep, "/proxy")
	if trace == nil {
		return
	}
	if w.Body.String() != "fallback" || trace["response_source"] != "proxy_fallback:inline" || trace["selected_rule"] != "default" {
		t.Errorf("trace=%v body=%q", trace, w.Body.String())
	}
}

func TestMatchTrace_NotFoundAndError(t *testing.T) {
	ep := config.Endpoint{Path: "/broken", Method: "GET", Default: config.ResponseConfig{ResponseFile: "/file/does/not/exist"}}
	for _, tt := range []struct{ path, selected, source string }{{"/missing", "not_found", "not_found"}, {"/broken", "default", "error"}} {
		trace, _, _ := captureMatchTrace(t, ep, tt.path)
		if trace == nil {
			continue
		}
		if trace["selected_rule"] != tt.selected || trace["response_source"] != tt.source {
			t.Errorf("trace=%v", trace)
		}
	}
}
