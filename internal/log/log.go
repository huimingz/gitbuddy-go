package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fatih/color"
)

var (
	debugMode           = false
	output    io.Writer = os.Stderr
)

// SetDebugMode enables or disables debug mode
func SetDebugMode(enabled bool) {
	debugMode = enabled
}

// IsDebugMode returns whether debug mode is enabled
func IsDebugMode() bool {
	return debugMode
}

// SetOutput sets the output writer for log messages
func SetOutput(w io.Writer) {
	output = w
}

// Debug prints debug messages (only in debug mode)
func Debug(format string, args ...interface{}) {
	if debugMode {
		gray := color.New(color.FgHiBlack)
		gray.Fprintf(output, "[DEBUG] "+format+"\n", args...)
	}
}

// DebugConfig prints configuration details in debug mode
func DebugConfig(label string, config interface{}) {
	if debugMode {
		gray := color.New(color.FgHiBlack)
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			gray.Fprintf(output, "[DEBUG] %s: (failed to serialize: %v)\n", label, err)
			return
		}
		gray.Fprintf(output, "[DEBUG] %s:\n%s\n", label, string(data))
	}
}

// DebugRequest logs API request details in debug mode
func DebugRequest(method, url string, body interface{}) {
	if debugMode {
		cyan := color.New(color.FgCyan)
		cyan.Fprintf(output, "[DEBUG] API Request: %s %s\n", method, url)
		if body != nil {
			data, _ := json.MarshalIndent(body, "", "  ")
			fmt.Fprintf(output, "[DEBUG] Request Body:\n%s\n", string(data))
		}
	}
}

// DebugResponse logs API response details in debug mode
func DebugResponse(statusCode int, body interface{}) {
	if debugMode {
		green := color.New(color.FgGreen)
		green.Fprintf(output, "[DEBUG] API Response: %d\n", statusCode)
		if body != nil {
			data, _ := json.MarshalIndent(body, "", "  ")
			fmt.Fprintf(output, "[DEBUG] Response Body:\n%s\n", string(data))
		}
	}
}

// DebugToolCall logs tool call information in debug mode
func DebugToolCall(toolName string, params interface{}) {
	if debugMode {
		yellow := color.New(color.FgYellow)
		yellow.Fprintf(output, "[DEBUG] Tool Call: %s\n", toolName)
		if params != nil {
			data, _ := json.MarshalIndent(params, "", "  ")
			fmt.Fprintf(output, "[DEBUG] Parameters:\n%s\n", string(data))
		}
	}
}

// DebugToolResult logs tool result in debug mode
func DebugToolResult(toolName string, result string, err error) {
	if debugMode {
		if err != nil {
			red := color.New(color.FgRed)
			red.Fprintf(output, "[DEBUG] Tool %s Error: %v\n", toolName, err)
		} else {
			green := color.New(color.FgGreen)
			green.Fprintf(output, "[DEBUG] Tool %s Result: %s\n", toolName, truncate(result, 200))
		}
	}
}

// DebugTokenUsage logs token usage in debug mode
func DebugTokenUsage(promptTokens, completionTokens, totalTokens int) {
	if debugMode {
		magenta := color.New(color.FgMagenta)
		magenta.Fprintf(output, "[DEBUG] Token Usage: prompt=%d, completion=%d, total=%d\n",
			promptTokens, completionTokens, totalTokens)
	}
}

// DebugDuration logs execution duration in debug mode
func DebugDuration(operation string, duration time.Duration) {
	if debugMode {
		blue := color.New(color.FgBlue)
		blue.Fprintf(output, "[DEBUG] %s took %v\n", operation, duration)
	}
}

// Info prints informational messages
func Info(format string, args ...interface{}) {
	fmt.Fprintf(output, format+"\n", args...)
}

// Error prints error messages
func Error(format string, args ...interface{}) {
	red := color.New(color.FgRed)
	red.Fprintf(output, "Error: "+format+"\n", args...)
}

// Warn prints warning messages
func Warn(format string, args ...interface{}) {
	yellow := color.New(color.FgYellow)
	yellow.Fprintf(output, "Warning: "+format+"\n", args...)
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
