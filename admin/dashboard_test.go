package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"gopkg.in/yaml.v3"
	"mock-api-server/config"
	"mock-api-server/recorder"
)

func TestListRequests_FiltersBeforePagination(t *testing.T) {
	h, _, rec, _ := newTestHandler()
	rec.Record(&recorder.RecordedRequest{Method: "POST", Path: "/Older/needle", ResponseStatus: 404})
	rec.Record(&recorder.RecordedRequest{Method: "POST", Path: "/new/needle", ResponseStatus: 403})
	for range 60 {
		rec.Record(&recorder.RecordedRequest{Method: "GET", Path: "/recent", ResponseStatus: 200})
	}
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests?limit=1&offset=1&method=post&path=NEEDLE&status=4xx", nil)
	var result struct {
		Total    int                        `json:"total"`
		Requests []recorder.RecordedRequest `json:"requests"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Requests) != 1 || result.Requests[0].Path != "/Older/needle" {
		t.Fatalf("filter result: %s", w.Body.String())
	}
	w = doRequest(r, "GET", "/admin/requests?status=404", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("exact status filter: %s", w.Body.String())
	}
}

func TestEndpointMutations_RejectInvalidRulesWithoutChangingRuntime(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.AddRuntimeEndpoint(config.Endpoint{Method: "GET", Path: "/keep"})
	r := setupRouter(h)
	invalid := config.Endpoint{Method: "GET", Path: "/bad", Rules: []config.Rule{{Conditions: []config.Condition{{Selector: "missing", MatchType: "exact", Value: "x"}}}}}
	for _, method := range []string{"POST", "PUT"} {
		path := "/admin/endpoints"
		if method == "PUT" {
			path += "/0"
		}
		w := doRequest(r, method, path, invalid)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s status=%d body=%s", method, w.Code, w.Body.String())
		}
		eps := cm.GetRuntimeEndpoints()
		if len(eps) != 1 || eps[0].Path != "/keep" {
			t.Fatalf("invalid edit changed endpoints: %#v", eps)
		}
	}
}

func TestExportEndpoint_CompleteYAMLWithoutRuntimeMutation(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	r := setupRouter(h)
	ep := config.Endpoint{Path: "/export", Method: "POST", Description: "complete", Selectors: []config.Selector{{Name: "q", Type: "query", Key: "q"}}, Default: config.ResponseConfig{InlineBody: "hello", StatusCode: 201, Headers: map[string]string{"X-Result": "ok"}, Template: &config.TemplateConfig{Enabled: true, Engine: "go"}}, Rules: []config.Rule{{Conditions: []config.Condition{{Selector: "q", MatchType: "exact", Value: "value"}}, ResponseConfig: config.ResponseConfig{InlineBody: "rule", StatusCode: 200}}}}
	w := doRequest(r, "POST", "/admin/endpoints/export", ep)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var doc struct {
		Endpoints []config.Endpoint `yaml:"endpoints"`
	}
	if err := yaml.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Endpoints) != 1 || doc.Endpoints[0].Default.InlineBody != "hello" || doc.Endpoints[0].Rules[0].InlineBody != "rule" || doc.Endpoints[0].Selectors[0].Key != "q" {
		t.Fatalf("incomplete YAML: %s", w.Body.String())
	}
	if len(cm.GetRuntimeEndpoints()) != 0 {
		t.Fatal("export persisted endpoint")
	}
}

func TestOpenAPISpec_UsesCurrentOrigin(t *testing.T) {
	h, cm, _, _ := newTestHandler()
	cm.SetConfig(&config.Config{Port: 9999})
	w := doRequest(setupRouter(h), "GET", "/admin/openapi.json", nil)
	var spec struct {
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if len(spec.Servers) != 1 || spec.Servers[0].URL != "/" {
		t.Fatalf("servers=%#v", spec.Servers)
	}
}
