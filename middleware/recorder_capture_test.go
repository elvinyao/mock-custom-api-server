package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"mock-api-server/recorder"
)

type partialBodyFailure struct {
	read bool
	err  error
}

func (b *partialBodyFailure) Read(p []byte) (int, error) {
	if b.read {
		return 0, io.EOF
	}
	b.read = true
	return copy(p, "partial"), b.err
}
func (*partialBodyFailure) Close() error { return nil }

func TestRequestRecorder_PreservesBodyReadError(t *testing.T) {
	rec := recorder.New(10)
	r := gin.New()
	r.Use(RequestRecorder(rec, true, 32, nil))
	bodyError := errors.New("client disconnected")
	r.POST("/test", func(c *gin.Context) {
		got, err := io.ReadAll(c.Request.Body)
		if string(got) != "partial" || !errors.Is(err, bodyError) {
			t.Errorf("downstream body=%q err=%v; want partial bytes and original error", got, err)
		}
		c.Status(400)
	})
	req := httptest.NewRequest("POST", "/test", nil)
	req.Body = &partialBodyFailure{err: bodyError}
	r.ServeHTTP(httptest.NewRecorder(), req)
	if rec.List(1, 0)[0].RequestBodyRecorded {
		t.Error("failed read must not be replayable")
	}
}

type countedBody struct {
	io.Reader
	reads int
}

func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.reads += n
	return n, err
}
func (b *countedBody) Close() error { return nil }

func TestRequestRecorder_DisabledSkipsBuffering(t *testing.T) {
	rec := recorder.New(10)
	r := gin.New()
	r.Use(RequestRecorder(rec, false, 8, nil))
	body := &countedBody{Reader: strings.NewReader("the complete request body")}
	r.POST("/test", func(c *gin.Context) {
		if body.reads != 0 {
			t.Errorf("recorder read %d bytes while body recording disabled", body.reads)
		}
		got, _ := io.ReadAll(c.Request.Body)
		if string(got) != "the complete request body" {
			t.Errorf("downstream body=%q", got)
		}
		c.String(200, "response")
	})
	req := httptest.NewRequest("POST", "/test", nil)
	req.Body = body
	r.ServeHTTP(httptest.NewRecorder(), req)
	encoded, _ := json.Marshal(rec.List(1, 0)[0])
	var entry map[string]any
	json.Unmarshal(encoded, &entry)
	if v, ok := entry["request_body_recorded"]; !ok || v != false {
		t.Errorf("missing explicit disabled body flag: %s", encoded)
	}
	if v, ok := entry["request_body_truncated"]; !ok || v != false {
		t.Errorf("missing explicit truncation flag: %s", encoded)
	}
}

func TestRequestRecorder_BoundedCapturePreservesDownstreamBytes(t *testing.T) {
	rec := recorder.New(10)
	r := gin.New()
	r.Use(RequestRecorder(rec, true, 8, nil))
	payload := strings.Repeat("0123456789", 1000)
	body := &countedBody{Reader: strings.NewReader(payload)}
	r.POST("/test", func(c *gin.Context) {
		if body.reads > 9 {
			t.Errorf("recorder buffered %d bytes before handler; maximum 9", body.reads)
		}
		got, _ := io.ReadAll(c.Request.Body)
		if string(got) != payload {
			t.Errorf("downstream body length=%d want=%d", len(got), len(payload))
		}
		c.Set("match_trace", map[string]any{"selected_rule": "rule_1"})
		c.Writer.WriteString(payload)
		if bw, ok := c.Writer.(*bodyWriter); ok && bw.body.Len() > 8 {
			t.Errorf("response capture retained %d bytes; maximum 8", bw.body.Len())
		}
	})
	req := httptest.NewRequest("POST", "/test", nil)
	req.Body = body
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.String() != payload {
		t.Errorf("downstream response length=%d want=%d", w.Body.Len(), len(payload))
	}
	entry := rec.List(1, 0)[0]
	if entry.RequestBody != "01234567...[truncated]" || entry.ResponseBody != "01234567...[truncated]" {
		t.Errorf("captured request=%q response=%q", entry.RequestBody, entry.ResponseBody)
	}
	encoded, _ := json.Marshal(entry)
	var fields map[string]any
	json.Unmarshal(encoded, &fields)
	if fields["request_body_recorded"] != true || fields["request_body_truncated"] != true {
		t.Errorf("capture metadata=%s", encoded)
	}
	if trace, ok := fields["match_trace"].(map[string]any); !ok || trace["selected_rule"] != "rule_1" {
		t.Errorf("trace not recorded: %s", encoded)
	}
}

func TestRequestRecorder_CompleteEmptyBodyIsReplayable(t *testing.T) {
	rec := recorder.New(10)
	r := gin.New()
	r.Use(RequestRecorder(rec, true, 8, nil))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/test", nil))
	encoded, _ := json.Marshal(rec.List(1, 0)[0])
	var fields map[string]any
	json.Unmarshal(encoded, &fields)
	if fields["request_body_recorded"] != true || fields["request_body_truncated"] != false {
		t.Errorf("empty body must be safely replayable: %s", encoded)
	}
}
