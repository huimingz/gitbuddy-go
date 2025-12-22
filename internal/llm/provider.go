package llm

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/huimingz/gitbuddy-go/internal/config"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// GetConfig returns the model configuration
	GetConfig() config.ModelConfig

	// CreateChatModel creates an Eino ChatModel instance
	CreateChatModel(ctx context.Context) (model.ChatModel, error)
}
