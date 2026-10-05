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

// RegisterRoutes dispatches mock routes against the live configuration. Keeping
// them outside Gin's static tree makes reloads and runtime additions equivalent,
// and leaves registered admin, UI, and health handlers in charge of their paths.
func (h *MockHandler) RegisterRoutes(r *gin.Engine) {
	r.NoRoute(h.handleRequest)
}

// handleRequest handles incoming requests and matches against config endpoints
func (h *MockHandler) handleRequest(c *gin.Context) {
	trace := MatchTrace{Selectors: map[string]string{}, Rules: []RuleTrace{}, SelectedRule: "not_found", ResponseSource: "not_found"}
	defer func() { c.Set("match_trace", trace) }()
	cfg := h.configManager.GetConfig()
	if cfg == nil {
		trace.SelectedRule, trace.ResponseSource = "error", "error"
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
	mode := strings.ToLower(endpoint.Mode)
	if mode == "" {
		mode = "mock"
	}
	trace.Endpoint = &TraceEndpoint{Path: endpoint.Path, Method: endpoint.Method, Mode: mode}

	// Store path params in context
	for k, v := range pathParams {
		c.Params = append(c.Params, gin.Param{Key: k, Value: v})
	}

	// Read body for potential reuse
	var bodyBytes []byte
	if c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		_ = c.Request.Body.Close()
		if err != nil {
			trace.SelectedRule, trace.ResponseSource = "error", "error"
			c.Set("matched_rule", "error")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to read request body: %v", err)})
			return
		}
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
	trace.Selectors = values
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	proxyFallback := false
	if mode == "proxy" {
		trace.SelectedRule = "proxy:" + endpoint.Proxy.Target
		trace.ResponseSource = trace.SelectedRule
		c.Set("matched_rule", trace.SelectedRule)
		c.Set("response_file", "")
		if proxy.New().ProxyRequest(c, *endpoint) {
			if c.GetString("proxy_error") != "" {
				trace.ResponseSource = "proxy_error:" + endpoint.Proxy.Target
			}
			return
		}
		proxyFallback = true
	}

	// Convert config rules to handler rules
	rules := convertRules(endpoint.Rules)

	// Match rules
	matchedRule, ruleTraces := MatchRulesWithTrace(values, rules)
	trace.Rules = ruleTraces

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
			InlineBody:      matchedRule.InlineBody,
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
			InlineBody:      endpoint.Default.InlineBody,
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
	trace.SelectedRule = matchedRuleName

	// Build response
	result, err := h.responseBuilder.Build(respCfg, values)
	if err != nil {
		trace.ResponseSource = "error"
		h.handleError(c, cfg, err)
		return
	}
	c.Set("response_file", result.ResponseFile)
	trace.ResponseSource = "inline"
	if result.ResponseFile != "" {
		trace.ResponseSource = "file:" + result.ResponseFile
		if len(respCfg.RandomResponses) > 0 {
			trace.ResponseSource = "random_file:" + result.ResponseFile
		}
	}
	if proxyFallback {
		trace.ResponseSource = "proxy_fallback:" + trace.ResponseSource
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
			ResponseFile:    r.ResponseFile,
			InlineBody:      r.InlineBody,
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
	var selected *config.Endpoint
	var params map[string]string
	for i := range endpoints {
		ep := &endpoints[i]

		// Check method
		if !strings.EqualFold(ep.Method, "ANY") && !strings.EqualFold(ep.Method, method) {
			continue
		}

		// Check path (with parameter support)
		pathParams, matched := matchPath(ep.Path, requestPath)
		if matched && (selected == nil || moreSpecificPath(ep.Path, selected.Path)) {
			selected, params = ep, pathParams
		}
	}
	return selected, params
}

// matchPath matches a request path against an endpoint path pattern.
// Supports :param segments and catch-all wildcards at the end:
//
//	/*name  (Gin-style named catch-all, e.g. /*path)
//	/*      (unnamed, also accepted)
func matchPath(pattern, requestPath string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	requestParts := strings.Split(strings.TrimPrefix(requestPath, "/"), "/")
	params := make(map[string]string)
	for i, patternPart := range patternParts {
		if strings.HasPrefix(patternPart, "*") && i == len(patternParts)-1 {
			if name := strings.TrimPrefix(patternPart, "*"); name != "" {
				params[name] = ""
				if i < len(requestParts) {
					params[name] = "/" + strings.Join(requestParts[i:], "/")
				}
			}
			return params, true
		}
		if i >= len(requestParts) {
			return nil, false
		}
		requestPart := requestParts[i]
		if strings.HasPrefix(patternPart, ":") {
			if requestPart == "" {
				return nil, false
			}
			paramName := patternPart[1:]
			params[paramName] = requestPart
		} else if patternPart != requestPart {
			return nil, false
		}
	}
	for _, remaining := range requestParts[len(patternParts):] {
		if remaining != "" {
			return nil, false
		}
	}
	return params, true
}

// moreSpecificPath compares left to right: static segments precede parameters,
// which precede catch-alls. Equal patterns keep their declaration order.
func moreSpecificPath(left, right string) bool {
	l := strings.Split(strings.Trim(left, "/"), "/")
	r := strings.Split(strings.Trim(right, "/"), "/")
	for i := 0; i < len(l) || i < len(r); i++ {
		lr, rr := routeSegmentRank(l, i), routeSegmentRank(r, i)
		if lr != rr {
			return lr > rr
		}
	}
	return false
}

func routeSegmentRank(parts []string, index int) int {
	if index >= len(parts) {
		return 4
	}
	if strings.HasPrefix(parts[index], "*") {
		return 1
	}
	if strings.HasPrefix(parts[index], ":") {
		return 2
	}
	return 3
}

// handleNotFound handles 404 responses
func (h *MockHandler) handleNotFound(c *gin.Context, cfg *config.Config) {
	if cfg != nil {
		// Check for custom 404 response
		if file, ok := cfg.ErrorHandling.CustomErrorResponses[404]; ok {
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
		if file, ok := cfg.ErrorHandling.CustomErrorResponses[500]; ok {
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

	if cfg != nil && cfg.ErrorHandling.ShowDetails {
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
				"hot_reload":      cfg != nil && cfg.HotReload,
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
