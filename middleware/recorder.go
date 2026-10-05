package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"mock-api-server/recorder"

	"github.com/gin-gonic/gin"
)

// bodyWriter wraps gin.ResponseWriter to capture response body
type bodyWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	limit     int
	truncated bool
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	remaining := w.limit - w.body.Len()
	if n > remaining {
		w.truncated = true
	}
	if remaining > n {
		remaining = n
	}
	if remaining > 0 {
		w.body.Write(b[:remaining])
	}
	return n, err
}

func (w *bodyWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	remaining := w.limit - w.body.Len()
	if n > remaining {
		w.truncated = true
	}
	if remaining > n {
		remaining = n
	}
	if remaining > 0 {
		w.body.WriteString(s[:remaining])
	}
	return n, err
}

type restoredBody struct {
	prefix    *bytes.Reader
	remainder io.Reader
	readErr   error
	io.Closer
}

// Read replays the captured bytes and any error consumed while peeking before
// resuming the original stream. A failed read must never become a clean EOF.
func (b *restoredBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.prefix.Len() > 0 {
		n, err := b.prefix.Read(p)
		if b.prefix.Len() == 0 && b.readErr != nil {
			err, b.readErr = b.readErr, nil
		}
		return n, err
	}
	if b.readErr != nil {
		err := b.readErr
		b.readErr = nil
		return 0, err
	}
	return b.remainder.Read(p)
}

// RequestRecorder returns a middleware that records requests/responses into the store.
// recordBody controls whether request/response bodies are captured.
// maxBodyBytes limits how many bytes are stored per body (the full body is always
// forwarded to downstream handlers unchanged).
func RequestRecorder(rec *recorder.Recorder, recordBody bool, maxBodyBytes int, excludePaths []string) gin.HandlerFunc {
	if rec == nil {
		return func(c *gin.Context) { c.Next() }
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = 65536
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip excluded paths
		for _, excluded := range excludePaths {
			if strings.HasPrefix(path, excluded) {
				c.Next()
				return
			}
		}

		start := time.Now()

		// Peek only at the bounded prefix. Replay it before the unread original
		// stream so downstream handlers receive all bytes and errors, and retain
		// body closing.
		var reqBody string
		reqRecorded := recordBody
		reqTruncated := false
		if recordBody && c.Request.Body != nil && c.Request.Body != http.NoBody {
			original := c.Request.Body
			limit := int64(maxBodyBytes)
			if limit < 1<<63-1 {
				limit++
			}
			prefix, err := io.ReadAll(io.LimitReader(original, limit))
			c.Request.Body = &restoredBody{prefix: bytes.NewReader(prefix), remainder: original, readErr: err, Closer: original}
			reqRecorded = err == nil
			reqTruncated = len(prefix) > maxBodyBytes
			if reqTruncated {
				reqBody = string(prefix[:maxBodyBytes]) + "...[truncated]"
			} else {
				reqBody = string(prefix)
			}
		}

		// Disabled recording leaves both body streams untouched.
		var bw *bodyWriter
		if recordBody {
			bw = &bodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}, limit: maxBodyBytes}
			c.Writer = bw
		}

		// Process request
		c.Next()

		durationMs := time.Since(start).Milliseconds()

		// Collect request headers
		reqHeaders := make(map[string]string)
		for k := range c.Request.Header {
			reqHeaders[k] = c.Request.Header.Get(k)
		}

		// Collect response headers
		respHeaders := make(map[string]string)
		for k := range c.Writer.Header() {
			respHeaders[k] = c.Writer.Header().Get(k)
		}

		// Get matched rule from context
		matchedRule := ""
		if v, ok := c.Get("matched_rule"); ok {
			if s, ok := v.(string); ok {
				matchedRule = s
			}
		}

		// Capture response body (truncate stored copy if too large)
		var respBodyStr string
		if bw != nil {
			respBodyStr = bw.body.String()
			if bw.truncated {
				respBodyStr += "...[truncated]"
			}
		}
		matchTrace, _ := c.Get("match_trace")

		entry := &recorder.RecordedRequest{
			Timestamp:            start,
			Method:               c.Request.Method,
			Path:                 path,
			Query:                c.Request.URL.RawQuery,
			RequestHeaders:       reqHeaders,
			RequestBody:          reqBody,
			RequestBodyRecorded:  reqRecorded,
			RequestBodyTruncated: reqTruncated,
			ResponseStatus:       c.Writer.Status(),
			ResponseHeaders:      respHeaders,
			ResponseBody:         respBodyStr,
			DurationMs:           durationMs,
			MatchedRule:          matchedRule,
			MatchTrace:           matchTrace,
		}

		rec.Record(entry)
	}
}
