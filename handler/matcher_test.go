package handler

import (
	"testing"
)

func TestMatchConditionExact(t *testing.T) {
	tests := []struct {
		name        string
		targetValue string
		cond        Condition
		expected    bool
	}{
		{"exact match", "1001", Condition{MatchType: "exact", Value: "1001"}, true},
		{"exact no match", "1002", Condition{MatchType: "exact", Value: "1001"}, false},
		{"exact empty", "", Condition{MatchType: "exact", Value: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchCondition(tt.targetValue, tt.cond)
			if result != tt.expected {
				t.Errorf("matchCondition(%q, %+v) = %v, want %v", tt.targetValue, tt.cond, result, tt.expected)
			}
		})
	}
}

func TestMatchConditionPrefix(t *testing.T) {
	tests := []struct {
		name        string
		targetValue string
		cond        Condition
		expected    bool
	}{
		{"prefix match", "VIP_1001", Condition{MatchType: "prefix", Value: "VIP_"}, true},
		{"prefix no match", "REG_1001", Condition{MatchType: "prefix", Value: "VIP_"}, false},
		{"prefix empty target", "", Condition{MatchType: "prefix", Value: "VIP_"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchCondition(tt.targetValue, tt.cond)
			if result != tt.expected {
				t.Errorf("matchCondition(%q, %+v) = %v, want %v", tt.targetValue, tt.cond, result, tt.expected)
			}
		})
	}
}

func TestMatchConditionSuffix(t *testing.T) {
	tests := []struct {
		name        string
		targetValue string
		cond        Condition
		expected    bool
	}{
		{"suffix match", "order_test", Condition{MatchType: "suffix", Value: "_test"}, true},
		{"suffix no match", "order_prod", Condition{MatchType: "suffix", Value: "_test"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchCondition(tt.targetValue, tt.cond)
			if result != tt.expected {
				t.Errorf("matchCondition(%q, %+v) = %v, want %v", tt.targetValue, tt.cond, result, tt.expected)
			}
		})
	}
}

func TestMatchConditionRegex(t *testing.T) {
	tests := []struct {
		name        string
		targetValue string
		cond        Condition
		expected    bool
	}{
		{"regex match", "ERR_1234", Condition{MatchType: "regex", Value: "^ERR_[0-9]{4}$"}, true},
		{"regex no match", "ERR_12345", Condition{MatchType: "regex", Value: "^ERR_[0-9]{4}$"}, false},
		{"regex no match 2", "ERR_ABCD", Condition{MatchType: "regex", Value: "^ERR_[0-9]{4}$"}, false},
		{"invalid regex", "test", Condition{MatchType: "regex", Value: "["}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchCondition(tt.targetValue, tt.cond)
			if result != tt.expected {
				t.Errorf("matchCondition(%q, %+v) = %v, want %v", tt.targetValue, tt.cond, result, tt.expected)
			}
		})
	}
}

func TestMatchConditionRange(t *testing.T) {
	tests := []struct {
		name        string
		targetValue string
		cond        Condition
		expected    bool
	}{
		{"range inclusive match", "50", Condition{MatchType: "range", Value: "[1, 100]"}, true},
		{"range inclusive match min", "1", Condition{MatchType: "range", Value: "[1, 100]"}, true},
		{"range inclusive match max", "100", Condition{MatchType: "range", Value: "[1, 100]"}, true},
		{"range inclusive no match low", "0", Condition{MatchType: "range", Value: "[1, 100]"}, false},
		{"range inclusive no match high", "101", Condition{MatchType: "range", Value: "[1, 100]"}, false},
		{"range exclusive", "50", Condition{MatchType: "range", Value: "(0, 100)"}, true},
		{"range exclusive no match min", "0", Condition{MatchType: "range", Value: "(0, 100)"}, false},
		{"range exclusive no match max", "100", Condition{MatchType: "range", Value: "(0, 100)"}, false},
		{"range non-numeric", "abc", Condition{MatchType: "range", Value: "[1, 100]"}, false},
		{"range invalid format", "50", Condition{MatchType: "range", Value: "invalid"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchCondition(tt.targetValue, tt.cond)
			if result != tt.expected {
				t.Errorf("matchCondition(%q, %+v) = %v, want %v", tt.targetValue, tt.cond, result, tt.expected)
			}
		})
	}
}

func TestMatchRules(t *testing.T) {
	rules := []Rule{
		{
			Conditions:   []Condition{{Selector: "order_id", MatchType: "exact", Value: "1001"}},
			ResponseFile: "success.json",
			StatusCode:   200,
		},
		{
			Conditions:   []Condition{{Selector: "order_id", MatchType: "prefix", Value: "VIP_"}},
			ResponseFile: "vip.json",
			StatusCode:   200,
		},
		{
			Conditions: []Condition{
				{Selector: "order_id", MatchType: "prefix", Value: "VIP_"},
				{Selector: "user_type", MatchType: "exact", Value: "premium"},
			},
			ResponseFile: "vip_premium.json",
			StatusCode:   200,
		},
	}

	tests := []struct {
		name     string
		values   map[string]string
		expected string // expected response file, empty if no match
	}{
		{"exact match", map[string]string{"order_id": "1001"}, "success.json"},
		{"prefix match", map[string]string{"order_id": "VIP_123"}, "vip.json"},
		{"no match", map[string]string{"order_id": "unknown"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchRules(tt.values, rules)
			if tt.expected == "" {
				if result != nil {
					t.Errorf("MatchRules() = %+v, want nil", result)
				}
			} else {
				if result == nil || result.ResponseFile != tt.expected {
					t.Errorf("MatchRules() = %+v, want ResponseFile=%s", result, tt.expected)
				}
			}
		})
	}
}

func TestMatchAllConditions(t *testing.T) {
	conditions := []Condition{
		{Selector: "order_id", MatchType: "prefix", Value: "VIP_"},
		{Selector: "user_type", MatchType: "exact", Value: "premium"},
	}

	tests := []struct {
		name     string
		values   map[string]string
		expected bool
	}{
		{"all match", map[string]string{"order_id": "VIP_123", "user_type": "premium"}, true},
		{"first no match", map[string]string{"order_id": "REG_123", "user_type": "premium"}, false},
		{"second no match", map[string]string{"order_id": "VIP_123", "user_type": "regular"}, false},
		{"both no match", map[string]string{"order_id": "REG_123", "user_type": "regular"}, false},
		{"missing value", map[string]string{"order_id": "VIP_123"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchAllConditions(tt.values, conditions)
			if result != tt.expected {
				t.Errorf("matchAllConditions() = %v, want %v", result, tt.expected)
			}
		})
	}
}
