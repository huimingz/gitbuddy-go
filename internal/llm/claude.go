package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/huimingz/gitbuddy-go/internal/config"
)

const (
	// ClaudeDefaultBaseURL is the default API base URL for Claude
	ClaudeDefaultBaseURL = "https://api.anthropic.com"
)

// ClaudeProvider implements Provider for Anthropic Claude
type ClaudeProvider struct {
	cfg config.ModelConfig
}

// NewClaudeProvider creates a new Claude provider
func NewClaudeProvider(cfg config.ModelConfig) *ClaudeProvider {
	return &ClaudeProvider{cfg: cfg}
}

// Name returns the provider name
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// GetConfig returns the model configuration
func (p *ClaudeProvider) GetConfig() config.ModelConfig {
	return p.cfg
}

// CreateChatModel creates an Eino ChatModel for Claude
func (p *ClaudeProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
	// Prepare BaseURL pointer (nil means use eino-ext default)
	var baseURLPtr *string
	if p.cfg.BaseURL != "" {
		baseURLPtr = &p.cfg.BaseURL
	}

	// Set default max tokens if not specified
	maxTokens := p.cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = config.DefaultMaxTokens
	}

	// Build Claude config
	cfg := &claude.Config{
		APIKey:      p.cfg.APIKey,
		Model:       p.cfg.Model,
		BaseURL:     baseURLPtr,
		MaxTokens:   maxTokens,
		Temperature: p.cfg.Temperature, // nil will use Claude's default
		TopP:        p.cfg.TopP,        // nil will use Claude's default
		TopK:        p.cfg.TopK,        // nil will use Claude's default
	}

	return claude.NewChatModel(ctx, cfg)
}
