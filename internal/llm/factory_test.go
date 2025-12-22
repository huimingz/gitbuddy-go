package llm

import (
	"testing"

	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderFactory(t *testing.T) {
	factory := NewProviderFactory()
	assert.NotNil(t, factory)
}

func TestProviderFactory_Create_OpenAI(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4o",
		BaseURL:  "",
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "openai", provider.Name())
}

func TestProviderFactory_Create_Deepseek(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "deepseek",
		APIKey:   "sk-test",
		Model:    "deepseek-chat",
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "deepseek", provider.Name())
}

func TestProviderFactory_Create_Ollama(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "ollama",
		Model:    "qwen2.5:14b",
		BaseURL:  "http://localhost:11434/v1",
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "ollama", provider.Name())
}

func TestProviderFactory_Create_Gemini(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "gemini",
		APIKey:   "test-key",
		Model:    "gemini-1.5-pro",
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "gemini", provider.Name())
}

func TestProviderFactory_Create_Grok(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "grok",
		APIKey:   "xai-test",
		Model:    "grok-beta",
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "grok", provider.Name())
}

func TestProviderFactory_Create_UnsupportedProvider(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "unsupported",
		APIKey:   "test",
		Model:    "test-model",
	}

	provider, err := factory.Create(cfg)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestProviderFactory_CreateFromConfig(t *testing.T) {
	factory := NewProviderFactory()

	appCfg := &config.Config{
		DefaultModel: "deepseek",
		Models: map[string]config.ModelConfig{
			"deepseek": {
				Provider: "deepseek",
				APIKey:   "sk-test",
				Model:    "deepseek-chat",
			},
			"gpt4": {
				Provider: "openai",
				APIKey:   "sk-openai",
				Model:    "gpt-4o",
			},
		},
	}

	t.Run("create default model", func(t *testing.T) {
		provider, err := factory.CreateFromConfig(appCfg, "")
		require.NoError(t, err)
		assert.Equal(t, "deepseek", provider.Name())
	})

	t.Run("create specific model", func(t *testing.T) {
		provider, err := factory.CreateFromConfig(appCfg, "gpt4")
		require.NoError(t, err)
		assert.Equal(t, "openai", provider.Name())
	})

	t.Run("create non-existing model", func(t *testing.T) {
		_, err := factory.CreateFromConfig(appCfg, "nonexistent")
		assert.Error(t, err)
	})
}

func TestProvider_GetConfig(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "gpt-4o",
		BaseURL:  "https://custom.api.com/v1",
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)

	// Provider should expose its config
	providerCfg := provider.GetConfig()
	assert.Equal(t, "openai", providerCfg.Provider)
	assert.Equal(t, "gpt-4o", providerCfg.Model)
	assert.Equal(t, "https://custom.api.com/v1", providerCfg.BaseURL)
}

func TestDeepseekProvider_DefaultBaseURL(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "deepseek",
		APIKey:   "sk-test",
		Model:    "deepseek-chat",
		// BaseURL not set
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)

	providerCfg := provider.GetConfig()
	assert.Equal(t, "https://api.deepseek.com/v1", providerCfg.BaseURL)
}

func TestOllamaProvider_DefaultBaseURL(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "ollama",
		Model:    "qwen2.5:14b",
		// BaseURL not set
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)

	providerCfg := provider.GetConfig()
	assert.Equal(t, "http://localhost:11434/v1", providerCfg.BaseURL)
}

func TestGrokProvider_DefaultBaseURL(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ModelConfig{
		Provider: "grok",
		APIKey:   "xai-test",
		Model:    "grok-beta",
		// BaseURL not set
	}

	provider, err := factory.Create(cfg)
	require.NoError(t, err)

	providerCfg := provider.GetConfig()
	assert.Equal(t, "https://api.x.ai/v1", providerCfg.BaseURL)
}
