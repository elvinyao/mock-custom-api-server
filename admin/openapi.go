package admin

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

var (
	rePathParam = regexp.MustCompile(`:([^/]+)`)
	reCatchAll  = regexp.MustCompile(`\*([^/]*)`)
)

// convertToOpenAPIPath converts Gin-style path params to OpenAPI format.
// :id  →  {id}
// *path → {path}
// *    → {path}
func convertToOpenAPIPath(ginPath string) string {
	path := rePathParam.ReplaceAllString(ginPath, `{$1}`)
	path = reCatchAll.ReplaceAllStringFunc(path, func(s string) string {
		m := reCatchAll.FindStringSubmatch(s)
		name := m[1]
		if name == "" {
			name = "path"
		}
		return "{" + name + "}"
	})
	return path
}

// extractPathParams returns OpenAPI parameter objects for path params.
func extractPathParams(ginPath string) []map[string]interface{} {
	var params []map[string]interface{}
	for _, m := range rePathParam.FindAllStringSubmatch(ginPath, -1) {
		params = append(params, map[string]interface{}{
			"name":     m[1],
			"in":       "path",
			"required": true,
			"schema":   map[string]interface{}{"type": "string"},
		})
	}
	for _, m := range reCatchAll.FindAllStringSubmatch(ginPath, -1) {
		name := m[1]
		if name == "" {
			name = "path"
		}
		params = append(params, map[string]interface{}{
			"name":     name,
			"in":       "path",
			"required": true,
			"schema":   map[string]interface{}{"type": "string"},
		})
	}
	return params
}

// buildResponses collects unique status codes from the default response + all rules.
func buildResponses(ep config.Endpoint) map[string]interface{} {
	codes := map[int]bool{}
	if ep.Default.StatusCode > 0 {
		codes[ep.Default.StatusCode] = true
	}
	for _, rule := range ep.Rules {
		if rule.StatusCode > 0 {
			codes[rule.StatusCode] = true
		}
	}
	if len(codes) == 0 {
		codes[200] = true
	}

	responses := map[string]interface{}{}
	for code := range codes {
		text := http.StatusText(code)
		if text == "" {
			text = "Response"
		}
		responses[strconv.Itoa(code)] = map[string]interface{}{
			"description": text,
		}
	}
	return responses
}

// cloneOperation makes a shallow copy of an operation map.
func cloneOperation(op map[string]interface{}) map[string]interface{} {
	cp := make(map[string]interface{}, len(op))
	for k, v := range op {
		cp[k] = v
	}
	return cp
}

// getOpenAPISpec generates and returns an OpenAPI 3.0 spec for all registered endpoints.
func (h *Handler) getOpenAPISpec(c *gin.Context) {
	allEps := h.configManager.GetAllEndpoints()

	paths := map[string]interface{}{}

	for _, ep := range allEps {
		oaPath := convertToOpenAPIPath(ep.Path)

		// Path parameters
		params := make([]interface{}, 0)
		for _, p := range extractPathParams(ep.Path) {
			params = append(params, p)
		}

		// Selector-based parameters (header / query only)
		for _, sel := range ep.Selectors {
			switch sel.Type {
			case "header", "query":
				params = append(params, map[string]interface{}{
					"name":        sel.Key,
					"in":          sel.Type,
					"required":    false,
					"schema":      map[string]interface{}{"type": "string"},
					"description": "selector: " + sel.Name,
				})
			}
		}

		baseOp := map[string]interface{}{
			"responses": buildResponses(ep),
		}
		if ep.Description != "" {
			baseOp["summary"] = ep.Description
		}
		if len(params) > 0 {
			baseOp["parameters"] = params
		}

		// Expand ANY to the common HTTP methods
		methods := []string{strings.ToLower(ep.Method)}
		if strings.ToUpper(ep.Method) == "ANY" {
			methods = []string{"get", "post", "put", "delete", "patch"}
		}

		if _, ok := paths[oaPath]; !ok {
			paths[oaPath] = map[string]interface{}{}
		}
		pathItem := paths[oaPath].(map[string]interface{})

		for _, m := range methods {
			op := cloneOperation(baseOp)
			if m == "post" || m == "put" || m == "patch" {
				op["requestBody"] = map[string]interface{}{
					"required": false,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{"type": "object"},
						},
					},
				}
			}
			pathItem[m] = op
		}
	}

	// Server URL: same host/port as the mock server itself
	cfg := h.configManager.GetConfig()
	port := 8080
	if cfg != nil && cfg.Port > 0 {
		port = cfg.Port
	}
	serverURL := fmt.Sprintf("http://localhost:%d", port)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Mock API Server",
			"description": "Auto-generated spec from registered mock endpoints",
			"version":     "1.0.0",
		},
		"servers": []interface{}{
			map[string]interface{}{"url": serverURL},
		},
		"paths": paths,
	}

	c.JSON(http.StatusOK, spec)
}
