package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/huimingz/gitbuddy-go/internal/config"
)

const (
	// GrokDefaultBaseURL is the default API base URL for Grok
	GrokDefaultBaseURL = "https://api.x.ai/v1"
)

// GrokProvider implements Provider for xAI Grok
// Grok uses OpenAI-compatible API
type GrokProvider struct {
	cfg config.ModelConfig
}

// NewGrokProvider creates a new Grok provider
func NewGrokProvider(cfg config.ModelConfig) *GrokProvider {
	// Set default base URL if not specified
	if cfg.BaseURL == "" {
		cfg.BaseURL = GrokDefaultBaseURL
	}
	return &GrokProvider{cfg: cfg}
}

// Name returns the provider name
func (p *GrokProvider) Name() string {
	return "grok"
}

// GetConfig returns the model configuration
func (p *GrokProvider) GetConfig() config.ModelConfig {
	return p.cfg
}

// CreateChatModel creates an Eino ChatModel for Grok
func (p *GrokProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
	// Grok uses OpenAI-compatible API
	cfg := &openai.ChatModelConfig{
		APIKey:  p.cfg.APIKey,
		Model:   p.cfg.Model,
		BaseURL: p.cfg.BaseURL,
	}

	return openai.NewChatModel(ctx, cfg)
}
