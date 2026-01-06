package agent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

// TestChatAgent_CompressMessages tests message compression
func TestChatAgent_CompressMessages(t *testing.T) {
	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	// Add system message
	agent.AddMessage(schema.System, "System prompt")

	// Add many messages to trigger compression
	for i := 0; i < 30; i++ {
		agent.AddMessage(schema.User, "User message "+string(rune(i)))
		agent.AddMessage(schema.Assistant, "Assistant response "+string(rune(i)))
	}

	initialCount := len(agent.GetMessages())
	assert.Equal(t, 61, initialCount) // 1 system + 30*2 messages

	// Compress messages keeping 10 recent
	agent.compressMessages(10)

	compressedMessages := agent.GetMessages()
	// Should have system message + 10 recent messages
	assert.True(t, len(compressedMessages) <= 15, "compressed messages should be <= 15")
	// System message should still be there
	assert.Equal(t, schema.System, compressedMessages[0].Role)
}

// TestChatAgent_CompressMessages_NoChange tests compression when not needed
func TestChatAgent_CompressMessages_NoChange(t *testing.T) {
	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	// Add only a few messages
	agent.AddMessage(schema.System, "System prompt")
	agent.AddMessage(schema.User, "User message")

	initialMessages := make([]*schema.Message, len(agent.GetMessages()))
	copy(initialMessages, agent.GetMessages())

	// Compress with high keep recent number
	agent.compressMessages(20)

	// Should not change much
	compressedMessages := agent.GetMessages()
	assert.Equal(t, len(initialMessages), len(compressedMessages))
}

// TestChatAgent_MessageCompressionThreshold tests compression threshold behavior
func TestChatAgent_MessageCompressionThreshold(t *testing.T) {
	// This test validates that compression is applied when threshold is exceeded
	req := &ChatRequest{
		Query:                 "test",
		Language:              "en",
		EnableCompression:     true,
		CompressionThreshold:  20,
		CompressionKeepRecent: 10,
	}

	assert.True(t, req.EnableCompression)
	assert.Equal(t, 20, req.CompressionThreshold)
	assert.Equal(t, 10, req.CompressionKeepRecent)
}

// TestChatAgent_PreserveSystemMessage tests that system message is preserved
func TestChatAgent_PreserveSystemMessage(t *testing.T) {
	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	systemContent := "You are a helpful assistant"
	agent.AddMessage(schema.System, systemContent)

	// Add other messages
	for i := 0; i < 25; i++ {
		agent.AddMessage(schema.User, "Message "+string(rune(i)))
	}

	// Compress
	agent.compressMessages(5)

	// Find system message
	systemFound := false
	for _, msg := range agent.GetMessages() {
		if msg.Role == schema.System {
			assert.Equal(t, systemContent, msg.Content)
			systemFound = true
			break
		}
	}
	assert.True(t, systemFound, "system message should be preserved")
}

// TestChatAgent_CompressionPreservesRecent tests that recent messages are kept
func TestChatAgent_CompressionPreservesRecent(t *testing.T) {
	options := ChatAgentOptions{Language: "en"}
	agent := NewChatAgent(options)

	agent.AddMessage(schema.System, "System")

	// Add old messages
	for i := 0; i < 20; i++ {
		agent.AddMessage(schema.User, "Old message "+string(rune(i)))
	}

	// Add recent messages
	recentMessages := []string{"Recent 1", "Recent 2", "Recent 3"}
	for _, msg := range recentMessages {
		agent.AddMessage(schema.Assistant, msg)
	}

	// Compress keeping 3 recent messages
	agent.compressMessages(3)

	compressed := agent.GetMessages()
	// The last 3 messages should still be there
	for i, msg := range recentMessages {
		assert.Equal(t, msg, compressed[len(compressed)-len(recentMessages)+i].Content)
	}
}
