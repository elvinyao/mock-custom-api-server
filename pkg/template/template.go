package template

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	gotemplate "text/template"
	"time"

	"github.com/google/uuid"
)

// ReplaceVariables replaces template variables in content using the simple engine.
// Supports:
// - {{.selector_name}} - values from selectors
// - {{.timestamp}} - current RFC3339 timestamp
// - {{.uuid}} - random UUID
// - {{.request_id}} - random request ID (shorter UUID)
func ReplaceVariables(content []byte, values map[string]string) []byte {
	return ReplaceVariablesWithEngine(content, values, "simple")
}

// ReplaceVariablesWithEngine replaces template variables using the specified engine.
// engine can be "simple" (default, backward-compatible) or "go" (uses text/template).
func ReplaceVariablesWithEngine(content []byte, values map[string]string, engine string) []byte {
	switch strings.ToLower(engine) {
	case "go":
		return replaceWithGoTemplate(content, values)
	default:
		return replaceSimple(content, values)
	}
}

// replaceSimple is the original simple string replacement engine
func replaceSimple(content []byte, values map[string]string) []byte {
	result := string(content)

	// Replace built-in variables
	builtins := getBuiltinVariables()

	result = strings.ReplaceAll(result, "{{.timestamp}}", builtins["timestamp"])
	result = strings.ReplaceAll(result, "{{.uuid}}", builtins["uuid"])
	result = strings.ReplaceAll(result, "{{.request_id}}", builtins["request_id"])

	// Replace selector values
	for name, value := range values {
		placeholder := "{{." + name + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return []byte(result)
}

// replaceWithGoTemplate uses Go's text/template engine for powerful templating
func replaceWithGoTemplate(content []byte, values map[string]string) []byte {
	builtins := getBuiltinVariables()

	// Build template data combining builtins and selector values
	data := make(map[string]interface{})
	for k, v := range builtins {
		data[k] = v
	}
	for k, v := range values {
		data[k] = v
	}

	funcMap := buildFuncMap()

	tmpl, err := gotemplate.New("response").Funcs(funcMap).Parse(string(content))
	if err != nil {
		// Fall back to simple engine on parse error
		return replaceSimple(content, values)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// Fall back to simple engine on execute error
		return replaceSimple(content, values)
	}

	return buf.Bytes()
}

// buildFuncMap returns the template function map for the go engine
func buildFuncMap() gotemplate.FuncMap {
	return gotemplate.FuncMap{
		"randomInt": func(min, max int) int {
			if max <= min {
				return min
			}
			return min + rand.Intn(max-min)
		},
		"randomFloat": func(min, max float64) float64 {
			if max <= min {
				return min
			}
			return min + rand.Float64()*(max-min)
		},
		"randomChoice": func(items ...string) string {
			if len(items) == 0 {
				return ""
			}
			return items[rand.Intn(len(items))]
		},
		"timestampMs": func() int64 {
			return time.Now().UnixMilli()
		},
		"timestamp": func() string {
			return time.Now().Format(time.RFC3339)
		},
		"uuid": func() string {
			return uuid.New().String()
		},
		"base64Encode": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"jsonEscape": func(s string) string {
			b, err := json.Marshal(s)
			if err != nil {
				return s
			}
			// Remove surrounding quotes
			return string(b[1 : len(b)-1])
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"env": func(key string) string {
			return os.Getenv(key)
		},
		"upper":   strings.ToUpper,
		"lower":   strings.ToLower,
		"trim":    strings.TrimSpace,
		"replace": strings.ReplaceAll,
		"sprintf": fmt.Sprintf,
	}
}

// getBuiltinVariables generates built-in variable values
func getBuiltinVariables() map[string]string {
	newUUID := uuid.New().String()
	return map[string]string{
		"timestamp":  time.Now().Format(time.RFC3339),
		"uuid":       newUUID,
		"request_id": strings.Split(newUUID, "-")[0], // First segment of UUID
	}
}

// cleanUnmatchedPlaceholders removes any remaining {{.xxx}} patterns
func cleanUnmatchedPlaceholders(content string) string {
	re := regexp.MustCompile(`\{\{\.[^}]+\}\}`)
	return re.ReplaceAllString(content, "")
}

// ExtractPlaceholders extracts all placeholder names from content
func ExtractPlaceholders(content []byte) []string {
	re := regexp.MustCompile(`\{\{\.([^}]+)\}\}`)
	matches := re.FindAllSubmatch(content, -1)

	names := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			name := string(match[1])
			if !seen[name] {
				names = append(names, name)
				seen[name] = true
			}
		}
	}

	return names
}
