package handler

import "strings"

// MatchTrace is a request-local snapshot, published only after response selection.
// It contains no references to mutable configuration objects.
type MatchTrace struct {
	Endpoint       *TraceEndpoint    `json:"endpoint"`
	Selectors      map[string]string `json:"selectors"`
	Rules          []RuleTrace       `json:"rules"`
	SelectedRule   string            `json:"selected_rule"`
	ResponseSource string            `json:"response_source"`
}

type TraceEndpoint struct {
	Path   string `json:"path"`
	Method string `json:"method"`
	Mode   string `json:"mode"`
}

type RuleTrace struct {
	Index      int              `json:"index"`
	Matched    bool             `json:"matched"`
	Conditions []ConditionTrace `json:"conditions"`
	Groups     []GroupTrace     `json:"groups"`
}

type GroupTrace struct {
	Logic      string           `json:"logic"`
	Matched    bool             `json:"matched"`
	Conditions []ConditionTrace `json:"conditions"`
}

type ConditionTrace struct {
	Selector  string `json:"selector"`
	Actual    string `json:"actual"`
	Expected  string `json:"expected"`
	MatchType string `json:"match_type"`
	Matched   bool   `json:"matched"`
}

// MatchRulesWithTrace explains every configured rule while selecting the first
// match. MatchRules uses this same evaluator, including empty condition lists.
func MatchRulesWithTrace(values map[string]string, rules []Rule) (*Rule, []RuleTrace) {
	traces := make([]RuleTrace, len(rules))
	var selected *Rule
	for i := range rules {
		traces[i] = evaluateRule(values, &rules[i])
		traces[i].Index = i
		if selected == nil && traces[i].Matched {
			selected = &rules[i]
		}
	}
	return selected, traces
}

func normalizedLogic(logic string) string {
	if strings.EqualFold(logic, "or") {
		return "or"
	}
	return "and"
}

func evaluateRule(values map[string]string, rule *Rule) RuleTrace {
	matched, conditions := evaluateConditions(values, rule.Conditions, rule.ConditionLogic)
	trace := RuleTrace{Matched: matched, Conditions: conditions, Groups: make([]GroupTrace, len(rule.ConditionGroups))}
	for i, group := range rule.ConditionGroups {
		groupMatched, groupConditions := evaluateConditions(values, group.Conditions, group.Logic)
		trace.Groups[i] = GroupTrace{Logic: normalizedLogic(group.Logic), Matched: groupMatched, Conditions: groupConditions}
		trace.Matched = trace.Matched && groupMatched
	}
	return trace
}

func evaluateConditions(values map[string]string, conditions []Condition, logic string) (bool, []ConditionTrace) {
	or := normalizedLogic(logic) == "or"
	matched := !or || len(conditions) == 0
	traces := make([]ConditionTrace, len(conditions))
	for i, cond := range conditions {
		actual := values[cond.Selector]
		conditionMatched := matchCondition(actual, cond)
		matchType := strings.ToLower(cond.MatchType)
		if matchType == "" {
			matchType = "exact"
		}
		traces[i] = ConditionTrace{Selector: cond.Selector, Actual: actual, Expected: cond.Value, MatchType: matchType, Matched: conditionMatched}
		if or {
			matched = matched || conditionMatched
		} else {
			matched = matched && conditionMatched
		}
	}
	return matched, traces
}
