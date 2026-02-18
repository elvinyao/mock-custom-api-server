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

// RequestRecorder returns a middleware that records requests/responses into the store
func RequestRecorder(rec *recorder.Recorder, maxBodyBytes int, excludePaths []string) gin.HandlerFunc {
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

		// Read request body (with limit)
		var reqBody string
		if c.Request.Body != nil && c.Request.Body != http.NoBody {
			limited := io.LimitReader(c.Request.Body, int64(maxBodyBytes))
			bodyBytes, err := io.ReadAll(limited)
			if err == nil {
				reqBody = string(bodyBytes)
			}
			// Restore body for downstream handlers
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
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

		// Capture response body (truncate if too large)
		respBodyStr := bw.body.String()
		if len(respBodyStr) > maxBodyBytes {
			respBodyStr = respBodyStr[:maxBodyBytes] + "...[truncated]"
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
