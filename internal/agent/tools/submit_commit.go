package tools

import (
	"context"
	"fmt"
	"strings"
)

// Valid commit types
var validCommitTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"docs":     true,
	"style":    true,
	"refactor": true,
	"perf":     true,
	"test":     true,
	"chore":    true,
	"build":    true,
	"ci":       true,
	"revert":   true,
}

// SubmitCommitParams represents the parameters for the submit_commit tool
// This tool is used by LLM to submit structured commit information
type SubmitCommitParams struct {
	// Type is the commit type (required)
	// Valid values: feat, fix, docs, style, refactor, perf, test, chore, build, ci, revert
	Type string `json:"type" jsonschema:"required,description=The type of commit: feat fix docs style refactor perf test chore build ci revert"`

	// Scope is the commit scope (optional)
	// Example: auth, api, ui, etc.
	Scope string `json:"scope,omitempty" jsonschema:"description=The scope of the commit (e.g. auth api ui)"`

	// Description is the short description/subject line (required)
	// Should be concise, 50 chars or less preferred
	// Use imperative mood, do not end with a period
	Description string `json:"description" jsonschema:"required,description=Short description of the change. Use imperative mood. Do not end with period. 50 chars or less preferred."`

	// Body is the detailed description (optional)
	// Explains what and why, not how
	Body string `json:"body,omitempty" jsonschema:"description=Detailed description explaining what and why (not how). Can be multiple lines."`

	// Footer is for breaking changes or issue references (optional)
	// Example: "BREAKING CHANGE: description" or "Closes #123"
	Footer string `json:"footer,omitempty" jsonschema:"description=Footer for breaking changes or issue references. Example: BREAKING CHANGE: xxx or Closes #123"`
}

// Validate validates the commit parameters
func (p *SubmitCommitParams) Validate() error {
	if p.Type == "" {
		return fmt.Errorf("commit type is required")
	}
	if !validCommitTypes[p.Type] {
		return fmt.Errorf("invalid commit type: %s", p.Type)
	}
	if p.Description == "" {
		return fmt.Errorf("commit description is required")
	}
	return nil
}

// FormatMessage formats the commit message according to Conventional Commits
func (p *SubmitCommitParams) FormatMessage() string {
	var parts []string

	// Title line
	var title string
	if p.Scope != "" {
		title = fmt.Sprintf("%s(%s): %s", p.Type, p.Scope, p.Description)
	} else {
		title = fmt.Sprintf("%s: %s", p.Type, p.Description)
	}
	parts = append(parts, title)

	// Body (optional)
	if p.Body != "" {
		parts = append(parts, "") // Empty line
		parts = append(parts, p.Body)
	}

	// Footer (optional)
	if p.Footer != "" {
		parts = append(parts, "") // Empty line
		parts = append(parts, p.Footer)
	}

	return strings.Join(parts, "\n")
}

// SubmitCommitCallback is called when commit info is submitted
type SubmitCommitCallback func(info *SubmitCommitParams) error

// SubmitCommitTool is a tool for submitting structured commit information
type SubmitCommitTool struct {
	callback SubmitCommitCallback
}

// NewSubmitCommitTool creates a new SubmitCommitTool
func NewSubmitCommitTool(callback SubmitCommitCallback) *SubmitCommitTool {
	return &SubmitCommitTool{callback: callback}
}

// Name returns the tool name
func (t *SubmitCommitTool) Name() string {
	return "submit_commit"
}

// Description returns the tool description
func (t *SubmitCommitTool) Description() string {
	return `Submit structured commit information following the Conventional Commits specification.
This tool MUST be called to submit the generated commit message.
The commit message will be formatted as: <type>[optional scope]: <description>

Parameters:
- type (required): The type of commit (feat, fix, docs, style, refactor, perf, test, chore, build, ci, revert)
- scope (optional): The scope of the commit (e.g., auth, api, ui)
- description (required): Short description of the change, use imperative mood, do not end with period
- body (optional): Detailed description explaining what and why
- footer (optional): For breaking changes or issue references`
}

// Execute runs the tool with the given parameters
func (t *SubmitCommitTool) Execute(ctx context.Context, params interface{}) (string, error) {
	p, ok := params.(*SubmitCommitParams)
	if !ok {
		return "", fmt.Errorf("invalid parameters type")
	}

	// Validate
	if err := p.Validate(); err != nil {
		return "", err
	}

	// Call the callback if set
	if t.callback != nil {
		if err := t.callback(p); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("Commit information submitted successfully:\n%s", p.FormatMessage()), nil
}
