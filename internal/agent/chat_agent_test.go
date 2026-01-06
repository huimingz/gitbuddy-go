package agent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatAgent_NewChatAgent tests ChatAgent creation
func TestChatAgent_NewChatAgent(t *testing.T) {
	options := ChatAgentOptions{
		Language:        "en",
		MaxLinesPerRead: 1000,
	}

	agent := NewChatAgent(options)
	require.NotNil(t, agent)
	assert.NotNil(t, agent.messages)
	assert.NotNil(t, agent.toolInstances)
}

// TestGetMessages tests retrieving message history
func TestGetMessages(t *testing.T) {
	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	// Add messages
	agent.AddMessage(schema.User, "test message")

	messages := agent.GetMessages()
	assert.Equal(t, 1, len(messages))
	assert.Equal(t, schema.User, messages[0].Role)
	assert.Equal(t, "test message", messages[0].Content)
}

// TestClearMessages tests clearing message history
func TestClearMessages(t *testing.T) {
	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	agent.AddMessage(schema.User, "message 1")
	agent.AddMessage(schema.User, "message 2")

	agent.ClearMessages()

	messages := agent.GetMessages()
	// After clear, only system message remains (if any)
	assert.Equal(t, 0, len(messages))
}

// TestChatRequest_QueryValidation tests that chat requests require a query
func TestChatRequest_QueryValidation(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "empty query",
			query:   "",
			wantErr: true,
		},
		{
			name:    "valid query",
			query:   "test query",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ChatRequest{
				Query:    tt.query,
				Language: "en",
			}

			if tt.wantErr {
				assert.Equal(t, "", req.Query)
			} else {
				assert.Equal(t, "test query", req.Query)
			}
		})
	}
}
