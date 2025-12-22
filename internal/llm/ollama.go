package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/huimingz/gitbuddy-go/internal/config"
)

const (
	// OllamaDefaultBaseURL is the default API base URL for Ollama
	OllamaDefaultBaseURL = "http://localhost:11434/v1"
)

// OllamaProvider implements Provider for local Ollama
// Ollama uses OpenAI-compatible API
type OllamaProvider struct {
	cfg config.ModelConfig
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(cfg config.ModelConfig) *OllamaProvider {
	// Set default base URL if not specified
	if cfg.BaseURL == "" {
		cfg.BaseURL = OllamaDefaultBaseURL
	}

	// Ollama doesn't require API key, set a placeholder
	if cfg.APIKey == "" {
		cfg.APIKey = "ollama"
	}
	return &OllamaProvider{cfg: cfg}
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// GetConfig returns the model configuration
func (p *OllamaProvider) GetConfig() config.ModelConfig {
	return p.cfg
}

// CreateChatModel creates an Eino ChatModel for Ollama
func (p *OllamaProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
	// Ollama uses OpenAI-compatible API
	cfg := &openai.ChatModelConfig{
		APIKey:  p.cfg.APIKey,
		Model:   p.cfg.Model,
		BaseURL: p.cfg.BaseURL,
	}

	return openai.NewChatModel(ctx, cfg)
}
