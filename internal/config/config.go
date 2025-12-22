package config

import (
	"fmt"
	"os"
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
