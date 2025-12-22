package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

// MockHandler handles mock API requests
type MockHandler struct {
	configManager   *config.ConfigManager
	responseBuilder *ResponseBuilder
}

// NewMockHandler creates a new MockHandler
func NewMockHandler(cfgManager *config.ConfigManager) *MockHandler {
	return &MockHandler{
		configManager:   cfgManager,
		responseBuilder: NewResponseBuilder(),
	}
}

// RegisterRoutes registers all endpoint routes from config
func (h *MockHandler) RegisterRoutes(r *gin.Engine) {
	// Use a catch-all handler that dynamically matches based on config
	r.NoRoute(h.handleRequest)

	// Also register specific methods to catch them before NoRoute
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	for _, method := range methods {
		r.Handle(method, "/*path", h.handleRequest)
	}
}

// handleRequest handles incoming requests and matches against config endpoints
func (h *MockHandler) handleRequest(c *gin.Context) {
	cfg := h.configManager.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration not loaded"})
		return
	}

	path := c.Request.URL.Path
	method := c.Request.Method

	// Find matching endpoint
	endpoint, pathParams := h.findEndpoint(cfg.Endpoints, path, method)
	if endpoint == nil {
		h.handleNotFound(c, cfg)
		return
	}

	// Store path params in context
	for k, v := range pathParams {
		c.Params = append(c.Params, gin.Param{Key: k, Value: v})
	}

	// Read body for potential reuse
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		bodyBytes = []byte{}
	}
	// Restore body for selectors
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Convert config selectors to handler selectors
	selectors := make([]Selector, len(endpoint.Selectors))
	for i, s := range endpoint.Selectors {
		selectors[i] = Selector{
			Name: s.Name,
			Type: s.Type,
			Key:  s.Key,
		}
	}

	// Extract values from request
	values := ExtractValues(c, selectors, pathParams)

	// Convert config rules to handler rules
	rules := make([]Rule, len(endpoint.Rules))
	for i, r := range endpoint.Rules {
		conditions := make([]Condition, len(r.Conditions))
		for j, cond := range r.Conditions {
			conditions[j] = Condition{
				Selector:  cond.Selector,
				MatchType: cond.MatchType,
				Value:     cond.Value,
			}
		}
		rules[i] = Rule{
			Conditions:   conditions,
			ResponseFile: r.ResponseFile,
			StatusCode:   r.StatusCode,
			DelayMs:      r.DelayMs,
			Headers:      r.Headers,
		}
	}

	// Match rules
	matchedRule := MatchRules(values, rules)

	// Build response config
	var respCfg ResponseBuildConfig
	var matchedRuleName string

	if matchedRule != nil {
		matchedRuleName = fmt.Sprintf("rule_%d", getRuleIndex(rules, matchedRule))
		respCfg = ResponseBuildConfig{
			ResponseFile:    matchedRule.ResponseFile,
			StatusCode:      matchedRule.StatusCode,
			DelayMs:         matchedRule.DelayMs,
			Headers:         matchedRule.Headers,
			TemplateEnabled: false, // Rules don't have template config currently
		}
	} else {
		matchedRuleName = "default"
		respCfg = ResponseBuildConfig{
			ResponseFile:    endpoint.Default.ResponseFile,
			StatusCode:      endpoint.Default.StatusCode,
			DelayMs:         endpoint.Default.DelayMs,
			Headers:         endpoint.Default.Headers,
			TemplateEnabled: endpoint.Default.Template != nil && endpoint.Default.Template.Enabled,
		}

		// Handle random responses
		if endpoint.Default.RandomResponses != nil && endpoint.Default.RandomResponses.Enabled {
			randomConfigs := make([]RandomResponseConfig, len(endpoint.Default.RandomResponses.Files))
			for i, rr := range endpoint.Default.RandomResponses.Files {
				randomConfigs[i] = RandomResponseConfig{
					File:       rr.File,
					Weight:     rr.Weight,
					StatusCode: rr.StatusCode,
					DelayMs:    rr.DelayMs,
				}
			}
			respCfg.RandomResponses = randomConfigs
		}
	}

	// Store matched rule name in context for logging
	c.Set("matched_rule", matchedRuleName)
	c.Set("response_file", respCfg.ResponseFile)

	// Build response
	result, err := h.responseBuilder.Build(respCfg, values)
	if err != nil {
		h.handleError(c, cfg, err)
		return
	}

	// Apply delay
	ApplyDelay(result.DelayMs)

	// Set headers
	for k, v := range result.Headers {
		c.Header(k, v)
	}

	// Send response
	c.Data(result.StatusCode, result.Headers["Content-Type"], result.Body)
}

// findEndpoint finds a matching endpoint for the given path and method
func (h *MockHandler) findEndpoint(endpoints []config.Endpoint, requestPath, method string) (*config.Endpoint, map[string]string) {
	for i := range endpoints {
		ep := &endpoints[i]

		// Check method
		if !strings.EqualFold(ep.Method, method) {
			continue
		}

		// Check path (with parameter support)
		pathParams, matched := matchPath(ep.Path, requestPath)
		if matched {
			return ep, pathParams
		}
	}
	return nil, nil
}

// matchPath matches a request path against an endpoint path pattern
// Supports path parameters like :id or :user_id
func matchPath(pattern, requestPath string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	requestParts := strings.Split(strings.Trim(requestPath, "/"), "/")

	if len(patternParts) != len(requestParts) {
		return nil, false
	}

	params := make(map[string]string)

	for i, patternPart := range patternParts {
		requestPart := requestParts[i]

		if strings.HasPrefix(patternPart, ":") {
			// This is a path parameter
			paramName := patternPart[1:]
			params[paramName] = requestPart
		} else if patternPart != requestPart {
			// Static part doesn't match
			return nil, false
		}
	}

	return params, true
}

// handleNotFound handles 404 responses
func (h *MockHandler) handleNotFound(c *gin.Context, cfg *config.Config) {
	// Check for custom 404 response
	if file, ok := cfg.Server.ErrorHandling.CustomErrorResponses[404]; ok {
		content, err := os.ReadFile(file)
		if err == nil {
			c.Data(http.StatusNotFound, "application/json", content)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"error": gin.H{
			"code":    "NOT_FOUND",
			"message": "The requested resource was not found",
			"path":    c.Request.URL.Path,
		},
	})
}

// handleError handles internal errors
func (h *MockHandler) handleError(c *gin.Context, cfg *config.Config, err error) {
	// Check for custom 500 response
	if file, ok := cfg.Server.ErrorHandling.CustomErrorResponses[500]; ok {
		content, readErr := os.ReadFile(file)
		if readErr == nil {
			c.Data(http.StatusInternalServerError, "application/json", content)
			return
		}
	}

	response := gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "An internal error occurred",
		},
	}

	if cfg.Server.ErrorHandling.ShowDetails {
		response["error"].(gin.H)["details"] = err.Error()
	}

	c.JSON(http.StatusInternalServerError, response)
}

// getRuleIndex returns the index of a rule in the rules slice
func getRuleIndex(rules []Rule, target *Rule) int {
	for i := range rules {
		if &rules[i] == target {
			return i
		}
	}
	return -1
}

// HealthHandler returns the health check handler
func HealthHandler(cfgManager *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := cfgManager.GetConfig()
		endpointsCount := 0
		if cfg != nil {
			endpointsCount = len(cfg.Endpoints)
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": cfgManager.GetLoadedAt().Format("2006-01-02T15:04:05Z07:00"),
			"config": gin.H{
				"loaded_at":       cfgManager.GetLoadedAt().Format("2006-01-02T15:04:05Z07:00"),
				"endpoints_count": endpointsCount,
				"hot_reload":      cfg != nil && cfg.Server.HotReload,
			},
		})
	}
}

// extractPathParams extracts path parameters from a pattern and value
func extractPathParams(pattern, value string) map[string]string {
	params := make(map[string]string)

	// Convert pattern to regex
	regexPattern := regexp.QuoteMeta(pattern)
	paramNames := []string{}

	// Find all :param patterns
	re := regexp.MustCompile(`:(\w+)`)
	matches := re.FindAllStringSubmatch(pattern, -1)
	for _, match := range matches {
		if len(match) > 1 {
			paramNames = append(paramNames, match[1])
		}
	}

	// Replace :param with capture groups
	regexPattern = re.ReplaceAllString(regexPattern, `([^/]+)`)
	regexPattern = "^" + regexPattern + "$"

	compiled, err := regexp.Compile(regexPattern)
	if err != nil {
		return params
	}

	valueMatches := compiled.FindStringSubmatch(value)
	if len(valueMatches) > 1 {
		for i, name := range paramNames {
			if i+1 < len(valueMatches) {
				params[name] = valueMatches[i+1]
			}
		}
	}

	return params
}
