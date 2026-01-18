package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/huimingz/gitbuddy-go/internal/config"
)

// OpenAIProvider implements Provider for OpenAI API
type OpenAIProvider struct {
	cfg config.ModelConfig
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(cfg config.ModelConfig) *OpenAIProvider {
	return &OpenAIProvider{cfg: cfg}
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// GetConfig returns the model configuration
func (p *OpenAIProvider) GetConfig() config.ModelConfig {
	return p.cfg
}

// CreateChatModel creates an Eino ChatModel for OpenAI
func (p *OpenAIProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
	// Set default max tokens if not specified
	maxTokens := p.cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = config.DefaultMaxTokens
	}

	cfg := &openai.ChatModelConfig{
		APIKey:      p.cfg.APIKey,
		Model:       p.cfg.Model,
		BaseURL:     p.cfg.BaseURL,
		MaxTokens:   &maxTokens,
		Temperature: p.cfg.Temperature,
		TopP:        p.cfg.TopP,
	}

	return openai.NewChatModel(ctx, cfg)
}
