package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type rawConfig struct {
	Port              int             `yaml:"port"`
	HotReload         bool            `yaml:"hot_reload"`
	ReloadIntervalSec int             `yaml:"reload_interval_sec"`
	Logging           LoggingConfig   `yaml:"logging"`
	ErrorHandling     ErrorHandling   `yaml:"error_handling"`
	CORS              CORSConfig      `yaml:"cors"`
	AdminAPI          AdminAPIConfig  `yaml:"admin_api"`
	Recording         RecordingConfig `yaml:"recording"`
	TLS               TLSConfig       `yaml:"tls"`
	HealthCheck       HealthCheck     `yaml:"health_check"`
	Endpoints         yaml.Node       `yaml:"endpoints"`
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
	if err := decodeStrict([]byte(expanded), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	endpoints, endpointConfigPaths, err := parseEndpoints(raw.Endpoints, path)
	if err != nil {
		return nil, err
	}

	cfg := Config{
		sourceFile:          path,
		Port:                raw.Port,
		HotReload:           raw.HotReload,
		ReloadIntervalSec:   raw.ReloadIntervalSec,
		Logging:             raw.Logging,
		ErrorHandling:       raw.ErrorHandling,
		CORS:                raw.CORS,
		AdminAPI:            raw.AdminAPI,
		Recording:           raw.Recording,
		TLS:                 raw.TLS,
		HealthCheck:         raw.HealthCheck,
		Endpoints:           endpoints,
		EndpointConfigPaths: endpointConfigPaths,
	}
	for i := range cfg.Endpoints {
		if cfg.Endpoints[i].sourceFile == "" {
			cfg.Endpoints[i].sourceFile = path
			cfg.Endpoints[i].sourceIndex = i
		}
	}

	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.LogFormat == "" {
		cfg.Logging.LogFormat = "json"
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
			if err := decodeStrictNode(node, &endpoints, "endpoints"); err != nil {
				return nil, nil, fmt.Errorf("failed to parse inline endpoints: %w", err)
			}
			return endpoints, nil, nil
		}
		return nil, nil, fmt.Errorf("invalid endpoints format: sequence entries must be all strings or all mappings")

	case yaml.MappingNode:
		var pathsCfg endpointPathsConfig
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == "config_paths" {
				if err := decodeStrictNode(node, &pathsCfg, "endpoints"); err != nil {
					return nil, nil, fmt.Errorf("invalid endpoints.config_paths: %w", err)
				}
				return loadEndpointsFromPaths(mainConfigPath, pathsCfg.ConfigPaths)
			}
		}

		// Backward-compatible: support a single inline endpoint mapping.
		var endpoint Endpoint
		if err := decodeStrictNode(node, &endpoint, "endpoints"); err != nil {
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
			return nil, nil, fmt.Errorf("failed to load endpoint config '%s': %w", resolvedPath, err)
		}
		endpoints = append(endpoints, loaded...)
		resolvedPaths = append(resolvedPaths, resolvedPath)
	}

	return endpoints, resolvedPaths, nil
}

func loadEndpointsFromFile(path string) (endpoints []Endpoint, err error) {
	defer func() {
		for i := range endpoints {
			endpoints[i].sourceFile = path
			endpoints[i].sourceIndex = i
		}
	}()
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
		if err := decodeStrictNode(*root, &endpoints, "endpoints"); err != nil {
			return nil, fmt.Errorf("invalid endpoints sequence: %w", err)
		}
		if len(endpoints) == 0 {
			return nil, fmt.Errorf("endpoint config file has empty endpoints sequence")
		}
		return endpoints, nil

	case yaml.MappingNode:
		var fileCfg endpointFileConfig
		if err := decodeStrictNode(*root, &fileCfg, ""); err != nil {
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
	case "exact", "prefix", "suffix", "regex", "range", "contains":
		return true
	default:
		return false
	}
}
