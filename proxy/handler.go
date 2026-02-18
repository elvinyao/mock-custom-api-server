package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

// Handler handles proxying requests to upstream targets
type Handler struct{}

// New creates a new proxy Handler
func New() *Handler {
	return &Handler{}
}

// ProxyRequest proxies the incoming request to the configured target.
// If fallback_on_error is true and the proxy fails, returns false so the caller can use mock rules.
func (h *Handler) ProxyRequest(c *gin.Context, ep config.Endpoint) bool {
	cfg := ep.Proxy
	if cfg.Target == "" {
		return false
	}

	targetURL, err := url.Parse(cfg.Target)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("invalid proxy target: %v", err)})
		return true
	}

	// Determine timeout
	timeout := 30 * time.Second
	if cfg.TimeoutMs > 0 {
		timeout = time.Duration(cfg.TimeoutMs) * time.Millisecond
	}

	// Build outgoing request path
	outPath := c.Request.URL.Path
	if cfg.StripPrefix != "" {
		outPath = strings.TrimPrefix(outPath, cfg.StripPrefix)
		if outPath == "" {
			outPath = "/"
		}
	}

	outURL := *targetURL
	outURL.Path = strings.TrimRight(targetURL.Path, "/") + outPath
	outURL.RawQuery = c.Request.URL.RawQuery

	// Read request body
	var reqBodyBytes []byte
	if c.Request.Body != nil {
		reqBodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
	}

	// Build upstream request
	upstreamReq, err := http.NewRequestWithContext(
		c.Request.Context(),
		c.Request.Method,
		outURL.String(),
		bytes.NewBuffer(reqBodyBytes),
	)
	if err != nil {
		if cfg.FallbackOnError {
			return false
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return true
	}

	// Copy original headers
	for k, vv := range c.Request.Header {
		for _, v := range vv {
			upstreamReq.Header.Add(k, v)
		}
	}

	// Inject configured headers
	for k, v := range cfg.Headers {
		upstreamReq.Header.Set(k, v)
	}

	// Fix Host header
	upstreamReq.Host = targetURL.Host

	// Execute
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		if cfg.FallbackOnError {
			return false
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("upstream error: %v", err)})
		return true
	}
	defer resp.Body.Close()

	// Read response body
	respBodyBytes, _ := io.ReadAll(resp.Body)

	// Record stub if configured
	if cfg.Record && cfg.RecordDir != "" {
		StubWriter{}.WriteStub(cfg.RecordDir, c.Request, resp, reqBodyBytes, respBodyBytes)
	}

	// Copy upstream response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Header(k, v)
		}
	}

	// Forward upstream status and body
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(resp.StatusCode, contentType, respBodyBytes)
	return true
}

// NewReverseProxy creates a standard httputil.ReverseProxy for the given target
func NewReverseProxy(target string) (*httputil.ReverseProxy, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(targetURL), nil
}
