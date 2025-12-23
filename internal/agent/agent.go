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
type CommitInfo struct {
	Type        string `json:"type"`
	Scope       string `json:"scope,omitempty"`
	Description string `json:"description"`
	Body        string `json:"body,omitempty"`
	Footer      string `json:"footer,omitempty"`
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
	parts = append(parts, c.Title())

	if c.Body != "" {
		parts = append(parts, "")
		parts = append(parts, c.Body)
	}

	if c.Footer != "" {
		parts = append(parts, "")
		parts = append(parts, c.Footer)
	}

	return strings.Join(parts, "\n")
}

// Validate checks if the commit info is valid
func (c *CommitInfo) Validate() error {
	validTypes := map[string]bool{
		"feat": true, "fix": true, "docs": true, "style": true,
		"refactor": true, "perf": true, "test": true, "chore": true,
		"build": true, "ci": true, "revert": true,
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

// CommitInfoFromToolParams creates a CommitInfo from tool parameters
func CommitInfoFromToolParams(params *tools.SubmitCommitParams) *CommitInfo {
	return &CommitInfo{
		Type:        params.Type,
		Scope:       params.Scope,
		Description: params.Description,
		Body:        params.Body,
		Footer:      params.Footer,
	}
}

// CommitResponse contains the result of commit message generation
type CommitResponse struct {
	CommitInfo       *CommitInfo
	Message          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// CommitAgentOptions contains configuration for CommitAgent
type CommitAgentOptions struct {
	Language    string
	GitExecutor git.Executor
	LLMProvider llm.Provider
	Printer     *ui.StreamPrinter
	Output      io.Writer
	Debug       bool
}

// Validate validates the options and sets defaults
func (o *CommitAgentOptions) Validate() error {
	if o.Language == "" {
		o.Language = "en"
	}
	if o.LLMProvider == nil {
		return fmt.Errorf("LLM provider is required")
	}
	if o.GitExecutor == nil {
		return fmt.Errorf("Git executor is required")
	}
	return nil
}

func (o *CommitAgentOptions) getPrinter() *ui.StreamPrinter {
	return o.Printer
}

// CommitAgent generates commit messages using LLM
type CommitAgent struct {
	opts CommitAgentOptions
}

// NewCommitAgent creates a new CommitAgent
func NewCommitAgent(opts CommitAgentOptions) (*CommitAgent, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	return &CommitAgent{opts: opts}, nil
}

// BuildSystemPrompt builds the system prompt for commit generation
func BuildSystemPrompt(language, context string) string {
	tmpl, err := template.New("commit_prompt").Parse(CommitSystemPrompt)
	if err != nil {
		return CommitSystemPrompt
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Language": language,
		"Context":  context,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return CommitSystemPrompt
	}
	return buf.String()
}

// GenerateCommitMessage generates a commit message using agent loop
func (a *CommitAgent) GenerateCommitMessage(ctx context.Context, req CommitRequest) (*CommitResponse, error) {
	printer := a.opts.getPrinter()

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
	estimateTokenCount := func(text string) int {
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
	gitStatusTool := tools.NewGitStatusTool(a.opts.GitExecutor)
	gitDiffCachedTool := tools.NewGitDiffCachedTool(a.opts.GitExecutor)
	gitLogTool := tools.NewGitLogTool(a.opts.GitExecutor)

	// Define tool schemas
	toolInfos := []*schema.ToolInfo{
		{
			Name:        "git_status",
			Desc:        gitStatusTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		},
		{
			Name:        "git_diff_cached",
			Desc:        gitDiffCachedTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		},
		{
			Name: "git_log",
			Desc: gitLogTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"count": {Type: schema.Integer, Desc: "Number of commits to retrieve (default 5)", Required: false},
			}),
		},
		{
			Name: "submit_commit",
			Desc: "Submit the structured commit information. Call this when you have analyzed the changes and are ready to generate the commit message.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"type":        {Type: schema.String, Desc: "Commit type: feat, fix, docs, style, refactor, perf, test, chore, build, ci, or revert", Required: true},
				"scope":       {Type: schema.String, Desc: "Commit scope (optional)", Required: false},
				"description": {Type: schema.String, Desc: "Short description (max 50 chars preferred)", Required: true},
				"body":        {Type: schema.String, Desc: "Detailed description (optional)", Required: false},
				"footer":      {Type: schema.String, Desc: "Footer for breaking changes or issue references (optional)", Required: false},
			}),
		},
	}

	// Bind tools to chat model
	if err := chatModel.BindTools(toolInfos); err != nil {
		return nil, fmt.Errorf("failed to bind tools: %w", err)
	}

	// Build system prompt
	systemPrompt := BuildSystemPrompt(req.Language, req.Context)
	printInfo(fmt.Sprintf("Language: %s", req.Language))
	if req.Context != "" {
		printInfo(fmt.Sprintf("Context: %s", req.Context))
	}

	// Initial messages
	userMsg := "Please generate a commit message for the staged changes. Use the available tools to analyze the changes first."

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userMsg},
	}

	var promptTokens, completionTokens, totalTokens int
	maxIterations := 10

	// Agent loop
	for i := 0; i < maxIterations; i++ {
		printProgress(fmt.Sprintf("Agent iteration %d...", i+1))

		// Stream LLM response
		streamReader, err := chatModel.Stream(ctx, messages)
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

		// Process tool calls
		if len(toolCalls) == 0 {
			return nil, fmt.Errorf("LLM did not call any tools")
		}

		for _, tc := range toolCalls {
			if tc.Function.Name == "" {
				continue
			}

			// Check if it's the final submit_commit call
			if tc.Function.Name == "submit_commit" {
				var params tools.SubmitCommitParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse submit_commit arguments: %v", err)
					continue
				}

				if err := params.Validate(); err != nil {
					log.Debug("Invalid commit params: %v", err)
					continue
				}

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

			// Execute other tools
			var result string
			var toolErr error

			switch tc.Function.Name {
			case "git_status":
				result, toolErr = gitStatusTool.Execute(ctx, nil)

			case "git_diff_cached":
				result, toolErr = gitDiffCachedTool.Execute(ctx, nil)
				// Check if result starts with "No staged changes" (not just contains)
				// This prevents false positives when the diff itself contains this string
				if toolErr == nil && strings.HasPrefix(result, "No staged changes") {
					return nil, fmt.Errorf("no staged changes found")
				}

			case "git_log":
				var params tools.GitLogParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					// Use default params if parsing fails
					params = tools.GitLogParams{Count: 5}
				}
				result, toolErr = gitLogTool.Execute(ctx, &params)

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
