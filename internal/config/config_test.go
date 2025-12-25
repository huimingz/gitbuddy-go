package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ModelConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid openai config",
			config: ModelConfig{
				Provider: "openai",
				APIKey:   "sk-xxx",
				Model:    "gpt-4o",
			},
			wantErr: false,
		},
		{
			name: "valid deepseek config",
			config: ModelConfig{
				Provider: "deepseek",
				APIKey:   "sk-xxx",
				Model:    "deepseek-chat",
			},
			wantErr: false,
		},
		{
			name: "valid ollama config without api key",
			config: ModelConfig{
				Provider: "ollama",
				Model:    "qwen2.5:14b",
				BaseURL:  "http://localhost:11434/v1",
			},
			wantErr: false,
		},
		{
			name: "missing provider",
			config: ModelConfig{
				APIKey: "sk-xxx",
				Model:  "gpt-4o",
			},
			wantErr: true,
			errMsg:  "provider is required",
		},
		{
			name: "invalid provider",
			config: ModelConfig{
				Provider: "invalid",
				APIKey:   "sk-xxx",
				Model:    "gpt-4o",
			},
			wantErr: true,
			errMsg:  "unsupported provider",
		},
		{
			name: "missing model",
			config: ModelConfig{
				Provider: "openai",
				APIKey:   "sk-xxx",
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name: "missing api key for openai",
			config: ModelConfig{
				Provider: "openai",
				Model:    "gpt-4o",
			},
			wantErr: true,
			errMsg:  "api_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_GetModel(t *testing.T) {
	cfg := &Config{
		DefaultModel: "deepseek",
		Models: map[string]ModelConfig{
			"deepseek": {
				Provider: "deepseek",
				APIKey:   "sk-deepseek",
				Model:    "deepseek-chat",
			},
			"gpt4": {
				Provider: "openai",
				APIKey:   "sk-openai",
				Model:    "gpt-4o",
			},
		},
		Language: "en",
	}

	t.Run("get existing model", func(t *testing.T) {
		model, err := cfg.GetModel("gpt4")
		require.NoError(t, err)
		assert.Equal(t, "openai", model.Provider)
		assert.Equal(t, "gpt-4o", model.Model)
	})

	t.Run("get default model when empty name", func(t *testing.T) {
		model, err := cfg.GetModel("")
		require.NoError(t, err)
		assert.Equal(t, "deepseek", model.Provider)
		assert.Equal(t, "deepseek-chat", model.Model)
	})

	t.Run("get non-existing model", func(t *testing.T) {
		_, err := cfg.GetModel("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestConfig_GetModelWithEnvOverride(t *testing.T) {
	cfg := &Config{
		DefaultModel: "deepseek",
		Models: map[string]ModelConfig{
			"deepseek": {
				Provider: "deepseek",
				APIKey:   "sk-deepseek",
				Model:    "deepseek-chat",
			},
			"gpt4": {
				Provider: "openai",
				APIKey:   "sk-openai",
				Model:    "gpt-4o",
			},
		},
	}

	t.Run("env variable overrides default", func(t *testing.T) {
		os.Setenv("GITBUDDY_MODEL", "gpt4")
		defer os.Unsetenv("GITBUDDY_MODEL")

		model, err := cfg.GetModel("")
		require.NoError(t, err)
		assert.Equal(t, "openai", model.Provider)
	})

	t.Run("explicit name overrides env", func(t *testing.T) {
		os.Setenv("GITBUDDY_MODEL", "gpt4")
		defer os.Unsetenv("GITBUDDY_MODEL")

		model, err := cfg.GetModel("deepseek")
		require.NoError(t, err)
		assert.Equal(t, "deepseek", model.Provider)
	})
}

func TestConfig_ExpandEnvInAPIKey(t *testing.T) {
	os.Setenv("TEST_API_KEY", "my-secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	cfg := &Config{
		DefaultModel: "test",
		Models: map[string]ModelConfig{
			"test": {
				Provider: "openai",
				APIKey:   "${TEST_API_KEY}",
				Model:    "gpt-4o",
			},
		},
	}

	model, err := cfg.GetModel("test")
	require.NoError(t, err)
	assert.Equal(t, "my-secret-key", model.APIKey)
}

func TestConfig_GetLanguage(t *testing.T) {
	t.Run("returns configured language", func(t *testing.T) {
		cfg := &Config{Language: "zh"}
		assert.Equal(t, "zh", cfg.GetLanguage(""))
	})

	t.Run("override with parameter", func(t *testing.T) {
		cfg := &Config{Language: "zh"}
		assert.Equal(t, "ja", cfg.GetLanguage("ja"))
	})

	t.Run("env variable override", func(t *testing.T) {
		os.Setenv("GITBUDDY_LANG", "ko")
		defer os.Unsetenv("GITBUDDY_LANG")

		cfg := &Config{Language: "zh"}
		assert.Equal(t, "ko", cfg.GetLanguage(""))
	})

	t.Run("parameter overrides env", func(t *testing.T) {
		os.Setenv("GITBUDDY_LANG", "ko")
		defer os.Unsetenv("GITBUDDY_LANG")

		cfg := &Config{Language: "zh"}
		assert.Equal(t, "ja", cfg.GetLanguage("ja"))
	})

	t.Run("default to en when empty", func(t *testing.T) {
		cfg := &Config{}
		assert.Equal(t, "en", cfg.GetLanguage(""))
	})
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".gitbuddy.yaml")

	configContent := `
default_model: deepseek
models:
  deepseek:
    provider: deepseek
    api_key: sk-test
    model: deepseek-chat
  gpt4:
    provider: openai
    api_key: sk-openai
    model: gpt-4o
language: zh
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, "deepseek", cfg.DefaultModel)
	assert.Equal(t, "zh", cfg.Language)
	assert.Len(t, cfg.Models, 2)

	deepseek, ok := cfg.Models["deepseek"]
	assert.True(t, ok)
	assert.Equal(t, "deepseek", deepseek.Provider)
	assert.Equal(t, "deepseek-chat", deepseek.Model)
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/.gitbuddy.yaml")
	assert.Error(t, err)
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			DefaultModel: "deepseek",
			Models: map[string]ModelConfig{
				"deepseek": {
					Provider: "deepseek",
					APIKey:   "sk-test",
					Model:    "deepseek-chat",
				},
			},
			Language: "en",
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("no models configured", func(t *testing.T) {
		cfg := &Config{
			DefaultModel: "deepseek",
			Models:       map[string]ModelConfig{},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no models configured")
	})

	t.Run("default model not found", func(t *testing.T) {
		cfg := &Config{
			DefaultModel: "nonexistent",
			Models: map[string]ModelConfig{
				"deepseek": {
					Provider: "deepseek",
					APIKey:   "sk-test",
					Model:    "deepseek-chat",
				},
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "default model")
	})

	t.Run("invalid model config", func(t *testing.T) {
		cfg := &Config{
			DefaultModel: "deepseek",
			Models: map[string]ModelConfig{
				"deepseek": {
					Provider: "invalid-provider",
					APIKey:   "sk-test",
					Model:    "deepseek-chat",
				},
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
	})
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()
	assert.Contains(t, providers, "openai")
	assert.Contains(t, providers, "deepseek")
	assert.Contains(t, providers, "ollama")
	assert.Contains(t, providers, "gemini")
	assert.Contains(t, providers, "grok")
}

func TestConfig_GetPRTemplate(t *testing.T) {
	t.Run("returns empty when no template configured", func(t *testing.T) {
		cfg := &Config{}
		template, err := cfg.GetPRTemplate()
		assert.NoError(t, err)
		assert.Empty(t, template)
	})

	t.Run("returns empty when PRTemplate is nil", func(t *testing.T) {
		cfg := &Config{PRTemplate: nil}
		template, err := cfg.GetPRTemplate()
		assert.NoError(t, err)
		assert.Empty(t, template)
	})

	t.Run("returns inline template", func(t *testing.T) {
		cfg := &Config{
			PRTemplate: &PRTemplateConfig{
				Template: "## Summary\n\nDescribe changes here",
			},
		}
		template, err := cfg.GetPRTemplate()
		assert.NoError(t, err)
		assert.Equal(t, "## Summary\n\nDescribe changes here", template)
	})

	t.Run("inline template has priority over file", func(t *testing.T) {
		cfg := &Config{
			PRTemplate: &PRTemplateConfig{
				Template: "inline template",
				File:     "/some/file/path",
			},
		}
		template, err := cfg.GetPRTemplate()
		assert.NoError(t, err)
		assert.Equal(t, "inline template", template)
	})

	t.Run("loads template from file", func(t *testing.T) {
		// Create a temporary template file
		tmpDir := t.TempDir()
		templatePath := filepath.Join(tmpDir, "template.txt")
		templateContent := "## PR Template\n\nFrom file"
		err := os.WriteFile(templatePath, []byte(templateContent), 0644)
		require.NoError(t, err)

		cfg := &Config{
			PRTemplate: &PRTemplateConfig{
				File: templatePath,
			},
		}
		template, err := cfg.GetPRTemplate()
		assert.NoError(t, err)
		assert.Equal(t, templateContent, template)
	})

	t.Run("returns error when file not found", func(t *testing.T) {
		cfg := &Config{
			PRTemplate: &PRTemplateConfig{
				File: "/nonexistent/path/template.txt",
			},
		}
		_, err := cfg.GetPRTemplate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestLoadFromFile_WithPRTemplate(t *testing.T) {
	// Create a temporary config file with PR template
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".gitbuddy.yaml")

	configContent := `
default_model: deepseek
models:
  deepseek:
    provider: deepseek
    api_key: sk-test
    model: deepseek-chat
language: zh
pr_template:
  template: |
    ## Summary
    Brief overview
    
    ## Changes
    - Change 1
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)

	assert.NotNil(t, cfg.PRTemplate)
	template, err := cfg.GetPRTemplate()
	require.NoError(t, err)
	assert.Contains(t, template, "## Summary")
	assert.Contains(t, template, "## Changes")
}
