package handler

import (
	"regexp"
	"strconv"
	"strings"
)

// Condition represents a matching condition
type Condition struct {
	Selector  string
	MatchType string
	Value     string
}

// Rule represents a matching rule with conditions and response
type Rule struct {
	Conditions   []Condition
	ResponseFile string
	StatusCode   int
	DelayMs      int
	Headers      map[string]string
}

// MatchRules finds the first matching rule based on extracted values
// Returns nil if no rule matches
func MatchRules(values map[string]string, rules []Rule) *Rule {
	for i := range rules {
		rule := &rules[i]
		if matchAllConditions(values, rule.Conditions) {
			return rule
		}
	}
	return nil
}

// matchAllConditions checks if all conditions in a rule match (AND logic)
func matchAllConditions(values map[string]string, conditions []Condition) bool {
	for _, cond := range conditions {
		targetValue, exists := values[cond.Selector]
		if !exists {
			targetValue = ""
		}

		if !matchCondition(targetValue, cond) {
			return false
		}
	}
	return true
}

// matchCondition checks if a single condition matches
func matchCondition(targetValue string, cond Condition) bool {
	switch strings.ToLower(cond.MatchType) {
	case "exact":
		return targetValue == cond.Value

	case "prefix":
		return strings.HasPrefix(targetValue, cond.Value)

	case "suffix":
		return strings.HasSuffix(targetValue, cond.Value)

	case "regex":
		matched, err := regexp.MatchString(cond.Value, targetValue)
		if err != nil {
			return false
		}
		return matched

	case "range":
		return matchRange(targetValue, cond.Value)

	default:
		// Default to exact match
		return targetValue == cond.Value
	}
}

// matchRange checks if a numeric value is within a range
// Range format: "[min, max]" or "(min, max)" for exclusive bounds
// Examples: "[1, 100]" means 1 <= value <= 100
//
//	"(0, 100)" means 0 < value < 100
func matchRange(targetValue string, rangeStr string) bool {
	// Parse target value as number
	num, err := strconv.ParseFloat(targetValue, 64)
	if err != nil {
		return false
	}

	// Parse range string
	rangeStr = strings.TrimSpace(rangeStr)
	if len(rangeStr) < 5 { // minimum: "[1,2]"
		return false
	}

	// Determine inclusive/exclusive bounds
	leftInclusive := rangeStr[0] == '['
	rightInclusive := rangeStr[len(rangeStr)-1] == ']'

	// Extract numbers
	inner := rangeStr[1 : len(rangeStr)-1]
	parts := strings.Split(inner, ",")
	if len(parts) != 2 {
		return false
	}

	minVal, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return false
	}

	maxVal, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return false
	}

	// Check bounds
	var minOK, maxOK bool
	if leftInclusive {
		minOK = num >= minVal
	} else {
		minOK = num > minVal
	}

	if rightInclusive {
		maxOK = num <= maxVal
	} else {
		maxOK = num < maxVal
	}

	return minOK && maxOK
}
