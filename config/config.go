package config

import (
	"sync"
	"time"
)

// ==================== Main Config ====================

type Config struct {
	Server              ServerConfig `yaml:"server"`
	HealthCheck         HealthCheck  `yaml:"health_check"`
	Endpoints           []Endpoint   `yaml:"endpoints"`
	EndpointConfigPaths []string     `yaml:"-"`
}

// ==================== Server Config ====================

type ServerConfig struct {
	Port              int            `yaml:"port"`
	HotReload         bool           `yaml:"hot_reload"`
	ReloadIntervalSec int            `yaml:"reload_interval_sec"`
	Logging           LoggingConfig  `yaml:"logging"`
	ErrorHandling     ErrorHandling  `yaml:"error_handling"`
	CORS              CORSConfig     `yaml:"cors"`
	AdminAPI          AdminAPIConfig `yaml:"admin_api"`
	Recording         RecordingConfig `yaml:"recording"`
}

type LoggingConfig struct {
	Level     string `yaml:"level"` // debug, info, warn, error
	AccessLog bool   `yaml:"access_log"`
	LogFormat string `yaml:"log_format"` // json, text
	LogFile   string `yaml:"log_file"`   // optional, empty means stdout
}

type ErrorHandling struct {
	ShowDetails          bool           `yaml:"show_details"`
	CustomErrorResponses map[int]string `yaml:"custom_error_responses"` // status_code -> file_path
}

// ==================== CORS Config ====================

type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAgeSeconds    int      `yaml:"max_age_seconds"`
}

// ==================== Admin API Config ====================

type AdminAPIConfig struct {
	Enabled bool      `yaml:"enabled"`
	Prefix  string    `yaml:"prefix"` // default "/mock-admin"
	Auth    AdminAuth `yaml:"auth"`
}

type AdminAuth struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ==================== Recording Config ====================

type RecordingConfig struct {
	Enabled      bool     `yaml:"enabled"`
	MaxEntries   int      `yaml:"max_entries"`    // default 1000
	RecordBody   bool     `yaml:"record_body"`
	MaxBodyBytes int      `yaml:"max_body_bytes"` // default 65536
	ExcludePaths []string `yaml:"exclude_paths"`
}

// ==================== Health Check ====================

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
	// Scenario support
	Scenario    string `yaml:"scenario,omitempty"`
	ScenarioKey string `yaml:"scenario_key,omitempty"`
	// Proxy support
	Mode  string      `yaml:"mode,omitempty"` // "mock" (default) | "proxy"
	Proxy ProxyConfig `yaml:"proxy,omitempty"`
}

type Selector struct {
	Name string `yaml:"name"` // selector name, used in rules
	Type string `yaml:"type"` // body, header, query, path
	Key  string `yaml:"key"`  // json path or header/query/path key
}

// ==================== Rule Config ====================

type Rule struct {
	ConditionLogic  string           `yaml:"condition_logic,omitempty"` // "and" (default) | "or"
	Conditions      []Condition      `yaml:"conditions"`
	ConditionGroups []ConditionGroup `yaml:"condition_groups,omitempty"`
	// Scenario step support
	ScenarioStep string `yaml:"scenario_step,omitempty"` // "idle", "initiated", "any", etc.
	NextStep     string `yaml:"next_step,omitempty"`      // step to transition to on match
	ResponseConfig `yaml:",inline"`
}

type ConditionGroup struct {
	Logic      string      `yaml:"logic"` // "and" | "or"
	Conditions []Condition `yaml:"conditions"`
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
	ContentType     string            `yaml:"content_type,omitempty"`
	Template        *TemplateConfig   `yaml:"template,omitempty"`
	RandomResponses *RandomResponses  `yaml:"random_responses,omitempty"`
}

type TemplateConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Engine    string   `yaml:"engine,omitempty"` // "simple" (default) | "go"
	Variables []string `yaml:"variables"`        // supported variables list
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

// ==================== Proxy Config ====================

type ProxyConfig struct {
	Target          string            `yaml:"target"`
	StripPrefix     string            `yaml:"strip_prefix,omitempty"`
	TimeoutMs       int               `yaml:"timeout_ms,omitempty"`
	Record          bool              `yaml:"record,omitempty"`
	RecordDir       string            `yaml:"record_dir,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	FallbackOnError bool              `yaml:"fallback_on_error,omitempty"`
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
	mu               sync.RWMutex
	config           *Config
	configPath       string
	loadedAt         time.Time
	runtimeEndpoints []Endpoint
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager(path string) *ConfigManager {
	return &ConfigManager{
		configPath: path,
	}
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// GetLoadedAt returns when the config was last loaded
func (cm *ConfigManager) GetLoadedAt() time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.loadedAt
}

// SetConfig sets a new configuration
func (cm *ConfigManager) SetConfig(cfg *Config) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config = cfg
	cm.loadedAt = time.Now()
}

// GetConfigPath returns the config file path
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

// AddRuntimeEndpoint adds an endpoint at runtime (in-memory only)
func (cm *ConfigManager) AddRuntimeEndpoint(ep Endpoint) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.runtimeEndpoints = append(cm.runtimeEndpoints, ep)
}

// RemoveRuntimeEndpoint removes a runtime endpoint by index
func (cm *ConfigManager) RemoveRuntimeEndpoint(index int) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if index < 0 || index >= len(cm.runtimeEndpoints) {
		return false
	}
	cm.runtimeEndpoints = append(cm.runtimeEndpoints[:index], cm.runtimeEndpoints[index+1:]...)
	return true
}

// UpdateRuntimeEndpoint updates a runtime endpoint by index
func (cm *ConfigManager) UpdateRuntimeEndpoint(index int, ep Endpoint) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if index < 0 || index >= len(cm.runtimeEndpoints) {
		return false
	}
	cm.runtimeEndpoints[index] = ep
	return true
}

// GetRuntimeEndpoints returns a copy of runtime endpoints
func (cm *ConfigManager) GetRuntimeEndpoints() []Endpoint {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]Endpoint, len(cm.runtimeEndpoints))
	copy(result, cm.runtimeEndpoints)
	return result
}

// GetAllEndpoints returns combined file-based and runtime endpoints
func (cm *ConfigManager) GetAllEndpoints() []Endpoint {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	var all []Endpoint
	if cm.config != nil {
		all = append(all, cm.config.Endpoints...)
	}
	all = append(all, cm.runtimeEndpoints...)
	return all
}
