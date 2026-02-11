package ui

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fatih/color"
)

// FlushingWriter wraps an io.Writer and flushes after each write
// It solves the streaming output buffer issue by calling os.File.Sync() after every write
type FlushingWriter struct {
	writer io.Writer  // Underlying writer (could be color.Color)
	file   *os.File  // os.Stdout file reference for Sync()
}

// NewFlushingWriter creates a writer that flushes after each write
func NewFlushingWriter(w io.Writer) *FlushingWriter {
	fw := &FlushingWriter{writer: w}
	if f, ok := w.(*os.File); ok {
		fw.file = f
	}
	return fw
}

// Write implements io.Writer, flushing after each write
func (w *FlushingWriter) Write(p []byte) (n int, err error) {
	n, err = w.writer.Write(p)
	if w.file != nil {
		_ = w.file.Sync() // Cross-platform flush
	}
	return
}

// Flush implements the Flush method for compatibility
func (w *FlushingWriter) Flush() error {
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

// ExecutionStats holds statistics about the agent execution
type ExecutionStats struct {
	StartTime        time.Time
	EndTime          time.Time
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Duration returns the execution duration
func (s *ExecutionStats) Duration() time.Duration {
	return s.EndTime.Sub(s.StartTime)
}

// StreamPrinterOption is a functional option for StreamPrinter
type StreamPrinterOption func(*StreamPrinter)

// WithColor enables or disables color output
func WithColor(enabled bool) StreamPrinterOption {
	return func(p *StreamPrinter) {
		p.colorEnabled = enabled
	}
}

// WithVerbose enables or disables verbose mode
func WithVerbose(verbose bool) StreamPrinterOption {
	return func(p *StreamPrinter) {
		p.verbose = verbose
	}
}

// StreamPrinter handles streaming output to the terminal
type StreamPrinter struct {
	writer       io.Writer
	colorEnabled bool
	verbose      bool
}

// NewStreamPrinter creates a new StreamPrinter
func NewStreamPrinter(writer io.Writer, opts ...StreamPrinterOption) *StreamPrinter {
	var effectiveWriter io.Writer = writer

	// Check if writer supports Flush interface
	_, hasFlush := writer.(interface{ Flush() })

	// If no Flush support and is os.File, wrap with FlushingWriter
	if !hasFlush {
		if _, ok := writer.(*os.File); ok {
			effectiveWriter = NewFlushingWriter(writer)
		}
	}

	p := &StreamPrinter{
		writer:       effectiveWriter,
		colorEnabled: true,
		verbose:      false,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// PrintToken prints a token from the LLM stream
func (p *StreamPrinter) PrintToken(token string) error {
	_, err := fmt.Fprint(p.writer, token)
	return err
}

// PrintToolCall prints information about a tool being called
func (p *StreamPrinter) PrintToolCall(name string, args map[string]interface{}) error {
	if p.colorEnabled {
		cyan := color.New(color.FgCyan)
		_, err := cyan.Fprintf(p.writer, "\n🔧 Calling tool: %s\n", name)
		return err
	}
	_, err := fmt.Fprintf(p.writer, "\n🔧 Calling tool: %s\n", name)
	return err
}

// PrintToolResult prints the result of a tool call
func (p *StreamPrinter) PrintToolResult(name string, result string, err error) error {
	if err != nil {
		return p.PrintError(fmt.Sprintf("Tool %s failed: %v", name, err))
	}

	if p.verbose {
		if p.colorEnabled {
			green := color.New(color.FgGreen)
			_, e := green.Fprintf(p.writer, "✓ Tool %s completed\n", name)
			return e
		}
		_, e := fmt.Fprintf(p.writer, "✓ Tool %s completed\n", name)
		return e
	}

	// In non-verbose mode, just indicate completion
	if p.colorEnabled {
		green := color.New(color.FgGreen)
		_, e := green.Fprintf(p.writer, "✓ %s done\n", name)
		return e
	}
	_, e := fmt.Fprintf(p.writer, "✓ %s done\n", name)
	return e
}

// PrintThinking prints thinking/planning information
func (p *StreamPrinter) PrintThinking(message string) error {
	if p.colorEnabled {
		gray := color.New(color.FgHiBlack)
		_, err := gray.Fprintf(p.writer, "💭 %s\n", message)
		return err
	}
	_, err := fmt.Fprintf(p.writer, "💭 %s\n", message)
	return err
}

// PrintStep prints a step in the process
func (p *StreamPrinter) PrintStep(step int, message string) error {
	if p.colorEnabled {
		blue := color.New(color.FgBlue)
		_, err := blue.Fprintf(p.writer, "📋 Step %d: %s\n", step, message)
		return err
	}
	_, err := fmt.Fprintf(p.writer, "📋 Step %d: %s\n", step, message)
	return err
}

// PrintProgress prints a progress message
func (p *StreamPrinter) PrintProgress(message string) error {
	if p.colorEnabled {
		yellow := color.New(color.FgYellow)
		_, err := yellow.Fprintf(p.writer, "⏳ %s\n", message)
		return err
	}
	_, err := fmt.Fprintf(p.writer, "⏳ %s\n", message)
	return err
}

// PrintInfo prints an info message
func (p *StreamPrinter) PrintInfo(message string) error {
	if p.colorEnabled {
		cyan := color.New(color.FgCyan)
		_, err := cyan.Fprintf(p.writer, "ℹ️  %s\n", message)
		return err
	}
	_, err := fmt.Fprintf(p.writer, "ℹ️  %s\n", message)
	return err
}

// PrintSuccess prints a success message
func (p *StreamPrinter) PrintSuccess(message string) error {
	if p.colorEnabled {
		green := color.New(color.FgGreen)
		_, err := green.Fprintf(p.writer, "✅ %s\n", message)
		return err
	}
	_, err := fmt.Fprintf(p.writer, "✅ %s\n", message)
	return err
}

// Flusher is an interface for writers that support flushing
type Flusher interface {
	Flush() error
}

// PrintLLMContent prints content from LLM (for streaming responses)
// It flushes the output immediately if the writer supports it
func (p *StreamPrinter) PrintLLMContent(content string) error {
	var err error
	if p.colorEnabled {
		white := color.New(color.FgWhite)
		_, err = white.Fprint(p.writer, content)
	} else {
		_, err = fmt.Fprint(p.writer, content)
	}

	// Try to flush immediately for real-time output
	if f, ok := p.writer.(Flusher); ok {
		_ = f.Flush()
	}

	return err
}

// PrintError prints an error message
func (p *StreamPrinter) PrintError(message string) error {
	if p.colorEnabled {
		red := color.New(color.FgRed)
		_, err := red.Fprintf(p.writer, "❌ Error: %s\n", message)
		return err
	}
	_, err := fmt.Fprintf(p.writer, "❌ Error: %s\n", message)
	return err
}

// PrintStats prints execution statistics
func (p *StreamPrinter) PrintStats(stats *ExecutionStats) error {
	if stats == nil {
		return nil
	}

	duration := stats.Duration()
	durationStr := formatDuration(duration)

	if p.colorEnabled {
		dim := color.New(color.FgHiBlack)
		_, err := dim.Fprintf(p.writer, "\n📊 Stats: %d tokens (prompt: %d, completion: %d) | Time: %s\n",
			stats.TotalTokens, stats.PromptTokens, stats.CompletionTokens, durationStr)
		return err
	}

	_, err := fmt.Fprintf(p.writer, "\n📊 Stats: %d tokens (prompt: %d, completion: %d) | Time: %s\n",
		stats.TotalTokens, stats.PromptTokens, stats.CompletionTokens, durationStr)
	return err
}

// Newline prints a newline
func (p *StreamPrinter) Newline() error {
	_, err := fmt.Fprintln(p.writer)
	return err
}

// PrintToolArgChunk prints a chunk of tool call arguments in real-time
// This allows users to see the tool call parameters as they stream in
func (p *StreamPrinter) PrintToolArgChunk(chunk string) error {
	var err error
	if p.colorEnabled {
		dim := color.New(color.FgHiBlack)
		_, err = dim.Fprint(p.writer, chunk)
	} else {
		_, err = fmt.Fprint(p.writer, chunk)
	}

	// Try to flush immediately for real-time output
	if f, ok := p.writer.(Flusher); ok {
		_ = f.Flush()
	}

	return err
}

// PrintToolArgStart prints the start of tool arguments display
func (p *StreamPrinter) PrintToolArgStart() error {
	if p.colorEnabled {
		dim := color.New(color.FgHiBlack)
		_, err := dim.Fprint(p.writer, "   └─ ")
		return err
	}
	_, err := fmt.Fprint(p.writer, "   └─ ")
	return err
}

// PrintToolArgEnd prints the end of tool arguments display
func (p *StreamPrinter) PrintToolArgEnd() error {
	_, err := fmt.Fprintln(p.writer)
	return err
}

// Flush explicitly flushes the underlying writer
func (p *StreamPrinter) Flush() error {
	if f, ok := p.writer.(Flusher); ok {
		return f.Flush()
	}
	return nil
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
