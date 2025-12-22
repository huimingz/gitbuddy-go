package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/cloudwego/eino/schema"
	"github.com/huimingz/gitbuddy-go/internal/agent/tools"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// CommitRequest represents a request to generate a commit message
type CommitRequest struct {
	Language string // Output language
	Context  string // User-provided context (optional)
}

// CommitInfo represents the structured commit information from LLM tool call
// This ensures the LLM outputs only commit-related content without extra descriptions
type CommitInfo struct {
	Type        string `json:"type"`             // Commit type: feat, fix, docs, style, refactor, perf, test, chore
	Scope       string `json:"scope,omitempty"`  // Commit scope (optional)
	Description string `json:"description"`      // Short description (subject line)
	Body        string `json:"body,omitempty"`   // Detailed description (optional)
	Footer      string `json:"footer,omitempty"` // Footer for breaking changes or issue references (optional)
}

// Title returns the formatted commit title (first line)
func (c *CommitInfo) Title() string {
	if c.Scope != "" {
		return fmt.Sprintf("%s(%s): %s", c.Type, c.Scope, c.Description)
	}
	return fmt.Sprintf("%s: %s", c.Type, c.Description)
}

// Message returns the complete formatted commit message
func (c *CommitInfo) Message() string {
	var parts []string

	// Title (required)
	parts = append(parts, c.Title())

	// Body (optional)
	if c.Body != "" {
		parts = append(parts, "") // Empty line between title and body
		parts = append(parts, c.Body)
	}

	// Footer (optional)
	if c.Footer != "" {
		parts = append(parts, "") // Empty line before footer
		parts = append(parts, c.Footer)
	}

	return strings.Join(parts, "\n")
}

// Validate checks if the commit info is valid
func (c *CommitInfo) Validate() error {
	validTypes := map[string]bool{
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

	if c.Type == "" {
		return fmt.Errorf("commit type is required")
	}
	if !validTypes[c.Type] {
		return fmt.Errorf("invalid commit type: %s", c.Type)
	}
	if c.Description == "" {
		return fmt.Errorf("commit description is required")
	}
	return nil
}

// CommitInfoFromToolParams converts SubmitCommitParams to CommitInfo
func CommitInfoFromToolParams(params *tools.SubmitCommitParams) *CommitInfo {
	return &CommitInfo{
		Type:        params.Type,
		Scope:       params.Scope,
		Description: params.Description,
		Body:        params.Body,
		Footer:      params.Footer,
	}
}

// CommitResponse represents the generated commit message
type CommitResponse struct {
	CommitInfo       *CommitInfo // Structured commit information
	Message          string      // Complete formatted commit message
	PromptTokens     int         // Number of tokens in the prompt
	CompletionTokens int         // Number of tokens in the completion
	TotalTokens      int         // Total tokens used
}

// CommitAgentOptions contains configuration for CommitAgent
type CommitAgentOptions struct {
	Language    string            // Output language (default: "en")
	GitExecutor git.Executor      // Git executor for running git commands
	LLMProvider llm.Provider      // LLM provider for generating messages
	Printer     *ui.StreamPrinter // Stream printer for output (optional)
	Output      io.Writer         // Output writer (used if Printer is nil)
	Debug       bool              // Enable debug mode
}

// Validate validates the options and sets defaults
func (o *CommitAgentOptions) Validate() error {
	if o.Language == "" {
		o.Language = "en"
	}
	return nil
}

// getPrinter returns the printer or creates a default one
func (o *CommitAgentOptions) getPrinter() *ui.StreamPrinter {
	if o.Printer != nil {
		return o.Printer
	}
	if o.Output != nil {
		return ui.NewStreamPrinter(o.Output, ui.WithVerbose(o.Debug))
	}
	return nil
}

// CommitAgent handles commit message generation
type CommitAgent struct {
	opts CommitAgentOptions
}

// NewCommitAgent creates a new CommitAgent instance
func NewCommitAgent(opts CommitAgentOptions) (*CommitAgent, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	return &CommitAgent{
		opts: opts,
	}, nil
}

// BuildSystemPrompt generates the system prompt for commit message generation
func BuildSystemPrompt(language, context string) string {
	tmpl, err := template.New("system_prompt").Parse(CommitSystemPrompt)
	if err != nil {
		// Fallback to raw prompt if template parsing fails
		return CommitSystemPrompt
	}

	data := struct {
		Language string
		Context  string
	}{
		Language: language,
		Context:  context,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return CommitSystemPrompt
	}

	return buf.String()
}

// GenerateCommitMessage generates a commit message based on staged changes
func (a *CommitAgent) GenerateCommitMessage(ctx context.Context, req CommitRequest) (*CommitResponse, error) {
	printer := a.opts.getPrinter()

	// Helper to print if printer is available
	printStep := func(step int, msg string) {
		if printer != nil {
			_ = printer.PrintStep(step, msg)
		}
		log.Debug("Step %d: %s", step, msg)
	}

	printProgress := func(msg string) {
		if printer != nil {
			_ = printer.PrintProgress(msg)
		}
		log.Debug("%s", msg)
	}

	printToolCall := func(name string) {
		if printer != nil {
			_ = printer.PrintToolCall(name, nil)
		}
		log.Debug("Tool call: %s", name)
	}

	printSuccess := func(msg string) {
		if printer != nil {
			_ = printer.PrintSuccess(msg)
		}
	}

	printInfo := func(msg string) {
		if printer != nil {
			_ = printer.PrintInfo(msg)
		}
	}

	// Create LLM chat model
	if a.opts.LLMProvider == nil {
		return nil, fmt.Errorf("LLM provider is not configured")
	}

	providerName := a.opts.LLMProvider.Name()
	modelName := a.opts.LLMProvider.GetConfig().Model
	printProgress(fmt.Sprintf("Initializing LLM provider (%s/%s)...", providerName, modelName))
	log.Debug("Using LLM: provider=%s, model=%s", providerName, modelName)

	chatModel, err := a.opts.LLMProvider.CreateChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}
	if chatModel == nil {
		return nil, fmt.Errorf("chat model is nil (provider: %s)", providerName)
	}

	// Step 1: Get git status overview (progressive analysis)
	var status string
	if a.opts.GitExecutor != nil {
		printStep(1, "Getting git status overview...")
		printToolCall("git_status")
		status, err = a.opts.GitExecutor.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get git status: %w", err)
		}
		printSuccess("Git status retrieved")
	}

	// Step 2: Get detailed diff
	var diff string
	if a.opts.GitExecutor != nil {
		printStep(2, "Getting staged diff details...")
		printToolCall("git_diff_cached")
		diff, err = a.opts.GitExecutor.DiffCached(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get staged changes: %w", err)
		}
		if diff == "" {
			return nil, fmt.Errorf("no staged changes found")
		}
		printSuccess(fmt.Sprintf("Staged diff retrieved (%d bytes)", len(diff)))
	}

	// Build system prompt
	printStep(3, "Analyzing changes and generating commit message...")
	systemPrompt := BuildSystemPrompt(req.Language, req.Context)
	printInfo(fmt.Sprintf("Language: %s", req.Language))
	if req.Context != "" {
		printInfo(fmt.Sprintf("Context: %s", req.Context))
	}
	log.Debug("System prompt built")

	// Build user message with progressive information
	var userMessageBuilder strings.Builder
	userMessageBuilder.WriteString("Please analyze the following staged changes and generate a commit message.\n\n")

	// Include git status overview first
	userMessageBuilder.WriteString("## Git Status Overview\n")
	userMessageBuilder.WriteString("```\n")
	userMessageBuilder.WriteString(status)
	userMessageBuilder.WriteString("\n```\n\n")

	// Then include detailed diff
	userMessageBuilder.WriteString("## Staged Changes (Diff)\n")
	userMessageBuilder.WriteString("```diff\n")
	userMessageBuilder.WriteString(diff)
	userMessageBuilder.WriteString("\n```\n")

	userMessage := userMessageBuilder.String()

	// Define the submit_commit tool for structured output
	submitCommitTool := &schema.ToolInfo{
		Name: "submit_commit",
		Desc: "Submit the structured commit information. Use this to output the commit message in a structured format.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"type": {
				Type:     schema.String,
				Desc:     "Commit type: feat, fix, docs, style, refactor, perf, test, chore, build, ci, or revert",
				Required: true,
			},
			"scope": {
				Type:     schema.String,
				Desc:     "Commit scope (optional)",
				Required: false,
			},
			"description": {
				Type:     schema.String,
				Desc:     "Short description (subject line, max 50 chars preferred)",
				Required: true,
			},
			"body": {
				Type:     schema.String,
				Desc:     "Detailed description explaining what and why (optional)",
				Required: false,
			},
			"footer": {
				Type:     schema.String,
				Desc:     "Footer for breaking changes or issue references (optional)",
				Required: false,
			},
		}),
	}

	// Bind tool to chat model
	if err := chatModel.BindTools([]*schema.ToolInfo{submitCommitTool}); err != nil {
		return nil, fmt.Errorf("failed to bind tools: %w", err)
	}

	// Create messages
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: userMessage,
		},
	}

	printProgress("Sending request to LLM (streaming)...")
	log.Debug("Sending request to LLM with streaming")

	// Use streaming API for real-time output
	streamReader, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM stream failed: %w", err)
	}
	defer streamReader.Close()

	// Collect the full response while streaming
	var fullContent strings.Builder
	var toolCalls []*schema.ToolCall
	var streamStarted bool
	var promptTokens, completionTokens, totalTokens int

	printInfo("LLM Response:")
	if printer != nil {
		_ = printer.Newline()
	}

	// Read from stream and output in real-time
	for {
		chunk, err := streamReader.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("stream read error: %w", err)
		}

		// Stream content token by token
		if chunk.Content != "" {
			if !streamStarted {
				streamStarted = true
			}
			fullContent.WriteString(chunk.Content)

			// Print each token in real-time
			if printer != nil {
				_ = printer.PrintLLMContent(chunk.Content)
			}
		}

		// Collect tool calls from chunks
		if len(chunk.ToolCalls) > 0 {
			for _, tc := range chunk.ToolCalls {
				// Get the index (default to 0 if nil)
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}

				// Find or create the tool call by index
				for len(toolCalls) <= idx {
					toolCalls = append(toolCalls, &schema.ToolCall{
						Function: schema.FunctionCall{},
					})
				}

				// Accumulate function name and arguments
				if tc.Function.Name != "" {
					// Show tool call immediately when we first see the name
					if toolCalls[idx].Function.Name == "" {
						printToolCall(fmt.Sprintf("%s (from LLM)", tc.Function.Name))
					}
					toolCalls[idx].Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCalls[idx].Function.Arguments += tc.Function.Arguments
				}
			}
		}

		// Collect token usage from ResponseMeta (usually in the last chunk)
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			usage := chunk.ResponseMeta.Usage
			if usage.PromptTokens > promptTokens {
				promptTokens = usage.PromptTokens
			}
			if usage.CompletionTokens > completionTokens {
				completionTokens = usage.CompletionTokens
			}
			if usage.TotalTokens > totalTokens {
				totalTokens = usage.TotalTokens
			}
		}
	}

	if printer != nil {
		_ = printer.Newline()
	}

	// Log the full content
	if fullContent.Len() > 0 {
		log.Debug("LLM content: %s", fullContent.String())
	}

	// Check for tool calls in the response
	if len(toolCalls) > 0 {
		for _, toolCall := range toolCalls {
			if toolCall.Function.Name == "" {
				continue
			}
			log.Debug("Tool call: %s with args: %s", toolCall.Function.Name, toolCall.Function.Arguments)

			if toolCall.Function.Name == "submit_commit" {
				var params tools.SubmitCommitParams
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse tool call arguments: %v", err)
					continue
				}

				// Validate the params
				if err := params.Validate(); err != nil {
					log.Debug("Invalid commit params: %v", err)
					continue
				}

				// Convert to CommitInfo
				commitInfo := CommitInfoFromToolParams(&params)
				printSuccess("Commit message generated successfully")

				return &CommitResponse{
					CommitInfo:       commitInfo,
					Message:          commitInfo.Message(),
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				}, nil
			}
		}
	}

	// If no tool call, try to parse the response content as JSON (fallback)
	content := fullContent.String()
	if content != "" {
		// Try to extract commit info from text response
		printInfo("Parsing text response (fallback mode)...")
		log.Debug("No tool call found, using fallback parsing")
		result, err := parseTextResponse(content)
		if err == nil {
			printSuccess("Commit message parsed from text response")
			// Add token usage to result
			result.PromptTokens = promptTokens
			result.CompletionTokens = completionTokens
			result.TotalTokens = totalTokens
		}
		return result, err
	}

	return nil, fmt.Errorf("failed to generate commit message: no valid response from LLM")
}

// parseTextResponse attempts to extract commit info from a text response
func parseTextResponse(content string) (*CommitResponse, error) {
	// This is a fallback - try to parse if the LLM returns text instead of tool call
	// Simple heuristic: look for conventional commit pattern
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	// Try to parse the first line as a conventional commit title
	title := strings.TrimSpace(lines[0])
	// Remove any markdown code blocks if present
	title = strings.TrimPrefix(title, "```")
	title = strings.TrimSuffix(title, "```")
	title = strings.TrimSpace(title)

	commitInfo := &CommitInfo{}

	// Parse type and optional scope from title
	if idx := strings.Index(title, ":"); idx > 0 {
		prefix := title[:idx]
		commitInfo.Description = strings.TrimSpace(title[idx+1:])

		// Check for scope: type(scope)
		if scopeStart := strings.Index(prefix, "("); scopeStart > 0 {
			if scopeEnd := strings.Index(prefix, ")"); scopeEnd > scopeStart {
				commitInfo.Type = prefix[:scopeStart]
				commitInfo.Scope = prefix[scopeStart+1 : scopeEnd]
			}
		} else {
			commitInfo.Type = prefix
		}
	} else {
		// No conventional commit format, use as description
		commitInfo.Type = "feat"
		commitInfo.Description = title
	}

	// Get body from remaining lines
	if len(lines) > 2 {
		bodyLines := []string{}
		for i := 2; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line != "" {
				bodyLines = append(bodyLines, line)
			}
		}
		if len(bodyLines) > 0 {
			commitInfo.Body = strings.Join(bodyLines, "\n")
		}
	}

	if err := commitInfo.Validate(); err != nil {
		return nil, fmt.Errorf("failed to parse commit message: %w", err)
	}

	return &CommitResponse{
		CommitInfo: commitInfo,
		Message:    commitInfo.Message(),
	}, nil
}
