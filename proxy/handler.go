package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

// sharedTransport is a reusable Transport with connection pooling.
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 20,
	IdleConnTimeout:     90 * time.Second,
}

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
		writeProxyError(c, http.StatusBadGateway, fmt.Errorf("invalid proxy target: %w", err))
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
		reqBodyBytes, err = io.ReadAll(c.Request.Body)
		_ = c.Request.Body.Close()
		if err != nil {
			writeProxyError(c, http.StatusBadRequest, fmt.Errorf("failed to read request body: %w", err))
			return true
		}
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
		writeProxyError(c, http.StatusBadGateway, err)
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
	removeHopHeaders(upstreamReq.Header)

	// Fix Host header
	upstreamReq.Host = targetURL.Host

	// Execute using a shared client with connection pooling
	client := &http.Client{
		Timeout: timeout, Transport: sharedTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		if cfg.FallbackOnError {
			return false
		}
		writeProxyError(c, http.StatusBadGateway, fmt.Errorf("upstream error: %w", err))
		return true
	}
	defer resp.Body.Close()

	// Read response body
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		if cfg.FallbackOnError {
			return false
		}
		writeProxyError(c, http.StatusBadGateway, fmt.Errorf("failed to read upstream response: %w", err))
		return true
	}

	// Record stub if configured
	if cfg.Record && cfg.RecordDir != "" {
		if err := (StubWriter{}).WriteStub(cfg.RecordDir, c.Request, resp, reqBodyBytes, respBodyBytes); err != nil {
			_ = c.Error(fmt.Errorf("record proxy interaction: %w", err))
			log.Printf("[WARN] Failed to record proxy interaction: %v", err)
		}
	}

	// Copy upstream response headers
	removeHopHeaders(resp.Header)
	for k, vv := range resp.Header {
		c.Writer.Header().Del(k)
		for _, v := range vv {
			c.Writer.Header().Add(k, v)
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

func writeProxyError(c *gin.Context, status int, err error) {
	c.Set("proxy_error", err.Error())
	c.JSON(status, gin.H{"error": err.Error()})
}

// Connection-specific headers describe one transport hop, not the proxied message.
func removeHopHeaders(headers http.Header) {
	for _, value := range headers.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			headers.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		headers.Del(name)
	}
}

// NewReverseProxy creates a standard httputil.ReverseProxy for the given target
func NewReverseProxy(target string) (*httputil.ReverseProxy, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(targetURL), nil
}
