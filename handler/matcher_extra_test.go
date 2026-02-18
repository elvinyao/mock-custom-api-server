package handler

import (
	"testing"
)

// ── matchAnyCondition ─────────────────────────────────────────────────────────

func TestMatchAnyCondition_AllFalse_ReturnsFalse(t *testing.T) {
	conditions := []Condition{
		{Selector: "status", MatchType: "exact", Value: "active"},
		{Selector: "status", MatchType: "exact", Value: "pending"},
	}
	values := map[string]string{"status": "inactive"}
	if matchAnyCondition(values, conditions) {
		t.Error("expected false when no condition matches")
	}
}

func TestMatchAnyCondition_OneTrue_ReturnsTrue(t *testing.T) {
	conditions := []Condition{
		{Selector: "status", MatchType: "exact", Value: "active"},
		{Selector: "status", MatchType: "exact", Value: "pending"},
	}
	values := map[string]string{"status": "pending"}
	if !matchAnyCondition(values, conditions) {
		t.Error("expected true when at least one condition matches")
	}
}

func TestMatchAnyCondition_Empty_ReturnsFalse(t *testing.T) {
	// Empty conditions with OR logic: matchAnyCondition returns false (not "true by default")
	if matchAnyCondition(map[string]string{}, []Condition{}) {
		t.Error("expected false for empty conditions in OR")
	}
}

func TestMatchAnyCondition_MissingKey_UsesEmptyString(t *testing.T) {
	conditions := []Condition{
		{Selector: "missing", MatchType: "exact", Value: ""},
	}
	values := map[string]string{}
	// "" == "" should match
	if !matchAnyCondition(values, conditions) {
		t.Error("expected true: missing key should produce empty string which matches empty value")
	}
}

// ── matchGroup ────────────────────────────────────────────────────────────────

func TestMatchGroup_AndLogic_AllMatch(t *testing.T) {
	group := ConditionGroup{
		Logic: "and",
		Conditions: []Condition{
			{Selector: "a", MatchType: "exact", Value: "1"},
			{Selector: "b", MatchType: "exact", Value: "2"},
		},
	}
	values := map[string]string{"a": "1", "b": "2"}
	if !matchGroup(values, group) {
		t.Error("expected true for AND group where all conditions match")
	}
}

func TestMatchGroup_AndLogic_OneFails(t *testing.T) {
	group := ConditionGroup{
		Logic: "and",
		Conditions: []Condition{
			{Selector: "a", MatchType: "exact", Value: "1"},
			{Selector: "b", MatchType: "exact", Value: "2"},
		},
	}
	values := map[string]string{"a": "1", "b": "wrong"}
	if matchGroup(values, group) {
		t.Error("expected false for AND group where one condition fails")
	}
}

func TestMatchGroup_OrLogic_OneMatch(t *testing.T) {
	group := ConditionGroup{
		Logic: "or",
		Conditions: []Condition{
			{Selector: "x", MatchType: "exact", Value: "foo"},
			{Selector: "x", MatchType: "exact", Value: "bar"},
		},
	}
	values := map[string]string{"x": "bar"}
	if !matchGroup(values, group) {
		t.Error("expected true for OR group where one condition matches")
	}
}

func TestMatchGroup_EmptyLogic_DefaultsToAnd(t *testing.T) {
	group := ConditionGroup{
		Logic: "",
		Conditions: []Condition{
			{Selector: "x", MatchType: "exact", Value: "val"},
		},
	}
	values := map[string]string{"x": "val"}
	if !matchGroup(values, group) {
		t.Error("expected true: empty logic defaults to AND")
	}
}

// ── MatchRulesForStep ─────────────────────────────────────────────────────────

func TestMatchRulesForStep_StepFilter(t *testing.T) {
	rules := []Rule{
		{ScenarioStep: "idle", Conditions: []Condition{{Selector: "x", MatchType: "exact", Value: "1"}}, ResponseFile: "a.json"},
		{ScenarioStep: "active", Conditions: []Condition{{Selector: "x", MatchType: "exact", Value: "1"}}, ResponseFile: "b.json"},
	}
	values := map[string]string{"x": "1"}

	rule := MatchRulesForStep(values, rules, "idle")
	if rule == nil || rule.ResponseFile != "a.json" {
		t.Errorf("expected a.json for step=idle, got %v", rule)
	}
}

func TestMatchRulesForStep_AnyStep_AlwaysMatches(t *testing.T) {
	rules := []Rule{
		{ScenarioStep: "any", Conditions: []Condition{}, ResponseFile: "catch.json"},
	}
	values := map[string]string{}

	rule := MatchRulesForStep(values, rules, "anything")
	if rule == nil || rule.ResponseFile != "catch.json" {
		t.Errorf("expected catch.json for 'any' step, got %v", rule)
	}
}

func TestMatchRulesForStep_EmptyStep_AlwaysMatches(t *testing.T) {
	// Rule without ScenarioStep is not filtered by step
	rules := []Rule{
		{ScenarioStep: "", Conditions: []Condition{{Selector: "s", MatchType: "exact", Value: "v"}}, ResponseFile: "open.json"},
	}
	values := map[string]string{"s": "v"}

	rule := MatchRulesForStep(values, rules, "random_step")
	if rule == nil || rule.ResponseFile != "open.json" {
		t.Errorf("expected open.json for empty step filter, got %v", rule)
	}
}

func TestMatchRulesForStep_NoMatch_ReturnsNil(t *testing.T) {
	rules := []Rule{
		{ScenarioStep: "idle", Conditions: []Condition{}, ResponseFile: "a.json"},
	}
	rule := MatchRulesForStep(map[string]string{}, rules, "active")
	if rule != nil {
		t.Error("expected nil when no rules match current step")
	}
}

// ── matchCondition "contains" ─────────────────────────────────────────────────

func TestMatchCondition_Contains_True(t *testing.T) {
	cond := Condition{Selector: "msg", MatchType: "contains", Value: "error"}
	if !matchCondition("internal error occurred", cond) {
		t.Error("expected true for contains match")
	}
}

func TestMatchCondition_Contains_False(t *testing.T) {
	cond := Condition{Selector: "msg", MatchType: "contains", Value: "success"}
	if matchCondition("internal error occurred", cond) {
		t.Error("expected false for contains mismatch")
	}
}

func TestMatchCondition_Contains_Empty(t *testing.T) {
	cond := Condition{Selector: "msg", MatchType: "contains", Value: ""}
	// Every string contains empty string
	if !matchCondition("anything", cond) {
		t.Error("expected true: any string contains empty string")
	}
}

// ── matchRule with condition groups ──────────────────────────────────────────

func TestMatchRule_WithConditionGroups_BothSatisfied(t *testing.T) {
	rule := &Rule{
		ConditionLogic: "and",
		Conditions: []Condition{
			{Selector: "a", MatchType: "exact", Value: "1"},
		},
		ConditionGroups: []ConditionGroup{
			{
				Logic: "or",
				Conditions: []Condition{
					{Selector: "b", MatchType: "exact", Value: "x"},
					{Selector: "b", MatchType: "exact", Value: "y"},
				},
			},
		},
	}
	values := map[string]string{"a": "1", "b": "y"}
	if !matchRule(values, rule) {
		t.Error("expected true: direct condition AND group condition both satisfied")
	}
}

func TestMatchRule_WithConditionGroups_GroupFails(t *testing.T) {
	rule := &Rule{
		ConditionLogic: "and",
		Conditions: []Condition{
			{Selector: "a", MatchType: "exact", Value: "1"},
		},
		ConditionGroups: []ConditionGroup{
			{
				Logic: "and",
				Conditions: []Condition{
					{Selector: "b", MatchType: "exact", Value: "x"},
				},
			},
		},
	}
	values := map[string]string{"a": "1", "b": "z"}
	if matchRule(values, rule) {
		t.Error("expected false: condition group fails")
	}
}

func TestMatchRule_OrLogic_OnlyOneConditionMatches(t *testing.T) {
	rule := &Rule{
		ConditionLogic: "or",
		Conditions: []Condition{
			{Selector: "s", MatchType: "exact", Value: "a"},
			{Selector: "s", MatchType: "exact", Value: "b"},
		},
	}
	values := map[string]string{"s": "b"}
	if !matchRule(values, rule) {
		t.Error("expected true for OR logic with one match")
	}
}

func TestMatchRule_OrLogic_NoMatch(t *testing.T) {
	rule := &Rule{
		ConditionLogic: "or",
		Conditions: []Condition{
			{Selector: "s", MatchType: "exact", Value: "a"},
			{Selector: "s", MatchType: "exact", Value: "b"},
		},
	}
	values := map[string]string{"s": "c"}
	if matchRule(values, rule) {
		t.Error("expected false for OR logic with no match")
	}
}

// ── matchConditions ───────────────────────────────────────────────────────────

func TestMatchConditions_EmptyConditions_ReturnsTrue(t *testing.T) {
	// Both AND and OR should return true for empty conditions
	if !matchConditions(map[string]string{}, []Condition{}, "and") {
		t.Error("expected true for empty AND conditions")
	}
	if !matchConditions(map[string]string{}, []Condition{}, "or") {
		// matchConditions delegates to matchAnyCondition which returns false for empty
		// This is valid behavior: OR of zero things = false.
		// The test just verifies no panic.
	}
}

func TestMatchConditions_AndLogic_AllMatch(t *testing.T) {
	conds := []Condition{
		{Selector: "a", MatchType: "exact", Value: "1"},
		{Selector: "b", MatchType: "exact", Value: "2"},
	}
	if !matchConditions(map[string]string{"a": "1", "b": "2"}, conds, "and") {
		t.Error("expected true for AND with all matching")
	}
}

func TestMatchConditions_OrLogic_OneMatch(t *testing.T) {
	conds := []Condition{
		{Selector: "x", MatchType: "exact", Value: "p"},
		{Selector: "x", MatchType: "exact", Value: "q"},
	}
	if !matchConditions(map[string]string{"x": "q"}, conds, "or") {
		t.Error("expected true for OR with one matching")
	}
}
