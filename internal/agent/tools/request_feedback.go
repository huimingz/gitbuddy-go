package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// RequestFeedbackParams contains parameters for requesting user feedback
type RequestFeedbackParams struct {
	Title   string   `json:"title"`             // Question title
	Content string   `json:"content"`           // Question content (description, context, or details)
	Prompt  string   `json:"prompt"`            // Prompt text (guide user how to answer)
	Options []string `json:"options,omitempty"` // Options list (if provided, it's multiple choice; otherwise open-ended)
}

// RequestFeedbackTool is a tool for requesting interactive feedback from the user
// This tool allows the LLM to pause analysis and ask the user for direction
type RequestFeedbackTool struct {
	input  io.Reader
	output io.Writer
}

// NewRequestFeedbackTool creates a new RequestFeedbackTool
func NewRequestFeedbackTool(input io.Reader, output io.Writer) *RequestFeedbackTool {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}
	return &RequestFeedbackTool{
		input:  input,
		output: output,
	}
}

// Name returns the tool name
func (t *RequestFeedbackTool) Name() string {
	return "request_feedback"
}

// Description returns the tool description
func (t *RequestFeedbackTool) Description() string {
	return `**IMPORTANT**: Request interactive feedback from the user during analysis.
This is a CRITICAL tool for effective debugging - use it proactively!

Use this tool EARLY and OFTEN when you need:
- Clarification about the problem (missing error messages, reproduction steps, timing)
- Understanding of impact scope (how many users, how often, severity)
- Domain knowledge or business logic clarification
- Direction when multiple investigation paths exist
- Validation of your findings or hypotheses
- Prioritization when you found multiple issues

Parameters:
- title (required): Question title - concise summary of what you're asking
- content (required): Question content - detailed background, current analysis state, or information the user needs to know
- prompt (required): Prompt text - guide the user on how to answer (e.g., "Please select an option", "Please enter the file path", "Please describe the error scenario")
- options (optional): Options list. If provided, user selects from options; if not provided, user enters free text

Returns:
- If options are provided: Returns the selected option text
- If no options: Returns the user's text input
- If user provides no input: Returns empty string (agent should make own judgment)

**When to use this tool (USE LIBERALLY)**:
✅ Phase 1 (Problem Definition): Missing critical information about symptoms, timing, or scope
✅ Phase 2 (Impact Analysis): Need to understand frequency, user impact, or severity
✅ Phase 3 (Root Cause Hypothesis): Need domain knowledge or business logic clarification
✅ Phase 5 (Execution): Multiple possible causes found - need prioritization
✅ Phase 5 (Execution): Stuck after several attempts - need different perspective
✅ Phase 6 (Verification): Need user to validate findings or confirm solution

**Best practices**:
- Ask early - don't wait until you're stuck
- Use clear, specific titles and prompts
- Provide relevant context in the content field
- For choice questions: provide 2-4 clear, distinct options
- For open questions: provide clear guidance in the prompt
- Use it 3-5 times per session is NORMAL and ENCOURAGED

**Example 1 (Multiple choice)**:
{
  "title": "Priority Selection",
  "content": "I found two potential issues:\n1. Memory leak in cache layer (more severe, affects all users)\n2. Slow user profile query (less severe, affects specific endpoint)",
  "prompt": "Please select which issue to investigate first",
  "options": [
    "Investigate memory leak first",
    "Investigate database query first",
    "Investigate both in parallel"
  ]
}

**Example 2 (Open-ended question)**:
{
  "title": "Error Reproduction Steps",
  "content": "I need to understand the exact steps to reproduce this error for accurate diagnosis.",
  "prompt": "Please describe in detail how to reproduce this error (including steps, input data, expected results, etc.)"
}

**Example 3 (Simple text input)**:
{
  "title": "Log File Path",
  "content": "Need to examine the application log file to analyze the error stack trace.",
  "prompt": "Please provide the complete path to the log file"
}`
}

// Execute runs the tool and requests feedback from the user
func (t *RequestFeedbackTool) Execute(ctx context.Context, params *RequestFeedbackParams) (string, error) {
	if params == nil {
		return "", fmt.Errorf("params is required")
	}

	if params.Title == "" {
		return "", fmt.Errorf("title is required")
	}

	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}

	if params.Prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	// Print a separator for clarity
	separator := strings.Repeat("═", 80)
	fmt.Fprintln(t.output, "\n"+separator)
	fmt.Fprintln(t.output, "🤔 Agent needs your input")
	fmt.Fprintln(t.output, separator)

	// Print title
	fmt.Fprintf(t.output, "\n📌 %s\n", params.Title)
	fmt.Fprintln(t.output)

	// Print content
	fmt.Fprintln(t.output, "📋 Details:")
	fmt.Fprintln(t.output, params.Content)
	fmt.Fprintln(t.output)

	var userResponse string
	var selectedIndex int = -1

	// Check if this is a multiple choice question or open-ended
	if len(params.Options) > 0 {
		// Multiple choice question
		if len(params.Options) < 2 {
			return "", fmt.Errorf("at least 2 options are required for multiple choice questions")
		}

		// Use the UI SelectOption function
		idx, err := ui.SelectOption(
			params.Prompt,
			params.Options,
			0, // default to first option
			t.input,
			t.output,
		)
		if err != nil {
			return "", fmt.Errorf("failed to get user feedback: %w", err)
		}

		selectedIndex = idx
		userResponse = params.Options[idx]

		// Print confirmation
		fmt.Fprintf(t.output, "\n✓ You selected: %s\n", userResponse)
	} else {
		// Open-ended question - get text input
		fmt.Fprintf(t.output, "💬 %s\n", params.Prompt)
		fmt.Fprint(t.output, "> ")

		// Read user input
		var input strings.Builder
		buf := make([]byte, 1)
		for {
			n, err := t.input.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return "", fmt.Errorf("failed to read user input: %w", err)
			}
			if n > 0 {
				if buf[0] == '\n' {
					break
				}
				input.WriteByte(buf[0])
			}
		}

		userResponse = strings.TrimSpace(input.String())

		// If user provided no input, return empty string (agent should make own judgment)
		if userResponse == "" {
			fmt.Fprintln(t.output, "\n⚠️  No input received, Agent will make its own judgment")
		} else {
			fmt.Fprintf(t.output, "\n✓ Received your input: %s\n", userResponse)
		}
	}

	fmt.Fprintln(t.output, separator+"\n")

	// Return the response as a structured JSON
	response := map[string]interface{}{
		"user_response": userResponse,
		"title":         params.Title,
	}

	if selectedIndex >= 0 {
		response["selected_index"] = selectedIndex
		response["is_choice"] = true
	} else {
		response["is_choice"] = false
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return userResponse, nil // Fallback to plain text
	}

	return string(responseJSON), nil
}
