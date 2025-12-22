package handler

import (
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ExtractValues extracts values from request based on selectors
func ExtractValues(c *gin.Context, selectors []Selector, pathParams map[string]string) map[string]string {
	values := make(map[string]string)

	// Read body once
	var bodyBytes []byte
	var bodyRead bool

	for _, sel := range selectors {
		var value string

		switch strings.ToLower(sel.Type) {
		case "body":
			// Read body if not already read
			if !bodyRead {
				bodyBytes, _ = io.ReadAll(c.Request.Body)
				bodyRead = true
			}
			// Use gjson to extract value
			result := gjson.GetBytes(bodyBytes, sel.Key)
			value = result.String()

		case "header":
			value = c.GetHeader(sel.Key)

		case "query":
			value = c.Query(sel.Key)

		case "path":
			// Get from path parameters
			if pathParams != nil {
				value = pathParams[sel.Key]
			}
			// Also try Gin's param method
			if value == "" {
				value = c.Param(sel.Key)
			}
		}

		values[sel.Name] = value
	}

	return values
}

// Selector represents a selector configuration
type Selector struct {
	Name string
	Type string
	Key  string
}

// ConvertSelectors converts config selectors to handler selectors
func ConvertSelectors(cfgSelectors []struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Key  string `yaml:"key"`
}) []Selector {
	selectors := make([]Selector, len(cfgSelectors))
	for i, s := range cfgSelectors {
		selectors[i] = Selector{
			Name: s.Name,
			Type: s.Type,
			Key:  s.Key,
		}
	}
	return selectors
}
