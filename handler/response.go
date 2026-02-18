package handler

import (
	"math/rand"
	"os"
	"time"

	"mock-api-server/pkg/template"
)

// ResponseBuilder builds HTTP responses
type ResponseBuilder struct{}

// NewResponseBuilder creates a new ResponseBuilder
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{}
}

// ResponseResult contains the built response data
type ResponseResult struct {
	Body        []byte
	StatusCode  int
	Headers     map[string]string
	DelayMs     int
	ContentType string
}

// RandomResponseConfig represents a random response configuration
type RandomResponseConfig struct {
	File       string
	Weight     int
	StatusCode int
	DelayMs    int
}

// ResponseBuildConfig contains all config needed to build a response
type ResponseBuildConfig struct {
	ResponseFile    string
	StatusCode      int
	DelayMs         int
	Headers         map[string]string
	ContentType     string
	TemplateEnabled bool
	TemplateEngine  string // "simple" | "go"
	RandomResponses []RandomResponseConfig
}

// Build builds a response based on configuration and extracted values
func (rb *ResponseBuilder) Build(cfg ResponseBuildConfig, values map[string]string) (*ResponseResult, error) {
	result := &ResponseResult{
		Headers: make(map[string]string),
	}

	// Handle random responses
	if len(cfg.RandomResponses) > 0 {
		rr := selectRandomResponse(cfg.RandomResponses)
		cfg.ResponseFile = rr.File
		cfg.StatusCode = rr.StatusCode
		cfg.DelayMs = rr.DelayMs
	}

	// Read response file
	if cfg.ResponseFile != "" {
		content, err := os.ReadFile(cfg.ResponseFile)
		if err != nil {
			return nil, err
		}
		result.Body = content
	}

	// Apply template substitution
	if cfg.TemplateEnabled && len(result.Body) > 0 {
		engine := cfg.TemplateEngine
		if engine == "" {
			engine = "simple"
		}
		result.Body = template.ReplaceVariablesWithEngine(result.Body, values, engine)
	}

	// Set status code
	result.StatusCode = cfg.StatusCode
	if result.StatusCode == 0 {
		result.StatusCode = 200
	}

	// Set delay
	result.DelayMs = cfg.DelayMs

	// Determine content type
	contentType := cfg.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	result.ContentType = contentType

	// Set Content-Type header first (can be overridden by explicit headers)
	result.Headers["Content-Type"] = contentType

	// Merge additional headers
	for k, v := range cfg.Headers {
		// Apply template to header values too
		if cfg.TemplateEnabled {
			engine := cfg.TemplateEngine
			if engine == "" {
				engine = "simple"
			}
			v = string(template.ReplaceVariablesWithEngine([]byte(v), values, engine))
		}
		result.Headers[k] = v
	}

	return result, nil
}

// selectRandomResponse selects a random response based on weights
func selectRandomResponse(responses []RandomResponseConfig) RandomResponseConfig {
	if len(responses) == 0 {
		return RandomResponseConfig{}
	}

	// Calculate total weight
	totalWeight := 0
	for _, r := range responses {
		totalWeight += r.Weight
	}

	if totalWeight == 0 {
		// If no weights, select randomly with equal probability
		return responses[rand.Intn(len(responses))]
	}

	// Select based on weight
	r := rand.Intn(totalWeight)
	cumulative := 0
	for _, resp := range responses {
		cumulative += resp.Weight
		if r < cumulative {
			return resp
		}
	}

	// Fallback to first
	return responses[0]
}

// ApplyDelay applies the configured delay
func ApplyDelay(delayMs int) {
	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
}
