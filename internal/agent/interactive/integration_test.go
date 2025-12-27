package interactive

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveSession_NaturalConversationIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create interactive session
	session := NewInteractiveSession(tmpDir)

	// Simulate natural conversation: ask about editing a file
	input := strings.NewReader("edit test.go to replace line 6 with better message\nexit\n")
	output := &strings.Builder{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Start(ctx, input, output)
	require.NoError(t, err)

	// Verify the question was received and processed as a natural conversation
	outputStr := output.String()
	assert.Contains(t, outputStr, "Question received")
	assert.Contains(t, outputStr, "edit test.go to replace line 6")
}

func TestInteractiveSession_ModifyRequestIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create interactive session without LLM provider
	session := NewInteractiveSession(tmpDir)
	session.SetReportContent("## Debug Report\nThis is a test report")

	// Simulate modify request without LLM provider
	input := strings.NewReader("modify add more performance analysis\nexit\n")
	output := &strings.Builder{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Start(ctx, input, output)
	require.NoError(t, err)

	// Verify the modification request was received and fallback message shown
	outputStr := output.String()
	assert.Contains(t, outputStr, "Report modification request received")
	assert.Contains(t, outputStr, "add more performance analysis")
	assert.Contains(t, outputStr, "LLM provider not configured")
}

func TestInteractiveSession_ModifyWithoutReportContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create interactive session with mock LLM provider but no report content
	session := NewInteractiveSession(tmpDir)
	// Note: Not setting report content to test error handling

	// Test modify without report content
	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "modify add details", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Report modification request received")
	assert.Contains(t, outputStr, "add details")
}

func TestInteractiveSession_MultilineModifyIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create interactive session
	session := NewInteractiveSession(tmpDir)
	session.SetReportContent("## Debug Report\nThis is a test report")

	// Simulate multiline modify command
	input := strings.NewReader("modify\nThis is a multiline\nmodification request\nwith several lines\n.\nexit\n")
	output := &strings.Builder{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Start(ctx, input, output)
	require.NoError(t, err)

	// Verify multiline input was processed
	outputStr := output.String()
	assert.Contains(t, outputStr, "Enter your modification")
	assert.Contains(t, outputStr, "multiline modification request")
}

func TestInteractiveSession_ConversationFlow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create interactive session
	session := NewInteractiveSession(tmpDir)

	// Simulate a complete conversation flow: help, question, modify, exit
	input := strings.NewReader("help\nWhat caused this error?\nmodify add error details\nexit\n")
	output := &strings.Builder{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Start(ctx, input, output)
	require.NoError(t, err)

	// Verify all interactions were processed
	outputStr := output.String()
	assert.Contains(t, outputStr, "Interactive Debug Session Help")
	assert.Contains(t, outputStr, "Question received")
	assert.Contains(t, outputStr, "What caused this error?")
	assert.Contains(t, outputStr, "Report modification request")
	assert.Contains(t, outputStr, "add error details")
	assert.Contains(t, outputStr, "Goodbye!")
}

func TestInteractiveSession_LLMProviderHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create interactive session without LLM provider
	session := NewInteractiveSession(tmpDir)
	session.SetReportContent("## Debug Report\nTest error analysis")

	// Test question without LLM provider
	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "What is the issue?", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Question received")
	assert.Contains(t, outputStr, "LLM provider not configured")
	assert.Contains(t, outputStr, "Unable to provide intelligent responses")

	// Test that SetLLMProvider works
	session.SetLLMProvider(nil) // Still nil, but method should work
	assert.NotPanics(t, func() {
		session.SetLLMProvider(nil)
	})
}