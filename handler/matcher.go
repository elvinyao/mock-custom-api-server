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

// ConditionGroup represents a group of conditions with its own logic
type ConditionGroup struct {
	Logic      string // "and" | "or"
	Conditions []Condition
}

// TemplateConfig holds template settings for a rule
type TemplateConfig struct {
	Enabled bool
	Engine  string // "simple" | "go"
}

// Rule represents a matching rule with conditions and response
type Rule struct {
	ConditionLogic  string // "and" (default) | "or"
	Conditions      []Condition
	ConditionGroups []ConditionGroup
	// Response fields
	ResponseFile string
	InlineBody   string
	StatusCode   int
	DelayMs      int
	Headers      map[string]string
	ContentType  string
	Template     *TemplateConfig
}

// MatchRules finds the first matching rule based on extracted values
// Returns nil if no rule matches
func MatchRules(values map[string]string, rules []Rule) *Rule {
	for i := range rules {
		rule := &rules[i]
		if matchRule(values, rule) {
			return rule
		}
	}
	return nil
}

// matchRule checks if all conditions/groups in a rule match according to its logic
func matchRule(values map[string]string, rule *Rule) bool {
	logic := strings.ToLower(rule.ConditionLogic)
	if logic == "" {
		logic = "and"
	}

	// Evaluate direct conditions
	condResult := matchConditions(values, rule.Conditions, logic)

	// If no condition groups, return direct condition result
	if len(rule.ConditionGroups) == 0 {
		return condResult
	}

	// Evaluate condition groups (always AND'd with direct conditions)
	groupResult := true
	for _, group := range rule.ConditionGroups {
		if !matchGroup(values, group) {
			groupResult = false
			break
		}
	}

	return condResult && groupResult
}

// matchConditions evaluates a list of conditions with the given logic ("and" | "or")
func matchConditions(values map[string]string, conditions []Condition, logic string) bool {
	if len(conditions) == 0 {
		return true
	}

	if logic == "or" {
		return matchAnyCondition(values, conditions)
	}
	return matchAllConditions(values, conditions)
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

// matchAnyCondition checks if any condition matches (OR logic)
func matchAnyCondition(values map[string]string, conditions []Condition) bool {
	for _, cond := range conditions {
		targetValue, exists := values[cond.Selector]
		if !exists {
			targetValue = ""
		}

		if matchCondition(targetValue, cond) {
			return true
		}
	}
	return false
}

// matchGroup evaluates a ConditionGroup
func matchGroup(values map[string]string, group ConditionGroup) bool {
	logic := strings.ToLower(group.Logic)
	if logic == "" {
		logic = "and"
	}
	return matchConditions(values, group.Conditions, logic)
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

	case "contains":
		return strings.Contains(targetValue, cond.Value)

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
