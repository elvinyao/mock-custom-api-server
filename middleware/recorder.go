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
	body *bytes.Buffer
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
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

		// Read full request body so downstream handlers (e.g. proxy) receive it intact,
		// then store a (possibly truncated) copy for recording.
		var reqBody string
		if recordBody && c.Request.Body != nil && c.Request.Body != http.NoBody {
			fullBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore the full body for downstream handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(fullBytes))
				// Store truncated version for recording
				if len(fullBytes) > maxBodyBytes {
					reqBody = string(fullBytes[:maxBodyBytes]) + "...[truncated]"
				} else {
					reqBody = string(fullBytes)
				}
			}
		} else if c.Request.Body != nil && c.Request.Body != http.NoBody {
			// Not recording body, but still need to make body re-readable for downstream
			fullBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(fullBytes))
			}
		}

		// Wrap response writer to capture body
		bw := &bodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = bw

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
		for k := range bw.Header() {
			respHeaders[k] = bw.Header().Get(k)
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
		if recordBody {
			respBodyStr = bw.body.String()
			if len(respBodyStr) > maxBodyBytes {
				respBodyStr = respBodyStr[:maxBodyBytes] + "...[truncated]"
			}
		}

		entry := &recorder.RecordedRequest{
			Timestamp:       start,
			Method:          c.Request.Method,
			Path:            path,
			Query:           c.Request.URL.RawQuery,
			RequestHeaders:  reqHeaders,
			RequestBody:     reqBody,
			ResponseStatus:  c.Writer.Status(),
			ResponseHeaders: respHeaders,
			ResponseBody:    respBodyStr,
			DurationMs:      durationMs,
			MatchedRule:     matchedRule,
		}

		rec.Record(entry)
	}
}
