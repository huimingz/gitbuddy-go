package ui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultilinePrompt_NormalInput(t *testing.T) {
	input := strings.NewReader("This is a test issue\nwith multiple lines\nof description\n\x04") // \x04 is Ctrl+D
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
		Hint:   "You can enter multiple lines. Press Ctrl+D when finished.",
		Examples: []string{
			"Login fails with 500 error",
			"Database connection timeout in production",
		},
	}

	result, err := prompt.Show(input, output)
	require.NoError(t, err)

	expectedResult := "This is a test issue\nwith multiple lines\nof description"
	assert.Equal(t, expectedResult, result)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Please describe your issue:")
	assert.Contains(t, outputStr, "You can enter multiple lines")
	assert.Contains(t, outputStr, "Login fails with 500 error")
}

func TestMultilinePrompt_EmptyInput(t *testing.T) {
	input := strings.NewReader("\x04") // Just Ctrl+D
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
		Hint:   "You can enter multiple lines. Press Ctrl+D when finished.",
	}

	result, err := prompt.Show(input, output)
	assert.Equal(t, ErrEmptyInput, err)
	assert.Empty(t, result)
}

func TestMultilinePrompt_InterruptInput(t *testing.T) {
	// Create a context that can be cancelled to simulate Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to simulate interrupt

	input := strings.NewReader("some input\n")
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
		Hint:   "You can enter multiple lines. Press Ctrl+D when finished.",
	}

	result, err := prompt.ShowWithContext(ctx, input, output)
	assert.Equal(t, ErrInterrupted, err)
	assert.Empty(t, result)
}

func TestMultilinePrompt_CtrlD(t *testing.T) {
	input := strings.NewReader("Single line input\x04") // Ctrl+D after single line
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
		Hint:   "Press Ctrl+D when finished.",
	}

	result, err := prompt.Show(input, output)
	require.NoError(t, err)
	assert.Equal(t, "Single line input", result)
}

func TestMultilinePrompt_WithTimeout(t *testing.T) {
	// Test with pre-cancelled context to simulate timeout
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := strings.NewReader("some input\n")
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
	}

	result, err := prompt.ShowWithContext(ctx, input, output)
	assert.Equal(t, ErrInterrupted, err) // A cancelled context should return ErrInterrupted
	assert.Empty(t, result)
}

func TestMultilinePrompt_EmptyLinesHandling(t *testing.T) {
	input := strings.NewReader("First line\n\nSecond line\n\n\nThird line\n\x04")
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
		Hint:   "Press Ctrl+D when finished.",
	}

	result, err := prompt.Show(input, output)
	require.NoError(t, err)

	expected := "First line\n\nSecond line\n\n\nThird line"
	assert.Equal(t, expected, result)
}

func TestMultilinePrompt_NoExamples(t *testing.T) {
	input := strings.NewReader("Test input\x04")
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
		Hint:   "Press Ctrl+D when finished.",
		Examples: nil, // No examples
	}

	result, err := prompt.Show(input, output)
	require.NoError(t, err)
	assert.Equal(t, "Test input", result)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Please describe your issue:")
	assert.NotContains(t, outputStr, "Examples:")
}

func TestMultilinePrompt_EOF(t *testing.T) {
	input := strings.NewReader("") // EOF immediately
	output := &bytes.Buffer{}

	prompt := &MultilinePrompt{
		Prompt: "Please describe your issue:",
	}

	result, err := prompt.Show(input, output)
	assert.Equal(t, io.EOF, err)
	assert.Empty(t, result)
}