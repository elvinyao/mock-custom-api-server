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
	"mock-api-server/proxy"
	"mock-api-server/state"

	"github.com/gin-gonic/gin"
)

// MockHandler handles mock API requests
type MockHandler struct {
	configManager   *config.ConfigManager
	responseBuilder *ResponseBuilder
	stateStore      *state.ScenarioStore
}

// NewMockHandler creates a new MockHandler
func NewMockHandler(cfgManager *config.ConfigManager) *MockHandler {
	return &MockHandler{
		configManager:   cfgManager,
		responseBuilder: NewResponseBuilder(),
		stateStore:      state.New(),
	}
}

// NewMockHandlerWithState creates a MockHandler with an existing state store
func NewMockHandlerWithState(cfgManager *config.ConfigManager, stateStore *state.ScenarioStore) *MockHandler {
	return &MockHandler{
		configManager:   cfgManager,
		responseBuilder: NewResponseBuilder(),
		stateStore:      stateStore,
	}
}

// GetStateStore returns the state store (for use by admin handlers)
func (h *MockHandler) GetStateStore() *state.ScenarioStore {
	return h.stateStore
}

// RegisterRoutes registers all endpoint routes from config
func (h *MockHandler) RegisterRoutes(r *gin.Engine) {
	endpoints := h.configManager.GetAllEndpoints()
	h.registerEndpoints(r, endpoints)

	// Set NoRoute handler for 404
	r.NoRoute(func(c *gin.Context) {
		h.handleNotFound(c, h.configManager.GetConfig())
	})
}

func (h *MockHandler) registerEndpoints(r *gin.Engine, endpoints []config.Endpoint) {
	for _, ep := range endpoints {
		path := ep.Path
		method := strings.ToUpper(ep.Method)

		switch method {
		case "GET":
			r.GET(path, h.handleRequest)
		case "POST":
			r.POST(path, h.handleRequest)
		case "PUT":
			r.PUT(path, h.handleRequest)
		case "DELETE":
			r.DELETE(path, h.handleRequest)
		case "PATCH":
			r.PATCH(path, h.handleRequest)
		case "OPTIONS":
			r.OPTIONS(path, h.handleRequest)
		case "HEAD":
			r.HEAD(path, h.handleRequest)
		case "ANY":
			r.Any(path, h.handleRequest)
		default:
			r.Handle(method, path, h.handleRequest)
		}
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

	// Find matching endpoint (from all sources: file + runtime)
	allEndpoints := h.configManager.GetAllEndpoints()
	endpoint, pathParams := h.findEndpoint(allEndpoints, path, method)
	if endpoint == nil {
		h.handleNotFound(c, cfg)
		return
	}

	// Store path params in context
	for k, v := range pathParams {
		c.Params = append(c.Params, gin.Param{Key: k, Value: v})
	}

	// Proxy mode: forward to upstream, fall back to mock rules only if configured
	if strings.EqualFold(endpoint.Mode, "proxy") {
		proxyHandler := proxy.New()
		if proxyHandler.ProxyRequest(c, *endpoint) {
			return // handled by proxy (success or non-fallback error)
		}
		// fallback_on_error=true and upstream failed: continue to mock rules below
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
	rules := convertRules(endpoint.Rules)

	// Match rules, considering scenario state if applicable
	var matchedRule *Rule
	if endpoint.Scenario != "" {
		// Get current scenario step using partition key
		partitionValue := ""
		if endpoint.ScenarioKey != "" {
			partitionValue = values[endpoint.ScenarioKey]
		}
		currentStep := h.stateStore.GetStep(endpoint.Scenario, partitionValue)

		// Match rule filtered by scenario step
		matchedRule = MatchRulesForStep(values, rules, currentStep)

		// Transition to next step if rule matched
		if matchedRule != nil && matchedRule.NextStep != "" {
			h.stateStore.SetStep(endpoint.Scenario, partitionValue, matchedRule.NextStep)
		}
	} else {
		matchedRule = MatchRules(values, rules)
	}

	// Build response config
	var respCfg ResponseBuildConfig
	var matchedRuleName string

	if matchedRule != nil {
		matchedRuleName = fmt.Sprintf("rule_%d", getRuleIndex(rules, matchedRule))
		ruleTemplateEnabled := matchedRule.Template != nil && matchedRule.Template.Enabled
		ruleTemplateEngine := "simple"
		if ruleTemplateEnabled && matchedRule.Template.Engine != "" {
			ruleTemplateEngine = matchedRule.Template.Engine
		}
		respCfg = ResponseBuildConfig{
			ResponseFile:    matchedRule.ResponseFile,
			StatusCode:      matchedRule.StatusCode,
			DelayMs:         matchedRule.DelayMs,
			Headers:         matchedRule.Headers,
			ContentType:     matchedRule.ContentType,
			TemplateEnabled: ruleTemplateEnabled,
			TemplateEngine:  ruleTemplateEngine,
		}
	} else {
		matchedRuleName = "default"
		templateEnabled := endpoint.Default.Template != nil && endpoint.Default.Template.Enabled
		templateEngine := "simple"
		if endpoint.Default.Template != nil && endpoint.Default.Template.Engine != "" {
			templateEngine = endpoint.Default.Template.Engine
		}
		respCfg = ResponseBuildConfig{
			ResponseFile:    endpoint.Default.ResponseFile,
			StatusCode:      endpoint.Default.StatusCode,
			DelayMs:         endpoint.Default.DelayMs,
			Headers:         endpoint.Default.Headers,
			ContentType:     endpoint.Default.ContentType,
			TemplateEnabled: templateEnabled,
			TemplateEngine:  templateEngine,
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

	// Store matched rule name in context for logging/recording
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
	contentType := result.ContentType
	if ct, ok := result.Headers["Content-Type"]; ok {
		contentType = ct
	}
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(result.StatusCode, contentType, result.Body)
}

// convertRules converts config rules to handler rules
func convertRules(cfgRules []config.Rule) []Rule {
	rules := make([]Rule, len(cfgRules))
	for i, r := range cfgRules {
		conditions := make([]Condition, len(r.Conditions))
		for j, cond := range r.Conditions {
			conditions[j] = Condition{
				Selector:  cond.Selector,
				MatchType: cond.MatchType,
				Value:     cond.Value,
			}
		}
		groups := make([]ConditionGroup, len(r.ConditionGroups))
		for j, g := range r.ConditionGroups {
			gConds := make([]Condition, len(g.Conditions))
			for k, gc := range g.Conditions {
				gConds[k] = Condition{
					Selector:  gc.Selector,
					MatchType: gc.MatchType,
					Value:     gc.Value,
				}
			}
			groups[j] = ConditionGroup{
				Logic:      g.Logic,
				Conditions: gConds,
			}
		}
		var tmplCfg *TemplateConfig
		if r.Template != nil {
			tmplCfg = &TemplateConfig{
				Enabled: r.Template.Enabled,
				Engine:  r.Template.Engine,
			}
		}
		rules[i] = Rule{
			ConditionLogic:  r.ConditionLogic,
			Conditions:      conditions,
			ConditionGroups: groups,
			ScenarioStep:    r.ScenarioStep,
			NextStep:        r.NextStep,
			ResponseFile:    r.ResponseFile,
			StatusCode:      r.StatusCode,
			DelayMs:         r.DelayMs,
			Headers:         r.Headers,
			ContentType:     r.ContentType,
			Template:        tmplCfg,
		}
	}
	return rules
}

// findEndpoint finds a matching endpoint for the given path and method
func (h *MockHandler) findEndpoint(endpoints []config.Endpoint, requestPath, method string) (*config.Endpoint, map[string]string) {
	for i := range endpoints {
		ep := &endpoints[i]

		// Check method
		if ep.Method != "ANY" && !strings.EqualFold(ep.Method, method) {
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

// matchPath matches a request path against an endpoint path pattern.
// Supports :param segments and catch-all wildcards at the end:
//   /*name  (Gin-style named catch-all, e.g. /*path)
//   /*      (unnamed, also accepted)
func matchPath(pattern, requestPath string) (map[string]string, bool) {
	// Detect catch-all wildcard: /*name or /*
	if idx := strings.Index(pattern, "/*"); idx != -1 {
		prefix := pattern[:idx]
		// match if requestPath starts with the prefix (with or without trailing slash)
		if strings.HasPrefix(requestPath, prefix) {
			return map[string]string{}, true
		}
		return nil, false
	}

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
	if cfg != nil {
		// Check for custom 404 response
		if file, ok := cfg.Server.ErrorHandling.CustomErrorResponses[404]; ok {
			content, err := os.ReadFile(file)
			if err == nil {
				c.Data(http.StatusNotFound, "application/json", content)
				return
			}
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
	if cfg != nil {
		// Check for custom 500 response
		if file, ok := cfg.Server.ErrorHandling.CustomErrorResponses[500]; ok {
			content, readErr := os.ReadFile(file)
			if readErr == nil {
				c.Data(http.StatusInternalServerError, "application/json", content)
				return
			}
		}
	}

	response := gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "An internal error occurred",
		},
	}

	if cfg != nil && cfg.Server.ErrorHandling.ShowDetails {
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
	re := regexp.MustCompile(`:(\\w+)`)
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
