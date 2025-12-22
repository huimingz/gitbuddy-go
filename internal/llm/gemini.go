package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"google.golang.org/genai"
)

// GeminiProvider implements Provider for Google Gemini
type GeminiProvider struct {
	cfg config.ModelConfig
}

// NewGeminiProvider creates a new Gemini provider
func NewGeminiProvider(cfg config.ModelConfig) *GeminiProvider {
	return &GeminiProvider{cfg: cfg}
}

// Name returns the provider name
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// GetConfig returns the model configuration
func (p *GeminiProvider) GetConfig() config.ModelConfig {
	return p.cfg
}

// CreateChatModel creates an Eino ChatModel for Gemini
func (p *GeminiProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
	// Create Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: p.cfg.APIKey,
	})
	if err != nil {
		return nil, err
	}

	cfg := &gemini.Config{
		Client: client,
		Model:  p.cfg.Model,
	}

	return gemini.NewChatModel(ctx, cfg)
}
