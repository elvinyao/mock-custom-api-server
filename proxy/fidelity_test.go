package proxy

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
	"mock-api-server/config"
)

func TestProxyFidelity_RepeatedHeadersAndHopHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Hop-Request") != "" || r.Header.Get("Connection") != "" {
			t.Errorf("hop-by-hop request headers reached upstream: %v", r.Header)
		}
		w.Header().Add("Set-Cookie", "first=1")
		w.Header().Add("Set-Cookie", "second=2")
		w.Header().Set("Connection", "X-Hop-Response")
		w.Header().Set("X-Hop-Response", "local-only")
		fmt.Fprint(w, "ok")
	}))
	defer upstream.Close()
	c, w := buildProxyContext(t, "GET", "/", "", map[string]string{
		"Connection": "X-Hop-Request", "X-Hop-Request": "local-only",
	})
	New().ProxyRequest(c, config.Endpoint{Proxy: config.ProxyConfig{Target: upstream.URL}})
	if got := w.Header().Values("Set-Cookie"); !slices.Equal(got, []string{"first=1", "second=2"}) {
		t.Errorf("cookies = %v, want both original cookies", got)
	}
	if w.Header().Get("Connection") != "" || w.Header().Get("X-Hop-Response") != "" {
		t.Errorf("hop-by-hop response headers leaked: %v", w.Header())
	}
}

func TestProxyFidelity_TruncatedBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		fmt.Fprint(w, "incomplete")
	}))
	defer upstream.Close()
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprintf("fallback_%v", fallback), func(t *testing.T) {
			dir := t.TempDir()
			c, w := buildProxyContext(t, "POST", "/", "original-body", nil)
			handled := New().ProxyRequest(c, config.Endpoint{Proxy: config.ProxyConfig{
				Target: upstream.URL, FallbackOnError: fallback, Record: true, RecordDir: dir,
			}})
			if fallback {
				if handled || c.Writer.Written() {
					t.Fatal("fallback must return before writing a partial upstream response")
				}
				body, err := io.ReadAll(c.Request.Body)
				if err != nil || string(body) != "original-body" {
					t.Fatalf("fallback body = %q, %v", body, err)
				}
			} else if !handled || w.Code != http.StatusBadGateway {
				t.Errorf("handled=%v, status=%d, want handled 502", handled, w.Code)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Errorf("failed upstream body must not be recorded: files=%v err=%v", files, err)
			}
		})
	}
}

type failingRequestBody struct{}

func (failingRequestBody) Read([]byte) (int, error) { return 0, errors.New("broken request body") }
func (failingRequestBody) Close() error             { return nil }

func TestProxyFidelity_RejectsUnreadableRequestBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unreadable client request must not reach upstream")
	}))
	defer upstream.Close()
	c, w := buildProxyContext(t, "POST", "/", "", nil)
	c.Request.Body = failingRequestBody{}
	handled := New().ProxyRequest(c, config.Endpoint{Proxy: config.ProxyConfig{
		Target: upstream.URL, FallbackOnError: true,
	}})
	if !handled || w.Code != http.StatusBadRequest {
		t.Errorf("handled=%v status=%d, want handled 400", handled, w.Code)
	}
}

func TestProxyFidelity_PreservesRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		fmt.Fprint(w, "followed")
	}))
	defer upstream.Close()
	c, w := buildProxyContext(t, "GET", "/", "", nil)
	New().ProxyRequest(c, config.Endpoint{Proxy: config.ProxyConfig{Target: upstream.URL}})
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/next" {
		t.Errorf("redirect lost: status=%d headers=%v", w.Code, w.Header())
	}
}

func TestStubWriter_ConcurrentRecordingsAreDistinctAndComplete(t *testing.T) {
	dir := t.TempDir()
	const count = 24
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/same/path", nil)
			resp := &http.Response{StatusCode: 200 + i, Header: make(http.Header)}
			StubWriter{}.WriteStub(dir, req, resp, nil, []byte(fmt.Sprint(200+i)))
		}(i)
	}
	wg.Wait()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != count*2 {
		t.Fatalf("got %d files, want %d (one pair per recording)", len(files), count*2)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var eps []config.Endpoint
		if err := yaml.Unmarshal(data, &eps); err != nil || len(eps) != 1 {
			t.Fatalf("invalid stub %s: %v", file.Name(), err)
		}
		body, err := os.ReadFile(eps[0].Default.ResponseFile)
		if err != nil || string(body) != fmt.Sprint(eps[0].Default.StatusCode) {
			t.Fatalf("stub/body pair mismatch: %s, body=%q err=%v", file.Name(), body, err)
		}
	}
}
