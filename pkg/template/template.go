package template

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReplaceVariables replaces template variables in content
// Supports:
// - {{.selector_name}} - values from selectors
// - {{.timestamp}} - current RFC3339 timestamp
// - {{.uuid}} - random UUID
// - {{.request_id}} - random request ID (shorter UUID)
func ReplaceVariables(content []byte, values map[string]string) []byte {
	result := string(content)

	// Replace built-in variables
	builtins := getBuiltinVariables()

	// Replace {{.timestamp}}
	result = strings.ReplaceAll(result, "{{.timestamp}}", builtins["timestamp"])

	// Replace {{.uuid}}
	result = strings.ReplaceAll(result, "{{.uuid}}", builtins["uuid"])

	// Replace {{.request_id}}
	result = strings.ReplaceAll(result, "{{.request_id}}", builtins["request_id"])

	// Replace selector values
	for name, value := range values {
		placeholder := "{{." + name + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// Clean up any remaining unmatched placeholders (optional behavior)
	// result = cleanUnmatchedPlaceholders(result)

	return []byte(result)
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
