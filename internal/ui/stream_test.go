package ui

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStreamPrinter(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)
	require.NotNil(t, printer)
}

func TestStreamPrinter_PrintToken(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)

	err := printer.PrintToken("Hello ")
	require.NoError(t, err)
	assert.Equal(t, "Hello ", buf.String())

	err = printer.PrintToken("World!")
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", buf.String())
}

func TestStreamPrinter_PrintToolCall(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)

	err := printer.PrintToolCall("git_diff_cached", map[string]interface{}{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "git_diff_cached")
}

func TestStreamPrinter_PrintToolResult(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)

	err := printer.PrintToolResult("git_diff_cached", "diff output here", nil)
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "git_diff_cached")
}

func TestStreamPrinter_PrintThinking(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)

	err := printer.PrintThinking("Analyzing staged changes...")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Analyzing")
}

func TestStreamPrinter_PrintError(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)

	err := printer.PrintError("something went wrong")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "wrong")
}

func TestExecutionStats(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 1, 12, 0, 2, 0, time.UTC)

	stats := &ExecutionStats{
		StartTime:        startTime,
		EndTime:          endTime,
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	assert.Equal(t, 2*time.Second, stats.Duration())
	assert.Equal(t, 150, stats.TotalTokens)
}

func TestStreamPrinter_PrintStats(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)

	stats := &ExecutionStats{
		StartTime:        time.Now(),
		EndTime:          time.Now().Add(1500 * time.Millisecond),
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	err := printer.PrintStats(stats)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "150") // Total tokens
}

func TestStreamPrinterOptions(t *testing.T) {
	var buf bytes.Buffer

	t.Run("with color disabled", func(t *testing.T) {
		printer := NewStreamPrinter(&buf, WithColor(false))
		require.NotNil(t, printer)
		assert.False(t, printer.colorEnabled)
	})

	t.Run("with verbose mode", func(t *testing.T) {
		printer := NewStreamPrinter(&buf, WithVerbose(true))
		require.NotNil(t, printer)
		assert.True(t, printer.verbose)
	})
}

func TestStreamPrinter_Newline(t *testing.T) {
	var buf bytes.Buffer
	printer := NewStreamPrinter(&buf)

	err := printer.Newline()
	require.NoError(t, err)
	assert.Equal(t, "\n", buf.String())
}

func TestStreamPrinter_CustomWriter(t *testing.T) {
	// Test with a custom io.Writer
	pr, pw := io.Pipe()
	defer pr.Close()

	printer := NewStreamPrinter(pw)

	go func() {
		defer pw.Close()
		_ = printer.PrintToken("test")
	}()

	buf := make([]byte, 100)
	n, _ := pr.Read(buf)
	assert.Contains(t, string(buf[:n]), "test")
}
