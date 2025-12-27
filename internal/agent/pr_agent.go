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

// PRRequest contains the input for PR description generation
type PRRequest struct {
	BaseBranch string // Target branch (e.g., main, develop)
	HeadBranch string // Source branch (current branch)
	Language   string // Output language
	Context    string // Additional context from user
}

// PRInfo contains PR information
type PRInfo struct {
	Title       string // PR title
	Description string // Full PR description
}

// PRResponse contains the result of PR description generation
type PRResponse struct {
	PRInfo           *PRInfo
	Title            string
	Description      string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
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
	Language    string
	Template    string // Custom PR template, if empty uses default
	GitExecutor git.Executor
	LLMProvider llm.Provider
	Printer     *ui.StreamPrinter
	Output      io.Writer
	Debug       bool
	RetryConfig llm.RetryConfig
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

// SubmitPRParams represents the PR information from LLM
type SubmitPRParams struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ToPRInfo converts SubmitPRParams to PRInfo
func (p *SubmitPRParams) ToPRInfo() *PRInfo {
	return &PRInfo{
		Title:       p.Title,
		Description: p.Description,
	}
}

// BuildPRSystemPrompt builds the system prompt for PR generation
func BuildPRSystemPrompt(language, context, baseBranch, headBranch, prTemplate string) string {
	// Use default template if not provided
	if prTemplate == "" {
		prTemplate = DefaultPRTemplate
	}

	tmpl, err := template.New("pr_prompt").Parse(PRSystemPrompt)
	if err != nil {
		return PRSystemPrompt
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Language":   language,
		"Context":    context,
		"BaseBranch": baseBranch,
		"HeadBranch": headBranch,
		"Template":   prTemplate,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return PRSystemPrompt
	}
	return buf.String()
}

// GeneratePRDescription generates a PR description using agent loop
func (a *PRAgent) GeneratePRDescription(ctx context.Context, req PRRequest) (*PRResponse, error) {
	printer := a.opts.Printer

	// Helper functions
	printProgress := func(msg string) {
		if printer != nil {
			_ = printer.PrintProgress(msg)
		}
		log.Debug(msg)
	}

	printToolCall := func(name string) {
		if printer != nil {
			_ = printer.PrintToolCall(name, nil)
		}
		log.Debug("Tool call: %s", name)
	}

	// estimateTokenCount estimates token count from text
	// This is a simple heuristic: ~4 chars per token for English, ~1.5 chars per token for Chinese
	// For mixed content, we use a weighted average
	printToolResult := func(name string, result string) {
		if printer != nil {
			bytes := len(result)
			tokens := estimateTokenCount(result)
			_ = printer.PrintSuccess(fmt.Sprintf("%s returned %d bytes (~%d tokens)", name, bytes, tokens))
		}
	}

	printInfo := func(msg string) {
		if printer != nil {
			_ = printer.PrintInfo(msg)
		}
	}

	printSuccess := func(msg string) {
		if printer != nil {
			_ = printer.PrintSuccess(msg)
		}
	}

	// Create LLM chat model
	if a.opts.LLMProvider == nil {
		return nil, fmt.Errorf("LLM provider is not configured")
	}

	providerName := a.opts.LLMProvider.Name()
	modelName := a.opts.LLMProvider.GetConfig().Model
	printProgress(fmt.Sprintf("Initializing LLM provider (%s/%s)...", providerName, modelName))

	chatModel, err := a.opts.LLMProvider.CreateChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}
	if chatModel == nil {
		return nil, fmt.Errorf("chat model is nil (provider: %s)", providerName)
	}

	// Create git tools
	gitDiffBranchesTool := tools.NewGitDiffBranchesTool(a.opts.GitExecutor)
	gitLogRangeTool := tools.NewGitLogRangeTool(a.opts.GitExecutor)
	gitStatusTool := tools.NewGitStatusTool(a.opts.GitExecutor)

	// Define tool schemas
	toolInfos := []*schema.ToolInfo{
		{
			Name: "git_diff_branches",
			Desc: gitDiffBranchesTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"base": {Type: schema.String, Desc: "Base branch to compare from", Required: true},
				"head": {Type: schema.String, Desc: "Head branch to compare to (defaults to HEAD)", Required: false},
			}),
		},
		{
			Name: "git_log_range",
			Desc: gitLogRangeTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"base": {Type: schema.String, Desc: "Base branch to compare from", Required: true},
				"head": {Type: schema.String, Desc: "Head branch to compare to (defaults to HEAD)", Required: false},
			}),
		},
		{
			Name:        "git_status",
			Desc:        gitStatusTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		},
		{
			Name: "submit_pr",
			Desc: "Submit the PR title and description. Call this when you have analyzed the changes and are ready to generate the PR description.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"title":       {Type: schema.String, Desc: "PR title (max 72 chars)", Required: true},
				"description": {Type: schema.String, Desc: "Full PR description following the template format", Required: true},
			}),
		},
	}

	// Bind tools to chat model
	if err := chatModel.BindTools(toolInfos); err != nil {
		return nil, fmt.Errorf("failed to bind tools: %w", err)
	}

	// Build system prompt
	systemPrompt := BuildPRSystemPrompt(req.Language, req.Context, req.BaseBranch, req.HeadBranch, a.opts.Template)
	printInfo(fmt.Sprintf("Generating PR: %s → %s", req.HeadBranch, req.BaseBranch))

	// Initial messages
	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: fmt.Sprintf("Please generate a PR description for merging branch '%s' into '%s'. Use the available tools to analyze the changes.", req.HeadBranch, req.BaseBranch)},
	}

	var promptTokens, completionTokens, totalTokens int
	maxIterations := 10

	// Agent loop
	for i := 0; i < maxIterations; i++ {
		printProgress(fmt.Sprintf("Agent iteration %d...", i+1))

		// Stream LLM response with retry
		streamReader, err := llm.WithRetryResult(ctx, a.opts.RetryConfig, func() (*schema.StreamReader[*schema.Message], error) {
			return chatModel.Stream(ctx, messages)
		})
		if err != nil {
			return nil, fmt.Errorf("LLM stream failed: %w", err)
		}

		var fullContent strings.Builder
		var toolCalls []*schema.ToolCall
		var toolArgStarted bool

		printInfo("LLM Response:")
		if printer != nil {
			_ = printer.Newline()
		}

		// Read stream
		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				streamReader.Close()
				return nil, fmt.Errorf("stream read error: %w", err)
			}

			if chunk.Content != "" {
				fullContent.WriteString(chunk.Content)
				if printer != nil {
					_ = printer.PrintLLMContent(chunk.Content)
				}
			}

			// Collect tool calls
			if len(chunk.ToolCalls) > 0 {
				for _, tc := range chunk.ToolCalls {
					idx := 0
					if tc.Index != nil {
						idx = *tc.Index
					}

					for len(toolCalls) <= idx {
						toolCalls = append(toolCalls, &schema.ToolCall{Function: schema.FunctionCall{}})
					}

					// Collect tool call ID
					if tc.ID != "" {
						toolCalls[idx].ID = tc.ID
					}

					if tc.Function.Name != "" {
						if toolCalls[idx].Function.Name == "" {
							printToolCall(tc.Function.Name)
							if printer != nil {
								_ = printer.PrintToolArgStart()
							}
							toolArgStarted = true
						}
						toolCalls[idx].Function.Name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						toolCalls[idx].Function.Arguments += tc.Function.Arguments
						if printer != nil && toolArgStarted {
							_ = printer.PrintToolArgChunk(tc.Function.Arguments)
						}
					}
				}
			}

			// Collect token usage
			if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				usage := chunk.ResponseMeta.Usage
				promptTokens += usage.PromptTokens
				completionTokens += usage.CompletionTokens
				totalTokens += usage.TotalTokens
			}
		}
		streamReader.Close()

		if printer != nil {
			_ = printer.Newline()
		}

		// Add assistant message to history
		// Convert []*schema.ToolCall to []schema.ToolCall
		var toolCallsValue []schema.ToolCall
		for _, tc := range toolCalls {
			if tc != nil {
				toolCallsValue = append(toolCallsValue, *tc)
			}
		}
		assistantMsg := &schema.Message{
			Role:      schema.Assistant,
			Content:   fullContent.String(),
			ToolCalls: toolCallsValue,
		}
		messages = append(messages, assistantMsg)

		// Process tool calls - use intelligent fallback if no tools called
		if len(toolCalls) == 0 {
			if err := HandleNoToolCallsResponse(fullContent.String(), "pr"); err != nil {
				return nil, err
			}
			// If we reach here, the response was accepted without tools
			// For PR agent, we still need structured PR info, so we should return error
			return nil, fmt.Errorf("PR agent requires tool usage to analyze changes and generate proper PR description")
		}

		for _, tc := range toolCalls {
			if tc.Function.Name == "" {
				continue
			}

			// Check if it's the final submit_pr call
			if tc.Function.Name == "submit_pr" {
				var params SubmitPRParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse submit_pr arguments: %v", err)
					continue
				}

				prInfo := params.ToPRInfo()
				printSuccess("PR description generated successfully")

				return &PRResponse{
					PRInfo:           prInfo,
					Title:            params.Title,
					Description:      params.Description,
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				}, nil
			}

			// Execute other tools
			var result string
			var toolErr error

			switch tc.Function.Name {
			case "git_diff_branches":
				var params tools.GitDiffBranchesParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = gitDiffBranchesTool.Execute(ctx, &params)
				}

			case "git_log_range":
				var params tools.GitLogRangeParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = gitLogRangeTool.Execute(ctx, &params)
				}

			case "git_status":
				result, toolErr = gitStatusTool.Execute(ctx, nil)

			default:
				toolErr = fmt.Errorf("unknown tool: %s", tc.Function.Name)
			}

			// Build tool result message
			var toolResult string
			if toolErr != nil {
				toolResult = fmt.Sprintf("Error: %v", toolErr)
				log.Debug("Tool %s error: %v", tc.Function.Name, toolErr)
			} else {
				toolResult = result
				printToolResult(tc.Function.Name, result)
			}

			// Add tool result to messages
			messages = append(messages, &schema.Message{
				Role:       schema.Tool,
				Content:    toolResult,
				ToolCallID: tc.ID,
			})
		}
	}

	return nil, fmt.Errorf("agent loop exceeded maximum iterations")
}

func estimateTokenCount(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Count Chinese characters (CJK unified ideographs)
	chineseChars := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseChars++
		}
	}
	// Estimate: Chinese ~1.5 chars/token, others ~4 chars/token
	otherChars := len([]rune(text)) - chineseChars
	tokens := (chineseChars * 2 / 3) + (otherChars / 4)
	if tokens == 0 && len(text) > 0 {
		tokens = 1 // At least 1 token for non-empty text
	}
	return tokens
}
