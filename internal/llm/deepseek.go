package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/huimingz/gitbuddy-go/internal/config"
)

const (
	// DeepseekDefaultBaseURL is the default API base URL for Deepseek
	DeepseekDefaultBaseURL = "https://api.deepseek.com/v1"
)

// DeepseekProvider implements Provider for Deepseek API
// Deepseek uses OpenAI-compatible API
type DeepseekProvider struct {
	cfg config.ModelConfig
}

// NewDeepseekProvider creates a new Deepseek provider
func NewDeepseekProvider(cfg config.ModelConfig) *DeepseekProvider {
	// Set default base URL if not specified
	if cfg.BaseURL == "" {
		cfg.BaseURL = DeepseekDefaultBaseURL
	}
	return &DeepseekProvider{cfg: cfg}
}

// Name returns the provider name
func (p *DeepseekProvider) Name() string {
	return "deepseek"
}

// GetConfig returns the model configuration
func (p *DeepseekProvider) GetConfig() config.ModelConfig {
	return p.cfg
}

// CreateChatModel creates an Eino ChatModel for Deepseek
func (p *DeepseekProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
	// Deepseek uses OpenAI-compatible API
	cfg := &openai.ChatModelConfig{
		APIKey:  p.cfg.APIKey,
		Model:   p.cfg.Model,
		BaseURL: p.cfg.BaseURL,
	}

	return openai.NewChatModel(ctx, cfg)
}
