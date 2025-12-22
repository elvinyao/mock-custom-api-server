package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Logging.Level == "" {
		cfg.Server.Logging.Level = "info"
	}
	if cfg.Server.Logging.LogFormat == "" {
		cfg.Server.Logging.LogFormat = "json"
	}
	if cfg.HealthCheck.Path == "" && cfg.HealthCheck.Enabled {
		cfg.HealthCheck.Path = "/health"
	}

	return &cfg, nil
}

// ValidateConfig validates the configuration and returns warnings
func ValidateConfig(cfg *Config) []string {
	var warnings []string

	// Validate endpoints
	for i, ep := range cfg.Endpoints {
		// Check path
		if ep.Path == "" {
			warnings = append(warnings, fmt.Sprintf("endpoint[%d]: path is empty", i))
		}

		// Check method
		if ep.Method == "" {
			warnings = append(warnings, fmt.Sprintf("endpoint[%d]: method is empty", i))
		}

		// Validate selectors
		selectorNames := make(map[string]bool)
		for j, sel := range ep.Selectors {
			if sel.Name == "" {
				warnings = append(warnings, fmt.Sprintf("endpoint[%d].selector[%d]: name is empty", i, j))
			}
			if selectorNames[sel.Name] {
				warnings = append(warnings, fmt.Sprintf("endpoint[%d].selector[%d]: duplicate name '%s'", i, j, sel.Name))
			}
			selectorNames[sel.Name] = true

			if !isValidSelectorType(sel.Type) {
				warnings = append(warnings, fmt.Sprintf("endpoint[%d].selector[%d]: invalid type '%s'", i, j, sel.Type))
			}
		}

		// Validate rules
		for j, rule := range ep.Rules {
			for k, cond := range rule.Conditions {
				// Check if selector exists
				if !selectorNames[cond.Selector] {
					warnings = append(warnings, fmt.Sprintf("endpoint[%d].rule[%d].condition[%d]: unknown selector '%s'", i, j, k, cond.Selector))
				}

				// Validate match type
				if !isValidMatchType(cond.MatchType) {
					warnings = append(warnings, fmt.Sprintf("endpoint[%d].rule[%d].condition[%d]: invalid match_type '%s'", i, j, k, cond.MatchType))
				}

				// Validate regex patterns
				if cond.MatchType == "regex" {
					if _, err := regexp.Compile(cond.Value); err != nil {
						warnings = append(warnings, fmt.Sprintf("endpoint[%d].rule[%d].condition[%d]: invalid regex '%s': %v", i, j, k, cond.Value, err))
					}
				}
			}

			// Check response file exists
			if rule.ResponseFile != "" {
				if _, err := os.Stat(rule.ResponseFile); os.IsNotExist(err) {
					warnings = append(warnings, fmt.Sprintf("endpoint[%d].rule[%d]: response_file not found: %s", i, j, rule.ResponseFile))
				}
			}
		}

		// Check default response file
		if ep.Default.ResponseFile != "" {
			if _, err := os.Stat(ep.Default.ResponseFile); os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("endpoint[%d].default: response_file not found: %s", i, ep.Default.ResponseFile))
			}
		}

		// Check random response files
		if ep.Default.RandomResponses != nil && ep.Default.RandomResponses.Enabled {
			for j, rr := range ep.Default.RandomResponses.Files {
				if _, err := os.Stat(rr.File); os.IsNotExist(err) {
					warnings = append(warnings, fmt.Sprintf("endpoint[%d].default.random_responses[%d]: file not found: %s", i, j, rr.File))
				}
			}
		}
	}

	// Check custom error response files
	for code, file := range cfg.Server.ErrorHandling.CustomErrorResponses {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("error_handling.custom_error_responses[%d]: file not found: %s", code, file))
		}
	}

	return warnings
}

func isValidSelectorType(t string) bool {
	switch strings.ToLower(t) {
	case "body", "header", "query", "path":
		return true
	default:
		return false
	}
}

func isValidMatchType(t string) bool {
	switch strings.ToLower(t) {
	case "exact", "prefix", "suffix", "regex", "range":
		return true
	default:
		return false
	}
}
