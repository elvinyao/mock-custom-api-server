package template

import (
	"strings"
	"testing"
)

func TestReplaceVariables(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		values   map[string]string
		contains []string
	}{
		{
			name:     "replace selector value",
			content:  `{"order_id": "{{.order_id}}"}`,
			values:   map[string]string{"order_id": "1001"},
			contains: []string{`"order_id": "1001"`},
		},
		{
			name:     "replace multiple values",
			content:  `{"order_id": "{{.order_id}}", "user_id": "{{.user_id}}"}`,
			values:   map[string]string{"order_id": "1001", "user_id": "U001"},
			contains: []string{`"order_id": "1001"`, `"user_id": "U001"`},
		},
		{
			name:     "replace timestamp",
			content:  `{"created_at": "{{.timestamp}}"}`,
			values:   map[string]string{},
			contains: []string{`"created_at": "`}, // timestamp should be replaced
		},
		{
			name:     "replace uuid",
			content:  `{"tracking_id": "{{.uuid}}"}`,
			values:   map[string]string{},
			contains: []string{`"tracking_id": "`}, // uuid should be replaced
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReplaceVariables([]byte(tt.content), tt.values)
			resultStr := string(result)

			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("ReplaceVariables() = %s, want to contain %s", resultStr, expected)
				}
			}

			// Check that placeholders are replaced
			if strings.Contains(resultStr, "{{.") && !strings.Contains(tt.content, "unmatched") {
				t.Errorf("ReplaceVariables() = %s, still contains unreplaced placeholders", resultStr)
			}
		})
	}
}

func TestExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "single placeholder",
			content:  `{"order_id": "{{.order_id}}"}`,
			expected: []string{"order_id"},
		},
		{
			name:     "multiple placeholders",
			content:  `{"order_id": "{{.order_id}}", "user_id": "{{.user_id}}"}`,
			expected: []string{"order_id", "user_id"},
		},
		{
			name:     "duplicate placeholders",
			content:  `{"a": "{{.id}}", "b": "{{.id}}"}`,
			expected: []string{"id"},
		},
		{
			name:     "no placeholders",
			content:  `{"order_id": "1001"}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPlaceholders([]byte(tt.content))

			if len(result) != len(tt.expected) {
				t.Errorf("ExtractPlaceholders() = %v, want %v", result, tt.expected)
				return
			}

			for i, name := range tt.expected {
				if result[i] != name {
					t.Errorf("ExtractPlaceholders()[%d] = %s, want %s", i, result[i], name)
				}
			}
		})
	}
}
