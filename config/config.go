package config

import "time"

// ==================== Main Config ====================

type Config struct {
	Server      ServerConfig `yaml:"server"`
	HealthCheck HealthCheck  `yaml:"health_check"`
	Endpoints   []Endpoint   `yaml:"endpoints"`
}

// ==================== Server Config ====================

type ServerConfig struct {
	Port              int           `yaml:"port"`
	HotReload         bool          `yaml:"hot_reload"`
	ReloadIntervalSec int           `yaml:"reload_interval_sec"`
	Logging           LoggingConfig `yaml:"logging"`
	ErrorHandling     ErrorHandling `yaml:"error_handling"`
}

type LoggingConfig struct {
	Level     string `yaml:"level"`      // debug, info, warn, error
	AccessLog bool   `yaml:"access_log"`
	LogFormat string `yaml:"log_format"` // json, text
	LogFile   string `yaml:"log_file"`   // optional, empty means stdout
}

type ErrorHandling struct {
	ShowDetails          bool           `yaml:"show_details"`
	CustomErrorResponses map[int]string `yaml:"custom_error_responses"` // status_code -> file_path
}

type HealthCheck struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// ==================== Endpoint Config ====================

type Endpoint struct {
	Path        string         `yaml:"path"`
	Method      string         `yaml:"method"`
	Description string         `yaml:"description"`
	Selectors   []Selector     `yaml:"selectors"`
	Rules       []Rule         `yaml:"rules"`
	Default     ResponseConfig `yaml:"default"`
}

type Selector struct {
	Name string `yaml:"name"` // selector name, used in rules
	Type string `yaml:"type"` // body, header, query, path
	Key  string `yaml:"key"`  // json path or header/query/path key
}

// ==================== Rule Config ====================

type Rule struct {
	Conditions     []Condition `yaml:"conditions"` // multiple conditions with AND logic
	ResponseConfig `yaml:",inline"`
}

type Condition struct {
	Selector  string `yaml:"selector"`   // reference to Selector name
	MatchType string `yaml:"match_type"` // exact, prefix, suffix, regex, range
	Value     string `yaml:"value"`      // match value
}

// ==================== Response Config ====================

type ResponseConfig struct {
	ResponseFile    string            `yaml:"response_file,omitempty"`
	StatusCode      int               `yaml:"status_code"`
	DelayMs         int               `yaml:"delay_ms,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	Template        *TemplateConfig   `yaml:"template,omitempty"`
	RandomResponses *RandomResponses  `yaml:"random_responses,omitempty"`
}

type TemplateConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Variables []string `yaml:"variables"` // supported variables list
}

type RandomResponses struct {
	Enabled bool             `yaml:"enabled"`
	Files   []RandomResponse `yaml:"files"`
}

type RandomResponse struct {
	File       string `yaml:"file"`
	Weight     int    `yaml:"weight"` // weight percentage
	StatusCode int    `yaml:"status_code"`
	DelayMs    int    `yaml:"delay_ms,omitempty"`
}

// ==================== Built-in Variables ====================

type BuiltinVariables struct {
	Timestamp time.Time
	UUID      string
	RequestID string
}

// ==================== Config Manager ====================

// ConfigManager manages configuration with thread-safe access
type ConfigManager struct {
	config     *Config
	configPath string
	loadedAt   time.Time
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager(path string) *ConfigManager {
	return &ConfigManager{
		configPath: path,
	}
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *Config {
	return cm.config
}

// GetLoadedAt returns when the config was last loaded
func (cm *ConfigManager) GetLoadedAt() time.Time {
	return cm.loadedAt
}

// SetConfig sets a new configuration
func (cm *ConfigManager) SetConfig(cfg *Config) {
	cm.config = cfg
	cm.loadedAt = time.Now()
}
