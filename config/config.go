package config

import (
	"sync"
	"time"
)

// ==================== Main Config ====================

type Config struct {
	Port              int             `yaml:"port"                json:"port"`
	HotReload         bool            `yaml:"hot_reload"          json:"hot_reload"`
	ReloadIntervalSec int             `yaml:"reload_interval_sec" json:"reload_interval_sec"`
	Logging           LoggingConfig   `yaml:"logging"             json:"logging"`
	ErrorHandling     ErrorHandling   `yaml:"error_handling"      json:"error_handling"`
	CORS              CORSConfig      `yaml:"cors"                json:"cors"`
	AdminAPI          AdminAPIConfig  `yaml:"admin_api"           json:"admin_api"`
	Recording         RecordingConfig `yaml:"recording"           json:"recording"`
	TLS               TLSConfig       `yaml:"tls"                 json:"tls"`
	HealthCheck         HealthCheck  `yaml:"health_check"         json:"health_check"`
	Endpoints           []Endpoint   `yaml:"endpoints"            json:"endpoints"`
	EndpointConfigPaths []string     `yaml:"-"                    json:"-"`
}

// TLSConfig holds TLS/HTTPS settings.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"   json:"enabled"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file"  json:"key_file"`
}

type LoggingConfig struct {
	Level     string `yaml:"level"      json:"level"`
	AccessLog bool   `yaml:"access_log" json:"access_log"`
	LogFormat string `yaml:"log_format" json:"log_format"`
	LogFile   string `yaml:"log_file"   json:"log_file"`
}

type ErrorHandling struct {
	ShowDetails          bool           `yaml:"show_details"           json:"show_details"`
	CustomErrorResponses map[int]string `yaml:"custom_error_responses" json:"custom_error_responses"`
}

// ==================== CORS Config ====================

type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"           json:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"   json:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"   json:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"   json:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"   json:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials" json:"allow_credentials"`
	MaxAgeSeconds    int      `yaml:"max_age_seconds"   json:"max_age_seconds"`
}

// ==================== Admin API Config ====================

type AdminAPIConfig struct {
	Enabled bool      `yaml:"enabled" json:"enabled"`
	Prefix  string    `yaml:"prefix"  json:"prefix"`
	Auth    AdminAuth `yaml:"auth"    json:"auth"`
}

type AdminAuth struct {
	Enabled       bool     `yaml:"enabled"        json:"enabled"`
	Username      string   `yaml:"username"       json:"username"`
	Password      string   `yaml:"password"       json:"-"` // never expose password in API
	AllowedIPs    []string `yaml:"allowed_ips"    json:"allowed_ips,omitempty"`
}

// ==================== Recording Config ====================

type RecordingConfig struct {
	Enabled      bool     `yaml:"enabled"        json:"enabled"`
	MaxEntries   int      `yaml:"max_entries"    json:"max_entries"`
	RecordBody   bool     `yaml:"record_body"    json:"record_body"`
	MaxBodyBytes int      `yaml:"max_body_bytes" json:"max_body_bytes"`
	ExcludePaths []string `yaml:"exclude_paths"  json:"exclude_paths"`
}

// ==================== Health Check ====================

type HealthCheck struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Path    string `yaml:"path"    json:"path"`
}

// ==================== Endpoint Config ====================

type Endpoint struct {
	Path        string         `yaml:"path"        json:"path"`
	Method      string         `yaml:"method"      json:"method"`
	Description string         `yaml:"description" json:"description"`
	Selectors   []Selector     `yaml:"selectors"   json:"selectors"`
	Rules       []Rule         `yaml:"rules"       json:"rules"`
	Default     ResponseConfig `yaml:"default"     json:"default"`
	// Proxy support
	Mode  string      `yaml:"mode,omitempty"  json:"mode,omitempty"`
	Proxy ProxyConfig `yaml:"proxy,omitempty" json:"proxy,omitempty"`
}

type Selector struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
	Key  string `yaml:"key"  json:"key"`
}

// ==================== Rule Config ====================

type Rule struct {
	ConditionLogic  string           `yaml:"condition_logic,omitempty"  json:"condition_logic,omitempty"`
	Conditions      []Condition      `yaml:"conditions"                 json:"conditions"`
	ConditionGroups []ConditionGroup `yaml:"condition_groups,omitempty" json:"condition_groups,omitempty"`
	ResponseConfig  `yaml:",inline"  json:",inline"`
}

type ConditionGroup struct {
	Logic      string      `yaml:"logic"      json:"logic"`
	Conditions []Condition `yaml:"conditions" json:"conditions"`
}

type Condition struct {
	Selector  string `yaml:"selector"   json:"selector"`
	MatchType string `yaml:"match_type" json:"match_type"`
	Value     string `yaml:"value"      json:"value"`
}

// ==================== Response Config ====================

type ResponseConfig struct {
	ResponseFile    string            `yaml:"response_file,omitempty"    json:"response_file,omitempty"`
	// InlineBody allows embedding the response body directly in the YAML config,
	// avoiding the need for a separate JSON/text file for simple responses.
	InlineBody      string            `yaml:"inline_body,omitempty"      json:"inline_body,omitempty"`
	StatusCode      int               `yaml:"status_code"                json:"status_code"`
	DelayMs         int               `yaml:"delay_ms,omitempty"         json:"delay_ms,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"          json:"headers,omitempty"`
	ContentType     string            `yaml:"content_type,omitempty"     json:"content_type,omitempty"`
	Template        *TemplateConfig   `yaml:"template,omitempty"         json:"template,omitempty"`
	RandomResponses *RandomResponses  `yaml:"random_responses,omitempty" json:"random_responses,omitempty"`
}

type TemplateConfig struct {
	Enabled   bool     `yaml:"enabled"          json:"enabled"`
	Engine    string   `yaml:"engine,omitempty" json:"engine,omitempty"`
	Variables []string `yaml:"variables"        json:"variables"`
}

type RandomResponses struct {
	Enabled bool             `yaml:"enabled" json:"enabled"`
	Files   []RandomResponse `yaml:"files"   json:"files"`
}

type RandomResponse struct {
	File       string `yaml:"file"              json:"file"`
	Weight     int    `yaml:"weight"            json:"weight"`
	StatusCode int    `yaml:"status_code"       json:"status_code"`
	DelayMs    int    `yaml:"delay_ms,omitempty" json:"delay_ms,omitempty"`
}

// ==================== Proxy Config ====================

type ProxyConfig struct {
	Target          string            `yaml:"target"                     json:"target"`
	StripPrefix     string            `yaml:"strip_prefix,omitempty"     json:"strip_prefix,omitempty"`
	TimeoutMs       int               `yaml:"timeout_ms,omitempty"       json:"timeout_ms,omitempty"`
	Record          bool              `yaml:"record,omitempty"           json:"record,omitempty"`
	RecordDir       string            `yaml:"record_dir,omitempty"       json:"record_dir,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"          json:"headers,omitempty"`
	FallbackOnError bool              `yaml:"fallback_on_error,omitempty" json:"fallback_on_error,omitempty"`
}

// ==================== Built-in Variables ====================

type BuiltinVariables struct {
	Timestamp time.Time `json:"timestamp"`
	UUID      string    `json:"uuid"`
	RequestID string    `json:"request_id"`
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
