package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"mock-api-server/config"
	"mock-api-server/middleware"
	"mock-api-server/recorder"

	"github.com/gin-gonic/gin"
)

// A client body can return partial bytes and a terminal error together, then EOF.
// Both recording and selector extraction must preserve that failure.
type partialErrorBody struct{ read bool }

func (b *partialErrorBody) Read(p []byte) (int, error) {
	if b.read {
		return 0, io.EOF
	}
	b.read = true
	return copy(p, "partial"), errors.New("incomplete client body")
}
func (*partialErrorBody) Close() error { return nil }

func TestHandleRequest_RejectsPartialBodyBeforeProxy(t *testing.T) {
	for _, recording := range []string{"none", "disabled", "enabled"} {
		t.Run(recording, func(t *testing.T) {
			var upstreamCalls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { upstreamCalls.Add(1); fmt.Fprint(w, "upstream") }))
			defer upstream.Close()
			ep := config.Endpoint{Path: "/proxy", Method: "POST", Mode: "proxy", Proxy: config.ProxyConfig{Target: upstream.URL, FallbackOnError: true}, Default: config.ResponseConfig{InlineBody: "fallback"}}
			r := gin.New()
			rec := recorder.New(10)
			if recording != "none" {
				r.Use(middleware.RequestRecorder(rec, recording == "enabled", 32, nil))
			}
			var trace MatchTrace
			r.Use(func(c *gin.Context) {
				c.Next()
				if value, ok := c.Get("match_trace"); ok {
					trace, _ = value.(MatchTrace)
				}
			})
			h, _ := buildHandlerRouter(t, []config.Endpoint{ep})
			h.RegisterRoutes(r)
			req := httptest.NewRequest("POST", "/proxy", nil)
			req.Body = &partialErrorBody{}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status=%d body=%s; want 400", w.Code, w.Body.String())
			}
			if upstreamCalls.Load() != 0 {
				t.Errorf("partial request reached upstream %d times", upstreamCalls.Load())
			}
			if trace.ResponseSource != "error" || trace.SelectedRule != "error" {
				t.Errorf("trace=%+v; want local request error", trace)
			}
			if recording != "none" {
				entries := rec.List(1, 0)
				if len(entries) != 1 || entries[0].RequestBodyRecorded {
					t.Errorf("failed body must not be replayable: %v", entries)
				}
			}
		})
	}
}

func TestMatchTrace_DistinguishesLocalProxyErrors(t *testing.T) {
	for _, scenario := range []string{"bad target", "connection failure", "truncated upstream", "upstream 400", "upstream 502"} {
		t.Run(scenario, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch scenario {
				case "truncated upstream":
					w.Header().Set("Content-Length", "100")
				case "upstream 400":
					w.WriteHeader(400)
				case "upstream 502":
					w.WriteHeader(502)
				}
				fmt.Fprint(w, "upstream")
			}))
			defer upstream.Close()
			target := upstream.URL
			if scenario == "bad target" {
				target = "http://%"
			}
			if scenario == "connection failure" {
				upstream.Close()
			}
			ep := config.Endpoint{Path: "/proxy", Method: "GET", Mode: "proxy", Proxy: config.ProxyConfig{Target: target}}
			trace, _, w := captureMatchTrace(t, ep, "/proxy")
			wantSource := "proxy_error:" + target
			wantStatus := http.StatusBadGateway
			if scenario == "upstream 400" {
				wantSource = "proxy:" + target
				wantStatus = 400
			}
			if scenario == "upstream 502" {
				wantSource = "proxy:" + target
			}
			if w.Code != wantStatus || trace["response_source"] != wantSource {
				t.Errorf("status=%d source=%v; want %d %q", w.Code, trace["response_source"], wantStatus, wantSource)
			}
		})
	}
}
