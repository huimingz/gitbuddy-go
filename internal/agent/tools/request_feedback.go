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
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Context  string   `json:"context,omitempty"`
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
- question (required): The question to ask the user. Should be clear and specific.
- options (required): List of possible choices for the user. Must have at least 2 options.
- context (optional): Additional context about the current analysis state to help the user make a decision.

Returns the user's selected option text.

**When to use this tool (USE LIBERALLY)**:
✅ Phase 1 (Problem Definition): Missing critical information about symptoms, timing, or scope
✅ Phase 2 (Impact Analysis): Need to understand frequency, user impact, or severity
✅ Phase 3 (Root Cause Hypothesis): Need domain knowledge or business logic clarification
✅ Phase 5 (Execution): Multiple possible causes found - need prioritization
✅ Phase 5 (Execution): Stuck after several attempts - need different perspective
✅ Phase 6 (Verification): Need user to validate findings or confirm solution

**Best practices**:
- Ask early - don't wait until you're stuck
- Provide 2-4 clear, distinct options
- Include relevant context from your analysis
- Explain what each option means for the investigation
- Use it 3-5 times per session is NORMAL and ENCOURAGED

**Example**:
{
  "question": "I found two potential issues. Which should I investigate first?",
  "options": [
    "Memory leak in the cache layer (more severe, affects all users)",
    "Slow database query in user profile (less severe, affects specific endpoint)",
    "Investigate both in parallel"
  ],
  "context": "Analysis so far: High memory usage observed. Also found N+1 query pattern in user profile endpoint."
}`
}

// Execute runs the tool and requests feedback from the user
func (t *RequestFeedbackTool) Execute(ctx context.Context, params *RequestFeedbackParams) (string, error) {
	if params == nil {
		return "", fmt.Errorf("params is required")
	}

	if params.Question == "" {
		return "", fmt.Errorf("question is required")
	}

	if len(params.Options) < 2 {
		return "", fmt.Errorf("at least 2 options are required")
	}

	// Print a separator for clarity
	separator := strings.Repeat("═", 80)
	fmt.Fprintln(t.output, "\n"+separator)
	fmt.Fprintln(t.output, "🤔 Agent needs your input")
	fmt.Fprintln(t.output, separator)

	// Print context if provided
	if params.Context != "" {
		fmt.Fprintln(t.output, "\n📋 Current Analysis State:")
		fmt.Fprintln(t.output, params.Context)
		fmt.Fprintln(t.output)
	}

	// Use the UI SelectOption function
	selectedIndex, err := ui.SelectOption(
		params.Question,
		params.Options,
		0, // default to first option
		t.input,
		t.output,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get user feedback: %w", err)
	}

	selectedOption := params.Options[selectedIndex]

	// Print confirmation
	fmt.Fprintf(t.output, "\n✓ You selected: %s\n", selectedOption)
	fmt.Fprintln(t.output, strings.Repeat("═", 80)+"\n")

	// Return the selected option as a structured response
	response := map[string]interface{}{
		"selected_option": selectedOption,
		"selected_index":  selectedIndex,
		"question":        params.Question,
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return selectedOption, nil // Fallback to plain text
	}

	return string(responseJSON), nil
}
