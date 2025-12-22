package llm

import (
	"fmt"

	"github.com/huimingz/gitbuddy-go/internal/config"
)

// ProviderFactory creates LLM providers based on configuration
type ProviderFactory struct{}

// NewProviderFactory creates a new ProviderFactory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{}
}

// Create creates a Provider based on the model configuration
func (f *ProviderFactory) Create(cfg config.ModelConfig) (Provider, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAIProvider(cfg), nil
	case "deepseek":
		return NewDeepseekProvider(cfg), nil
	case "ollama":
		return NewOllamaProvider(cfg), nil
	case "gemini":
		return NewGeminiProvider(cfg), nil
	case "grok":
		return NewGrokProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

// CreateFromConfig creates a Provider from application config by model name
func (f *ProviderFactory) CreateFromConfig(appCfg *config.Config, modelName string) (Provider, error) {
	modelCfg, err := appCfg.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return f.Create(*modelCfg)
}
