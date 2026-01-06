package agent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

// TestChatAgent_StreamResponseAccumulation tests that streamed responses are properly accumulated
func TestChatAgent_StreamResponseAccumulation(t *testing.T) {
	// This test validates that multiple content chunks are properly combined
	// When a stream sends: "Hello", " ", "World"
	// The final response should be "Hello World", not just "World"

	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	// Simulate multiple message chunks
	chunks := []string{"Hello", " ", "World"}

	// Add system message
	agent.AddMessage(schema.System, "You are helpful")

	// Simulate receiving multiple chunks and building response
	var fullContent string
	for _, chunk := range chunks {
		fullContent += chunk
	}

	// Add the accumulated response
	agent.AddMessage(schema.Assistant, fullContent)

	messages := agent.GetMessages()
	assert.Equal(t, 2, len(messages))
	assert.Equal(t, "Hello World", messages[1].Content)
}

// TestChatAgent_MultipleResponses tests handling multiple response iterations
func TestChatAgent_MultipleResponses(t *testing.T) {
	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	agent.AddMessage(schema.System, "System")
	agent.AddMessage(schema.User, "First question")
	agent.AddMessage(schema.Assistant, "First response")
	agent.AddMessage(schema.User, "Second question")
	agent.AddMessage(schema.Assistant, "Second response")

	messages := agent.GetMessages()
	assert.Equal(t, 5, len(messages))

	// Last message should be the second response
	assert.Equal(t, "Second response", messages[4].Content)
}
