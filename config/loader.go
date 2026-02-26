package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type rawConfig struct {
	Server      ServerConfig `yaml:"server"`
	HealthCheck HealthCheck  `yaml:"health_check"`
	Endpoints   yaml.Node    `yaml:"endpoints"`
}

type endpointPathsConfig struct {
	ConfigPaths []string `yaml:"config_paths"`
}

type endpointFileConfig struct {
	Endpoint  `yaml:",inline"`
	Paths     []Endpoint `yaml:"paths"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

// expandEnvVars replaces ${VAR} and $VAR patterns in s with the corresponding
// environment variable values. Unset variables are replaced with an empty string.
func expandEnvVars(s string) string {
	return os.Expand(s, os.Getenv)
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand ${ENV_VAR} references before YAML parsing
	expanded := expandEnvVars(string(data))

	var raw rawConfig
	if err := yaml.Unmarshal([]byte(expanded), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	endpoints, endpointConfigPaths, err := parseEndpoints(raw.Endpoints, path)
	if err != nil {
		return nil, err
	}

	cfg := Config{
		Server:              raw.Server,
		HealthCheck:         raw.HealthCheck,
		Endpoints:           endpoints,
		EndpointConfigPaths: endpointConfigPaths,
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

func parseEndpoints(node yaml.Node, mainConfigPath string) ([]Endpoint, []string, error) {
	// endpoints section omitted
	if node.Kind == 0 {
		return nil, nil, nil
	}

	switch node.Kind {
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			return nil, nil, nil
		}
		if allChildrenKind(node, yaml.ScalarNode) {
			var endpointPaths []string
			if err := node.Decode(&endpointPaths); err != nil {
				return nil, nil, fmt.Errorf("failed to parse endpoints as config path list: %w", err)
			}
			return loadEndpointsFromPaths(mainConfigPath, endpointPaths)
		}
		if allChildrenKind(node, yaml.MappingNode) {
			var endpoints []Endpoint
			if err := node.Decode(&endpoints); err != nil {
				return nil, nil, fmt.Errorf("failed to parse inline endpoints: %w", err)
			}
			return endpoints, nil, nil
		}
		return nil, nil, fmt.Errorf("invalid endpoints format: sequence entries must be all strings or all mappings")

	case yaml.MappingNode:
		var pathsCfg endpointPathsConfig
		if err := node.Decode(&pathsCfg); err == nil && len(pathsCfg.ConfigPaths) > 0 {
			return loadEndpointsFromPaths(mainConfigPath, pathsCfg.ConfigPaths)
		}

		// Backward-compatible: support a single inline endpoint mapping.
		var endpoint Endpoint
		if err := node.Decode(&endpoint); err != nil {
			return nil, nil, fmt.Errorf("failed to parse endpoints mapping: %w", err)
		}
		if !hasEndpointContent(endpoint) {
			return nil, nil, fmt.Errorf("invalid endpoints mapping: expected config_paths or a valid endpoint")
		}
		return []Endpoint{endpoint}, nil, nil

	default:
		return nil, nil, fmt.Errorf("invalid endpoints format: expected sequence or mapping")
	}
}

func allChildrenKind(node yaml.Node, kind yaml.Kind) bool {
	for _, child := range node.Content {
		if child.Kind != kind {
			return false
		}
	}
	return true
}

func loadEndpointsFromPaths(mainConfigPath string, configPaths []string) ([]Endpoint, []string, error) {
	baseDir := filepath.Dir(mainConfigPath)
	var endpoints []Endpoint
	var resolvedPaths []string

	for i, endpointPath := range configPaths {
		trimmed := strings.TrimSpace(endpointPath)
		if trimmed == "" {
			return nil, nil, fmt.Errorf("endpoints.config_paths[%d] is empty", i)
		}

		resolvedPath := trimmed
		if !filepath.IsAbs(trimmed) {
			resolvedPath = filepath.Join(baseDir, trimmed)
		}
		resolvedPath = filepath.Clean(resolvedPath)

		loaded, err := loadEndpointsFromFile(resolvedPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load endpoint config '%s': %w", trimmed, err)
		}
		endpoints = append(endpoints, loaded...)
		resolvedPaths = append(resolvedPaths, resolvedPath)
	}

	return endpoints, resolvedPaths, nil
}

func loadEndpointsFromFile(path string) ([]Endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse endpoint config file: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty endpoint config file")
	}

	root := doc.Content[0]
	switch root.Kind {
	case yaml.SequenceNode:
		var endpoints []Endpoint
		if err := root.Decode(&endpoints); err != nil {
			return nil, fmt.Errorf("invalid endpoints sequence: %w", err)
		}
		if len(endpoints) == 0 {
			return nil, fmt.Errorf("endpoint config file has empty endpoints sequence")
		}
		return endpoints, nil

	case yaml.MappingNode:
		var fileCfg endpointFileConfig
		if err := root.Decode(&fileCfg); err != nil {
			return nil, fmt.Errorf("invalid endpoint config mapping: %w", err)
		}

		var endpoints []Endpoint
		if hasEndpointContent(fileCfg.Endpoint) {
			endpoints = append(endpoints, fileCfg.Endpoint)
		}
		if len(fileCfg.Paths) > 0 {
			endpoints = append(endpoints, fileCfg.Paths...)
		}
		if len(fileCfg.Endpoints) > 0 {
			endpoints = append(endpoints, fileCfg.Endpoints...)
		}

		if len(endpoints) == 0 {
			return nil, fmt.Errorf("endpoint config must define 'path' or 'paths' or 'endpoints'")
		}
		return endpoints, nil

	default:
		return nil, fmt.Errorf("invalid endpoint config: expected mapping or sequence")
	}
}

func hasEndpointContent(ep Endpoint) bool {
	return ep.Path != "" ||
		ep.Method != "" ||
		ep.Description != "" ||
		len(ep.Selectors) > 0 ||
		len(ep.Rules) > 0 ||
		ep.Default.ResponseFile != "" ||
		ep.Default.StatusCode != 0 ||
		ep.Default.DelayMs != 0 ||
		len(ep.Default.Headers) > 0 ||
		ep.Default.Template != nil ||
		ep.Default.RandomResponses != nil
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
