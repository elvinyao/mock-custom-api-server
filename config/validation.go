package config

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	templatepkg "mock-api-server/pkg/template"
)

// LoadValidatedConfig loads a complete candidate and rejects it before activation
// if any diagnostic is found. LoadConfig remains available for inspection tools.
func LoadValidatedConfig(path string) (*Config, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if diagnostics := ValidateConfig(cfg); len(diagnostics) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n%s", strings.Join(diagnostics, "\n"))
	}
	return cfg, nil
}

// ValidateConfig preserves the diagnostic-list interface used by callers.
func ValidateConfig(cfg *Config) []string {
	if cfg == nil {
		return []string{"configuration is nil"}
	}
	v := &validation{}
	prefix := cfg.sourceFile
	if prefix != "" {
		prefix += ": "
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		v.add(prefix+"port", "must be between 1 and 65535")
	}
	if cfg.ReloadIntervalSec < 0 || int64(cfg.ReloadIntervalSec) > math.MaxInt64/int64(time.Second) {
		v.add(prefix+"reload_interval_sec", "must be a nonnegative duration in seconds")
	}
	for i, ep := range cfg.Endpoints {
		field := fmt.Sprintf("endpoints[%d]", i)
		if ep.sourceFile != "" {
			field = fmt.Sprintf("%s: endpoints[%d]", ep.sourceFile, ep.sourceIndex)
		}
		v.endpoint(ep, field)
	}
	for code, file := range cfg.ErrorHandling.CustomErrorResponses {
		field := fmt.Sprintf("%serror_handling.custom_error_responses[%d]", prefix, code)
		if code < 100 || code > 599 {
			v.add(field, "invalid status code")
		}
		v.file(file, field, "file", nil)
	}
	return v.diagnostics
}

// ValidateEndpoint validates the same endpoint rules used by file configuration.
func ValidateEndpoint(ep Endpoint) error {
	v := &validation{}
	v.endpoint(ep, "endpoint")
	if len(v.diagnostics) > 0 {
		return fmt.Errorf("%s", strings.Join(v.diagnostics, "\n"))
	}
	return nil
}

type validation struct{ diagnostics []string }

func (v *validation) add(field, message string) {
	v.diagnostics = append(v.diagnostics, field+": "+message)
}

var paramName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var methodToken = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

func (v *validation) endpoint(ep Endpoint, field string) {
	if err := validatePath(ep.Path); err != nil {
		v.add(field+".path", err.Error())
	}
	if ep.Method == "" {
		v.add(field+".method", "method is empty")
	} else if !methodToken.MatchString(ep.Method) {
		v.add(field+".method", "invalid HTTP method")
	}
	names := map[string]bool{}
	for i, sel := range ep.Selectors {
		p := fmt.Sprintf("%s.selectors[%d]", field, i)
		if sel.Name == "" {
			v.add(p+".name", "name is empty")
		} else if names[sel.Name] {
			v.add(p+".name", "duplicate name '"+sel.Name+"'")
		}
		names[sel.Name] = true
		if !isValidSelectorType(sel.Type) {
			v.add(p+".type", "invalid type '"+sel.Type+"'")
		}
		if sel.Key == "" {
			v.add(p+".key", "key is empty")
		}
	}
	for i, rule := range ep.Rules {
		p := fmt.Sprintf("%s.rules[%d]", field, i)
		v.logic(rule.ConditionLogic, p+".condition_logic")
		v.conditions(rule.Conditions, names, p+".conditions")
		for j, group := range rule.ConditionGroups {
			gp := fmt.Sprintf("%s.condition_groups[%d]", p, j)
			v.logic(group.Logic, gp+".logic")
			v.conditions(group.Conditions, names, gp+".conditions")
		}
		v.response(rule.ResponseConfig, p)
	}
	v.response(ep.Default, field+".default")
}

func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?# \t\r\n") {
		return fmt.Errorf("path must start with / and contain no query, fragment, or whitespace")
	}
	seen := map[string]bool{}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if !strings.ContainsAny(part, ":*") {
			continue
		}
		if part[0] != ':' && part[0] != '*' {
			return fmt.Errorf("parameters and wildcards must occupy a whole segment")
		}
		if part[0] == '*' && i != len(parts)-1 {
			return fmt.Errorf("wildcard must be the final segment")
		}
		name := part[1:]
		if name == "" && part == "*" {
			continue
		}
		if !paramName.MatchString(name) {
			return fmt.Errorf("invalid parameter name %q", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate parameter name %q", name)
		}
		seen[name] = true
	}
	return nil
}

func (v *validation) logic(logic, field string) {
	if logic != "" && !strings.EqualFold(logic, "and") && !strings.EqualFold(logic, "or") {
		v.add(field, "must be AND or OR")
	}
}
func (v *validation) conditions(conditions []Condition, names map[string]bool, field string) {
	for i, cond := range conditions {
		p := fmt.Sprintf("%s[%d]", field, i)
		if !names[cond.Selector] {
			v.add(p+".selector", "unknown selector '"+cond.Selector+"'")
		}
		if !isValidMatchType(cond.MatchType) {
			v.add(p+".match_type", "invalid match_type '"+cond.MatchType+"'")
		}
		switch strings.ToLower(cond.MatchType) {
		case "regex":
			if _, err := regexp.Compile(cond.Value); err != nil {
				v.add(p+".value", "invalid regex: "+err.Error())
			}
		case "range":
			if !validRange(cond.Value) {
				v.add(p+".value", "invalid range; expected [min,max] or (min,max) with ordered finite bounds")
			}
		}
	}
}
func validRange(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 5 || !strings.ContainsRune("[(", rune(value[0])) || !strings.ContainsRune(")]", rune(value[len(value)-1])) {
		return false
	}
	parts := strings.Split(value[1:len(value)-1], ",")
	if len(parts) != 2 {
		return false
	}
	min, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	max, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	return err1 == nil && err2 == nil && !math.IsNaN(min) && !math.IsNaN(max) && !math.IsInf(min, 0) && !math.IsInf(max, 0) && min <= max
}
func (v *validation) statusDelay(status, delay int, field string) {
	if status != 0 && (status < 100 || status > 599) {
		v.add(field+".status_code", "must be 0 (default) or between 100 and 599")
	}
	if delay < 0 || int64(delay) > math.MaxInt64/int64(time.Millisecond) {
		v.add(field+".delay_ms", "must be a nonnegative duration in milliseconds")
	}
}
func (v *validation) response(response ResponseConfig, field string) {
	v.statusDelay(response.StatusCode, response.DelayMs, field)
	var tmpl *TemplateConfig
	if response.Template != nil && response.Template.Enabled {
		tmpl = response.Template
		if err := templatepkg.Validate(nil, tmpl.Engine); err != nil {
			v.add(field+".template.engine", err.Error())
			tmpl = nil
		}
	}
	if response.ResponseFile != "" {
		v.file(response.ResponseFile, field+".response_file", "response_file", tmpl)
	}
	if tmpl != nil {
		if err := templatepkg.Validate([]byte(response.InlineBody), tmpl.Engine); err != nil {
			v.add(field+".inline_body", err.Error())
		}
		keys := make([]string, 0, len(response.Headers))
		for key := range response.Headers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := templatepkg.Validate([]byte(response.Headers[key]), tmpl.Engine); err != nil {
				v.add(field+".headers["+key+"]", err.Error())
			}
		}
	}
	if random := response.RandomResponses; random != nil && random.Enabled {
		if len(random.Files) == 0 {
			v.add(field+".random_responses.files", "must contain at least one file")
		}
		total := 0
		for i, item := range random.Files {
			p := fmt.Sprintf("%s.random_responses.files[%d]", field, i)
			if item.Weight < 0 {
				v.add(p+".weight", "must be nonnegative")
			} else if item.Weight > math.MaxInt-total {
				v.add(p+".weight", "total weight overflows int")
			} else {
				total += item.Weight
			}
			v.statusDelay(item.StatusCode, item.DelayMs, p)
			v.file(item.File, p+".file", "file", tmpl)
		}
	}
}
func (v *validation) file(path, field, label string, tmpl *TemplateConfig) {
	data, err := os.ReadFile(path)
	if err != nil {
		v.add(field, label+" not found or unreadable: "+err.Error())
		return
	}
	if tmpl != nil {
		if err := templatepkg.Validate(data, tmpl.Engine); err != nil {
			v.add(field, fmt.Sprintf("%s: %v", path, err))
		}
	}
}
