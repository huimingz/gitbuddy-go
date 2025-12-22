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
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// PRRequest contains the input for PR description generation
type PRRequest struct {
	BaseBranch string // Target branch (e.g., main, develop)
	HeadBranch string // Source branch (current branch)
	Language   string // Output language
	Context    string // Additional context from user
}

// PRInfo contains structured PR information
type PRInfo struct {
	Title       string   // PR title
	Summary     string   // Brief summary of changes
	Changes     []string // List of main changes
	Why         string   // Why these changes were made
	Impact      string   // Potential impact
	TestingNote string   // Testing notes (optional)
}

// FormatDescription formats the PR description as markdown
func (p *PRInfo) FormatDescription() string {
	var sb strings.Builder

	// Summary section
	if p.Summary != "" {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(p.Summary)
		sb.WriteString("\n\n")
	}

	// Changes section
	if len(p.Changes) > 0 {
		sb.WriteString("## Changes\n\n")
		for _, change := range p.Changes {
			sb.WriteString("- ")
			sb.WriteString(change)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Why section
	if p.Why != "" {
		sb.WriteString("## Why\n\n")
		sb.WriteString(p.Why)
		sb.WriteString("\n\n")
	}

	// Impact section
	if p.Impact != "" {
		sb.WriteString("## Impact\n\n")
		sb.WriteString(p.Impact)
		sb.WriteString("\n\n")
	}

	// Testing notes section
	if p.TestingNote != "" {
		sb.WriteString("## Testing\n\n")
		sb.WriteString(p.TestingNote)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// PRResponse contains the result of PR description generation
type PRResponse struct {
	PRInfo           *PRInfo // Structured PR information
	Title            string  // PR title
	Description      string  // Complete formatted description
	PromptTokens     int     // Number of tokens in the prompt
	CompletionTokens int     // Number of tokens in the completion
	TotalTokens      int     // Total tokens used
}

// GetTitle returns the PR title (implements ui.PRDescriptionDisplayer)
func (r *PRResponse) GetTitle() string {
	return r.Title
}

// GetDescription returns the PR description (implements ui.PRDescriptionDisplayer)
func (r *PRResponse) GetDescription() string {
	return r.Description
}

// PRAgentOptions contains configuration for PRAgent
type PRAgentOptions struct {
	Language    string            // Output language (default: "en")
	GitExecutor git.Executor      // Git executor for running git commands
	LLMProvider llm.Provider      // LLM provider for generating messages
	Printer     *ui.StreamPrinter // Stream printer for output (optional)
	Output      io.Writer         // Output writer (used if Printer is nil)
	Debug       bool              // Enable debug mode
}

// PRAgent generates PR descriptions using LLM
type PRAgent struct {
	opts PRAgentOptions
}

// NewPRAgent creates a new PRAgent
func NewPRAgent(opts PRAgentOptions) *PRAgent {
	if opts.Language == "" {
		opts.Language = "en"
	}
	return &PRAgent{opts: opts}
}

// SubmitPRParams represents the structured PR information from LLM
type SubmitPRParams struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Changes     []string `json:"changes"`
	Why         string   `json:"why"`
	Impact      string   `json:"impact,omitempty"`
	TestingNote string   `json:"testing_note,omitempty"`
}

// ToPRInfo converts SubmitPRParams to PRInfo
func (p *SubmitPRParams) ToPRInfo() *PRInfo {
	return &PRInfo{
		Title:       p.Title,
		Summary:     p.Summary,
		Changes:     p.Changes,
		Why:         p.Why,
		Impact:      p.Impact,
		TestingNote: p.TestingNote,
	}
}

// BuildPRSystemPrompt builds the system prompt for PR generation
func BuildPRSystemPrompt(language, context string) string {
	tmpl, err := template.New("pr_prompt").Parse(PRSystemPrompt)
	if err != nil {
		return PRSystemPrompt
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Language": language,
		"Context":  context,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return PRSystemPrompt
	}
	return buf.String()
}

// GeneratePRDescription generates a PR description based on branch diff
func (a *PRAgent) GeneratePRDescription(ctx context.Context, req PRRequest) (*PRResponse, error) {
	printer := a.opts.Printer

	// Helper functions for printing
	printProgress := func(msg string) {
		if printer != nil {
			_ = printer.PrintProgress(msg)
		}
		log.Debug(msg)
	}

	printStep := func(step int, msg string) {
		if printer != nil {
			_ = printer.PrintStep(step, msg)
		}
		log.Debug("Step %d: %s", step, msg)
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

	// Step 1: Get current branch info
	printStep(1, fmt.Sprintf("Comparing %s with %s...", req.HeadBranch, req.BaseBranch))

	// Step 2: Get commit log between branches
	var commitLog string
	if a.opts.GitExecutor != nil {
		printToolCall("git_log")
		commitLog, err = a.opts.GitExecutor.LogRange(ctx, req.BaseBranch, req.HeadBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit log: %w", err)
		}
		printSuccess(fmt.Sprintf("Found commits: %d", len(strings.Split(commitLog, "\n"))))
	}

	// Step 3: Get diff between branches
	var diff string
	if a.opts.GitExecutor != nil {
		printStep(2, "Getting diff between branches...")
		printToolCall("git_diff")
		diff, err = a.opts.GitExecutor.DiffBranches(ctx, req.BaseBranch, req.HeadBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get branch diff: %w", err)
		}
		if diff == "" {
			return nil, fmt.Errorf("no differences found between %s and %s", req.BaseBranch, req.HeadBranch)
		}
		printSuccess(fmt.Sprintf("Diff retrieved (%d bytes)", len(diff)))
	}

	// Build system prompt
	printStep(3, "Analyzing changes and generating PR description...")
	systemPrompt := BuildPRSystemPrompt(req.Language, req.Context)
	printInfo(fmt.Sprintf("Language: %s", req.Language))
	log.Debug("System prompt built")

	// Build user message with branch info, commits, and diff
	var userMessage strings.Builder
	userMessage.WriteString(fmt.Sprintf("## Branch Information\n"))
	userMessage.WriteString(fmt.Sprintf("- Source branch: %s\n", req.HeadBranch))
	userMessage.WriteString(fmt.Sprintf("- Target branch: %s\n\n", req.BaseBranch))

	userMessage.WriteString("## Commits in this PR\n")
	userMessage.WriteString("```\n")
	userMessage.WriteString(commitLog)
	userMessage.WriteString("\n```\n\n")

	userMessage.WriteString("## Code Changes (Diff)\n")
	userMessage.WriteString("```diff\n")
	userMessage.WriteString(diff)
	userMessage.WriteString("\n```\n")

	// Define the submit_pr tool for structured output
	submitPRTool := &schema.ToolInfo{
		Name: "submit_pr",
		Desc: "Submit the structured PR information. Use this to output the PR title and description in a structured format.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title": {
				Type:     schema.String,
				Desc:     "PR title - concise summary of the changes (max 72 chars)",
				Required: true,
			},
			"summary": {
				Type:     schema.String,
				Desc:     "Brief summary explaining what this PR does",
				Required: true,
			},
			"changes": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "List of main changes made in this PR",
				Required: true,
			},
			"why": {
				Type:     schema.String,
				Desc:     "Explanation of why these changes were needed",
				Required: true,
			},
			"impact": {
				Type:     schema.String,
				Desc:     "Potential impact of these changes (optional)",
				Required: false,
			},
			"testing_note": {
				Type:     schema.String,
				Desc:     "Notes about testing (optional)",
				Required: false,
			},
		}),
	}

	// Bind tool to chat model
	if err := chatModel.BindTools([]*schema.ToolInfo{submitPRTool}); err != nil {
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
			Content: userMessage.String(),
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
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}

				for len(toolCalls) <= idx {
					toolCalls = append(toolCalls, &schema.ToolCall{
						Function: schema.FunctionCall{},
					})
				}

				if tc.Function.Name != "" {
					if toolCalls[idx].Function.Name == "" {
						printToolCall(fmt.Sprintf("%s (from LLM)", tc.Function.Name))
						// Start argument display
						if printer != nil {
							_ = printer.PrintToolArgStart()
						}
					}
					toolCalls[idx].Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCalls[idx].Function.Arguments += tc.Function.Arguments
					// Stream the argument chunk in real-time
					if printer != nil {
						_ = printer.PrintToolArgChunk(tc.Function.Arguments)
					}
				}
			}
		}

		// Collect token usage from ResponseMeta
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

	// Check for tool calls in the response
	if len(toolCalls) > 0 {
		for _, toolCall := range toolCalls {
			if toolCall.Function.Name == "" {
				continue
			}
			log.Debug("Tool call: %s with args: %s", toolCall.Function.Name, toolCall.Function.Arguments)

			if toolCall.Function.Name == "submit_pr" {
				var params SubmitPRParams
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse tool call arguments: %v", err)
					continue
				}

				prInfo := params.ToPRInfo()
				printSuccess("PR description generated successfully")

				return &PRResponse{
					PRInfo:           prInfo,
					Title:            params.Title,
					Description:      prInfo.FormatDescription(),
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to generate PR description: no valid response from LLM")
}
