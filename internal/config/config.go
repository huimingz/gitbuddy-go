package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Supported providers
var supportedProviders = map[string]bool{
	"openai":   true,
	"deepseek": true,
	"ollama":   true,
	"gemini":   true,
	"grok":     true,
}

// SupportedProviders returns a list of supported providers
func SupportedProviders() []string {
	providers := make([]string, 0, len(supportedProviders))
	for p := range supportedProviders {
		providers = append(providers, p)
	}
	return providers
}

// Config represents the application configuration
type Config struct {
	DefaultModel string                 `yaml:"default_model" mapstructure:"default_model"`
	Models       map[string]ModelConfig `yaml:"models" mapstructure:"models"`
	Language     string                 `yaml:"language" mapstructure:"language"`
	PRTemplate   *PRTemplateConfig      `yaml:"pr_template" mapstructure:"pr_template"`
	Review       *ReviewConfig          `yaml:"review" mapstructure:"review"`
	Debug        *DebugConfig           `yaml:"debug" mapstructure:"debug"`
	Retry        *RetryConfig           `yaml:"retry" mapstructure:"retry"`
	Session      *SessionConfig         `yaml:"session" mapstructure:"session"`
}

// ReviewConfig represents the review command configuration
type ReviewConfig struct {
	MaxLinesPerRead int `yaml:"max_lines_per_read" mapstructure:"max_lines_per_read"`
	GrepMaxFileSize int `yaml:"grep_max_file_size" mapstructure:"grep_max_file_size"` // in MB
	GrepTimeout     int `yaml:"grep_timeout" mapstructure:"grep_timeout"`             // in seconds
	GrepMaxResults  int `yaml:"grep_max_results" mapstructure:"grep_max_results"`
}

// DefaultReviewConfig returns the default review configuration
func DefaultReviewConfig() *ReviewConfig {
	return &ReviewConfig{
		MaxLinesPerRead: 1000,
		GrepMaxFileSize: 10,  // 10 MB
		GrepTimeout:     10,  // 10 seconds
		GrepMaxResults:  100, // 100 results
	}
}

// DebugConfig represents the debug command configuration
type DebugConfig struct {
	MaxLinesPerRead        int    `yaml:"max_lines_per_read" mapstructure:"max_lines_per_read"`
	IssuesDir              string `yaml:"issues_dir" mapstructure:"issues_dir"`
	MaxIterations          int    `yaml:"max_iterations" mapstructure:"max_iterations"`
	EnableCompression      bool   `yaml:"enable_compression" mapstructure:"enable_compression"`
	CompressionThreshold   int    `yaml:"compression_threshold" mapstructure:"compression_threshold"`       // Number of messages before compression
	CompressionKeepRecent  int    `yaml:"compression_keep_recent" mapstructure:"compression_keep_recent"`   // Number of recent messages to keep
	ShowCompressionSummary bool   `yaml:"show_compression_summary" mapstructure:"show_compression_summary"` // Show compression summary to user
	GrepMaxFileSize        int    `yaml:"grep_max_file_size" mapstructure:"grep_max_file_size"`             // in MB
	GrepTimeout            int    `yaml:"grep_timeout" mapstructure:"grep_timeout"`                         // in seconds
	GrepMaxResults         int    `yaml:"grep_max_results" mapstructure:"grep_max_results"`
}

// DefaultDebugConfig returns the default debug configuration
func DefaultDebugConfig() *DebugConfig {
	return &DebugConfig{
		MaxLinesPerRead:        1000,
		IssuesDir:              "./issues",
		MaxIterations:          50,    // 50 iterations (increased from 30)
		EnableCompression:      true,  // Enable compression by default
		CompressionThreshold:   20,    // Compress when > 20 messages
		CompressionKeepRecent:  10,    // Keep last 10 messages
		ShowCompressionSummary: false, // Don't show summary by default
		GrepMaxFileSize:        10,    // 10 MB
		GrepTimeout:            10,    // 10 seconds
		GrepMaxResults:         100,   // 100 results
	}
}

// PRTemplateConfig represents the PR template configuration
type PRTemplateConfig struct {
	Template string `yaml:"template" mapstructure:"template"` // Inline template content
	File     string `yaml:"file" mapstructure:"file"`         // Path to template file
}

// RetryConfig represents the retry configuration
type RetryConfig struct {
	Enabled     bool    `yaml:"enabled" mapstructure:"enabled"`
	MaxAttempts int     `yaml:"max_attempts" mapstructure:"max_attempts"`
	BackoffBase float64 `yaml:"backoff_base" mapstructure:"backoff_base"` // in seconds
	BackoffMax  float64 `yaml:"backoff_max" mapstructure:"backoff_max"`   // in seconds
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		Enabled:     true,
		MaxAttempts: 3,
		BackoffBase: 1.0,
		BackoffMax:  8.0,
	}
}

// Validate validates the retry configuration
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts < 0 {
		return fmt.Errorf("max_attempts must be non-negative")
	}
	if r.BackoffBase < 0 {
		return fmt.Errorf("backoff_base must be non-negative")
	}
	if r.BackoffMax < r.BackoffBase {
		return fmt.Errorf("backoff_max must be greater than or equal to backoff_base")
	}
	return nil
}

// SessionConfig represents the session configuration
type SessionConfig struct {
	SaveDir     string `yaml:"save_dir" mapstructure:"save_dir"`
	AutoSave    bool   `yaml:"auto_save" mapstructure:"auto_save"`
	MaxSessions int    `yaml:"max_sessions" mapstructure:"max_sessions"`
}

// DefaultSessionConfig returns the default session configuration
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		SaveDir:     "./.gitbuddy/sessions",
		AutoSave:    true,
		MaxSessions: 10,
	}
}

// Validate validates the session configuration
func (s *SessionConfig) Validate() error {
	if s.SaveDir == "" {
		return fmt.Errorf("save_dir is required")
	}
	if s.MaxSessions < 0 {
		return fmt.Errorf("max_sessions must be non-negative")
	}
	return nil
}

// ModelConfig represents a single model configuration
type ModelConfig struct {
	Provider string `yaml:"provider" mapstructure:"provider"`
	APIKey   string `yaml:"api_key" mapstructure:"api_key"`
	Model    string `yaml:"model" mapstructure:"model"`
	BaseURL  string `yaml:"base_url" mapstructure:"base_url"`
}

// Validate validates the model configuration
func (m *ModelConfig) Validate() error {
	if m.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if !supportedProviders[m.Provider] {
		return fmt.Errorf("unsupported provider: %s", m.Provider)
	}
	if m.Model == "" {
		return fmt.Errorf("model is required")
	}
	// API key is required for all providers except ollama
	if m.Provider != "ollama" && m.APIKey == "" {
		return fmt.Errorf("api_key is required for provider %s", m.Provider)
	}
	return nil
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if len(c.Models) == 0 {
		return fmt.Errorf("no models configured")
	}

	// Validate default model exists
	if c.DefaultModel != "" {
		if _, ok := c.Models[c.DefaultModel]; !ok {
			return fmt.Errorf("default model '%s' not found in models configuration", c.DefaultModel)
		}
	}

	// Validate each model
	for name, model := range c.Models {
		if err := model.Validate(); err != nil {
			return fmt.Errorf("invalid model '%s': %w", name, err)
		}
	}

	// Validate retry config if present
	if c.Retry != nil {
		if err := c.Retry.Validate(); err != nil {
			return fmt.Errorf("invalid retry configuration: %w", err)
		}
	}

	// Validate session config if present
	if c.Session != nil {
		if err := c.Session.Validate(); err != nil {
			return fmt.Errorf("invalid session configuration: %w", err)
		}
	}

	return nil
}

// GetModel returns the model configuration by name
// Priority: parameter > env variable (GITBUDDY_MODEL) > default_model
func (c *Config) GetModel(modelName string) (*ModelConfig, error) {
	// If modelName is empty, check env variable
	if modelName == "" {
		modelName = os.Getenv("GITBUDDY_MODEL")
	}

	// If still empty, use default model
	if modelName == "" {
		modelName = c.DefaultModel
	}

	// If still empty, return error
	if modelName == "" {
		return nil, fmt.Errorf("no model specified and no default model configured")
	}

	model, ok := c.Models[modelName]
	if !ok {
		return nil, fmt.Errorf("model '%s' not found in configuration", modelName)
	}

	// Expand environment variables in API key
	model.APIKey = expandEnv(model.APIKey)

	return &model, nil
}

// GetLanguage returns the language to use
// Priority: parameter > env variable (GITBUDDY_LANG) > config file > default (en)
func (c *Config) GetLanguage(langParam string) string {
	// Parameter has highest priority
	if langParam != "" {
		return langParam
	}

	// Check env variable
	if envLang := os.Getenv("GITBUDDY_LANG"); envLang != "" {
		return envLang
	}

	// Use config file value
	if c.Language != "" {
		return c.Language
	}

	// Default to English
	return "en"
}

// GetReviewConfig returns the review configuration with defaults applied
func (c *Config) GetReviewConfig() *ReviewConfig {
	if c.Review == nil {
		return DefaultReviewConfig()
	}
	// Apply defaults for unset values
	defaults := DefaultReviewConfig()
	if c.Review.MaxLinesPerRead <= 0 {
		c.Review.MaxLinesPerRead = defaults.MaxLinesPerRead
	}
	if c.Review.GrepMaxFileSize <= 0 {
		c.Review.GrepMaxFileSize = defaults.GrepMaxFileSize
	}
	if c.Review.GrepTimeout <= 0 {
		c.Review.GrepTimeout = defaults.GrepTimeout
	}
	if c.Review.GrepMaxResults <= 0 {
		c.Review.GrepMaxResults = defaults.GrepMaxResults
	}
	return c.Review
}

// GetDebugConfig returns the debug configuration with defaults applied
func (c *Config) GetDebugConfig() *DebugConfig {
	if c.Debug == nil {
		return DefaultDebugConfig()
	}
	// Apply defaults for unset values
	defaults := DefaultDebugConfig()
	if c.Debug.MaxLinesPerRead <= 0 {
		c.Debug.MaxLinesPerRead = defaults.MaxLinesPerRead
	}
	if c.Debug.IssuesDir == "" {
		c.Debug.IssuesDir = defaults.IssuesDir
	}
	if c.Debug.MaxIterations <= 0 {
		c.Debug.MaxIterations = defaults.MaxIterations
	}
	if c.Debug.CompressionThreshold <= 0 {
		c.Debug.CompressionThreshold = defaults.CompressionThreshold
	}
	if c.Debug.CompressionKeepRecent <= 0 {
		c.Debug.CompressionKeepRecent = defaults.CompressionKeepRecent
	}
	if c.Debug.GrepMaxFileSize <= 0 {
		c.Debug.GrepMaxFileSize = defaults.GrepMaxFileSize
	}
	if c.Debug.GrepTimeout <= 0 {
		c.Debug.GrepTimeout = defaults.GrepTimeout
	}
	if c.Debug.GrepMaxResults <= 0 {
		c.Debug.GrepMaxResults = defaults.GrepMaxResults
	}
	return c.Debug
}

// GetRetryConfig returns the retry configuration with defaults applied
func (c *Config) GetRetryConfig() *RetryConfig {
	if c.Retry == nil {
		return DefaultRetryConfig()
	}
	// Apply defaults for unset values
	defaults := DefaultRetryConfig()
	if c.Retry.MaxAttempts < 0 {
		c.Retry.MaxAttempts = defaults.MaxAttempts
	}
	if c.Retry.BackoffBase < 0 {
		c.Retry.BackoffBase = defaults.BackoffBase
	}
	if c.Retry.BackoffMax < 0 {
		c.Retry.BackoffMax = defaults.BackoffMax
	}
	return c.Retry
}

// GetSessionConfig returns the session configuration with defaults applied
func (c *Config) GetSessionConfig() *SessionConfig {
	if c.Session == nil {
		return DefaultSessionConfig()
	}
	// Apply defaults for unset values
	defaults := DefaultSessionConfig()
	if c.Session.SaveDir == "" {
		c.Session.SaveDir = defaults.SaveDir
	}
	if c.Session.MaxSessions < 0 {
		c.Session.MaxSessions = defaults.MaxSessions
	}
	return c.Session
}

// GetPRTemplate returns the PR template content
// Priority: inline template > file template > empty string (use default)
// Returns the template content and any error encountered
func (c *Config) GetPRTemplate() (string, error) {
	if c.PRTemplate == nil {
		return "", nil
	}

	// Inline template has priority
	if c.PRTemplate.Template != "" {
		return c.PRTemplate.Template, nil
	}

	// Load from file if specified
	if c.PRTemplate.File != "" {
		// Expand ~ to home directory
		filePath := c.PRTemplate.File
		if strings.HasPrefix(filePath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			filePath = filepath.Join(homeDir, filePath[2:])
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("PR template file not found: %s", filePath)
			}
			return "", fmt.Errorf("failed to read PR template file: %w", err)
		}
		return string(content), nil
	}

	return "", nil
}

// expandEnv expands environment variables in the format ${VAR} or $VAR
func expandEnv(s string) string {
	// Handle ${VAR} format
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		envName := s[2 : len(s)-1]
		return os.Getenv(envName)
	}
	// Handle $VAR format
	if strings.HasPrefix(s, "$") {
		envName := s[1:]
		return os.Getenv(envName)
	}
	return s
}

// LoadFromFile loads configuration from a file
func LoadFromFile(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// Load loads configuration with the following priority:
// 1. Custom path if provided
// 2. Current directory .gitbuddy.yaml
// 3. Home directory ~/.gitbuddy.yaml
func Load(customPath string) (*Config, error) {
	// If custom path is provided, use it exclusively
	if customPath != "" {
		return LoadFromFile(customPath)
	}

	// Try current directory first
	if cfg, err := LoadFromFile(".gitbuddy.yaml"); err == nil {
		return cfg, nil
	}

	// Try home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	homeCfgPath := fmt.Sprintf("%s/.gitbuddy.yaml", homeDir)
	if cfg, err := LoadFromFile(homeCfgPath); err == nil {
		return cfg, nil
	}

	return nil, fmt.Errorf("no configuration file found. Run 'gitbuddy init' to create one")
}
