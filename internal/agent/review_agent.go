package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/huimingz/gitbuddy-go/internal/agent/session"
	"github.com/huimingz/gitbuddy-go/internal/agent/tools"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// Severity levels for review issues
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// Issue categories
const (
	CategoryBug         = "bug"
	CategorySecurity    = "security"
	CategoryPerformance = "performance"
	CategoryStyle       = "style"
	CategorySuggestion  = "suggestion"
)

// ReviewRequest contains the input for code review
type ReviewRequest struct {
	Language              string           // Output language
	Context               string           // Additional context from user
	Files                 []string         // Specific files to review (empty = all staged)
	Severity              string           // Minimum severity filter (error, warning, info)
	Focus                 []string         // Focus areas (security, performance, style)
	WorkDir               string           // Working directory
	MaxLines              int              // Maximum lines per file read
	Session               *session.Session // Optional session to resume from
	PreGeneratedSessionID string           // Optional pre-generated session ID
}

// ReviewIssue represents a single issue found during review
type ReviewIssue struct {
	Severity    string `json:"severity"`    // error, warning, info
	Category    string `json:"category"`    // bug, security, performance, style, suggestion
	File        string `json:"file"`        // File path
	Line        int    `json:"line"`        // Line number (0 if not applicable)
	Title       string `json:"title"`       // Brief title
	Description string `json:"description"` // Detailed explanation
	Suggestion  string `json:"suggestion"`  // How to fix (optional)
}

// ReviewResponse contains the result of code review
type ReviewResponse struct {
	Issues           []ReviewIssue
	Summary          string
	SessionID        string // Session ID for resuming
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// GetIssues returns the review issues
func (r *ReviewResponse) GetIssues() []ReviewIssue {
	return r.Issues
}

// GetSummary returns the review summary
func (r *ReviewResponse) GetSummary() string {
	return r.Summary
}

// ReviewAgentOptions contains configuration for ReviewAgent
type ReviewAgentOptions struct {
	Language        string
	GitExecutor     git.Executor
	LLMProvider     llm.Provider
	Printer         *ui.StreamPrinter
	Output          io.Writer
	Debug           bool
	WorkDir         string
	MaxLinesPerRead int
	RetryConfig     llm.RetryConfig
	SessionManager  *session.Manager
}

// ReviewAgent performs code review using LLM
type ReviewAgent struct {
	opts ReviewAgentOptions
}

// NewReviewAgent creates a new ReviewAgent
func NewReviewAgent(opts ReviewAgentOptions) *ReviewAgent {
	if opts.Language == "" {
		opts.Language = "en"
	}
	if opts.MaxLinesPerRead <= 0 {
		opts.MaxLinesPerRead = tools.DefaultMaxLinesPerRead
	}
	return &ReviewAgent{opts: opts}
}

// SubmitReviewParams represents the review result from LLM
type SubmitReviewParams struct {
	Issues  []ReviewIssue `json:"issues"`
	Summary string        `json:"summary"`
}

// BuildReviewSystemPrompt builds the system prompt for review
func BuildReviewSystemPrompt(language, context, files, focus, minSeverity string) string {
	tmpl, err := template.New("review_prompt").Parse(ReviewSystemPrompt)
	if err != nil {
		return ReviewSystemPrompt
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Language":    language,
		"Context":     context,
		"Files":       files,
		"Focus":       focus,
		"MinSeverity": minSeverity,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return ReviewSystemPrompt
	}
	return buf.String()
}

// Review performs code review on staged changes
func (a *ReviewAgent) Review(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
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

	// Create tools
	gitDiffCachedTool := tools.NewGitDiffCachedTool(a.opts.GitExecutor)
	gitStatusTool := tools.NewGitStatusTool(a.opts.GitExecutor)

	maxLines := req.MaxLines
	if maxLines <= 0 {
		maxLines = a.opts.MaxLinesPerRead
	}
	readFileTool := tools.NewReadFileTool(req.WorkDir, maxLines)

	// Create grep tools
	grepFileTool := tools.NewGrepFileTool(req.WorkDir, tools.DefaultMaxFileSize)
	grepDirectoryTool := tools.NewGrepDirectoryTool(req.WorkDir, tools.DefaultMaxFileSize, tools.DefaultMaxResults, tools.DefaultGrepTimeout)

	// Define tool schemas
	toolInfos := []*schema.ToolInfo{
		{
			Name:        "git_diff_cached",
			Desc:        gitDiffCachedTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		},
		{
			Name:        "git_status",
			Desc:        gitStatusTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		},
		{
			Name: "read_file",
			Desc: readFileTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"file_path":  {Type: schema.String, Desc: "Path to the file to read", Required: true},
				"start_line": {Type: schema.Integer, Desc: "Starting line number (1-indexed)", Required: false},
				"end_line":   {Type: schema.Integer, Desc: "Ending line number (1-indexed, inclusive)", Required: false},
			}),
		},
		{
			Name: "grep_file",
			Desc: grepFileTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"file_path":      {Type: schema.String, Desc: "Path to the file to search", Required: true},
				"pattern":        {Type: schema.String, Desc: "Regular expression pattern to search for", Required: true},
				"ignore_case":    {Type: schema.Boolean, Desc: "Perform case-insensitive search", Required: false},
				"before_context": {Type: schema.Integer, Desc: "Number of lines to show before each match", Required: false},
				"after_context":  {Type: schema.Integer, Desc: "Number of lines to show after each match", Required: false},
				"context":        {Type: schema.Integer, Desc: "Number of lines to show before and after each match", Required: false},
			}),
		},
		{
			Name: "grep_directory",
			Desc: grepDirectoryTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"directory":      {Type: schema.String, Desc: "Path to the directory to search", Required: true},
				"pattern":        {Type: schema.String, Desc: "Regular expression pattern to search for", Required: true},
				"recursive":      {Type: schema.Boolean, Desc: "Search subdirectories recursively", Required: false},
				"file_pattern":   {Type: schema.String, Desc: "Glob pattern to filter files (e.g., '*.go')", Required: false},
				"ignore_case":    {Type: schema.Boolean, Desc: "Perform case-insensitive search", Required: false},
				"before_context": {Type: schema.Integer, Desc: "Number of lines to show before each match", Required: false},
				"after_context":  {Type: schema.Integer, Desc: "Number of lines to show after each match", Required: false},
				"context":        {Type: schema.Integer, Desc: "Number of lines to show before and after each match", Required: false},
				"max_results":    {Type: schema.Integer, Desc: "Maximum number of matches to return", Required: false},
			}),
		},
		{
			Name: "submit_review",
			Desc: "Submit the code review findings. Call this when you have analyzed the changes and are ready to submit your review.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"issues":  {Type: schema.Array, Desc: "Array of issues found during review", Required: true},
				"summary": {Type: schema.String, Desc: "Brief overall summary of the review", Required: true},
			}),
		},
	}

	// Bind tools to chat model
	if err := chatModel.BindTools(toolInfos); err != nil {
		return nil, fmt.Errorf("failed to bind tools: %w", err)
	}

	// Format request parameters for prompt
	filesStr := ""
	if len(req.Files) > 0 {
		filesStr = strings.Join(req.Files, ", ")
	}

	focusStr := ""
	if len(req.Focus) > 0 {
		focusStr = strings.Join(req.Focus, ", ")
	}

	// Build system prompt
	systemPrompt := BuildReviewSystemPrompt(req.Language, req.Context, filesStr, focusStr, req.Severity)
	printInfo("Starting code review...")

	// Initial messages
	userMessage := "Please review the staged code changes and provide your findings."
	if len(req.Files) > 0 {
		userMessage = fmt.Sprintf("Please review the staged changes in these files: %s", filesStr)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userMessage},
	}

	var promptTokens, completionTokens, totalTokens int
	maxIterations := 15 // Allow more iterations for thorough review

	// Initialize session management
	var currentSession *session.Session
	var sessionID string

	if req.Session != nil {
		// Resume from existing session
		currentSession = req.Session
		sessionID = currentSession.ID

		// Use messages from session if available
		if len(currentSession.Messages) > 0 {
			messages = currentSession.Messages
			printProgress(fmt.Sprintf("Restored %d messages from session", len(messages)))
		}

		promptTokens = currentSession.TokenUsage.PromptTokens
		completionTokens = currentSession.TokenUsage.CompletionTokens
		totalTokens = currentSession.TokenUsage.TotalTokens

		printProgress(fmt.Sprintf("Resumed session %s", sessionID))
	} else {
		// Create new session
		if req.PreGeneratedSessionID != "" {
			sessionID = req.PreGeneratedSessionID
		} else {
			sessionID = session.GenerateSessionID("review")
		}
		currentSession = &session.Session{
			ID:             sessionID,
			AgentType:      "review",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			Messages:       make([]*schema.Message, 0),
			IterationCount: 0,
			MaxIterations:  maxIterations,
			TokenUsage:     session.TokenUsage{},
			Metadata:       make(map[string]string),
		}

		// Store request as JSON
		reqBytes, err := json.Marshal(req)
		if err != nil {
			log.Debug("Failed to marshal review request: %v", err)
		} else {
			currentSession.Request = reqBytes
		}

		printProgress(fmt.Sprintf("Created new session %s", sessionID))
	}

	// Agent loop
	for i := 0; i < maxIterations; i++ {
		// Check if context was cancelled (e.g., due to Ctrl+C)
		select {
		case <-ctx.Done():
			printProgress("Review execution cancelled by user")
			// Save current session state before returning
			if a.opts.SessionManager != nil && currentSession != nil {
				currentSession.Messages = messages
				currentSession.IterationCount = i + 1
				currentSession.MaxIterations = maxIterations
				currentSession.TokenUsage = session.TokenUsage{
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				}

				// Save session on cancellation
				if err := a.opts.SessionManager.Save(currentSession); err != nil {
					log.Debug("Failed to save session on cancellation: %v", err)
				} else {
					log.Debug("Session %s saved on cancellation", sessionID)
				}
			}
			return &ReviewResponse{
				Issues:           []ReviewIssue{},
				Summary:          "Review session was cancelled by user",
				SessionID:        sessionID,
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			}, ctx.Err()
		default:
			// Continue with normal execution
		}

		printProgress(fmt.Sprintf("Agent iteration %d...", i+1))

		// Stream LLM response
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

		// Process tool calls - use intelligent fallback if no tools called
		if len(toolCalls) == 0 {
			if err := HandleNoToolCallsResponse(fullContent.String(), "review"); err != nil {
				return nil, err
			}
			// If we reach here, the response was accepted without tools
			// For review agent, we could potentially accept high-quality direct analysis,
			// but we still need structured review data, so we should return error
			return nil, fmt.Errorf("review agent requires tool usage to examine code and provide thorough analysis")
		}

		for _, tc := range toolCalls {
			if tc.Function.Name == "" {
				continue
			}

			// Check if it's the final submit_review call
			if tc.Function.Name == "submit_review" {
				var params SubmitReviewParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse submit_review arguments: %v", err)
					continue
				}

				// Filter issues by severity if specified
				filteredIssues := filterIssuesBySeverity(params.Issues, req.Severity)

				printSuccess("Code review completed successfully")

				// Save final session state
				if a.opts.SessionManager != nil && currentSession != nil {
					currentSession.Messages = messages
					currentSession.IterationCount = i + 1
					currentSession.MaxIterations = maxIterations
					currentSession.TokenUsage = session.TokenUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      totalTokens,
					}

					// Save final session
					if err := a.opts.SessionManager.Save(currentSession); err != nil {
						log.Debug("Failed to save final session: %v", err)
					} else {
						log.Debug("Final session %s saved", sessionID)
					}
				}

				return &ReviewResponse{
					Issues:           filteredIssues,
					Summary:          params.Summary,
					SessionID:        sessionID,
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				}, nil
			}

			// Execute other tools
			var result string
			var toolErr error

			switch tc.Function.Name {
			case "git_diff_cached":
				result, toolErr = gitDiffCachedTool.Execute(ctx, nil)

			case "git_status":
				result, toolErr = gitStatusTool.Execute(ctx, nil)

			case "read_file":
				var params tools.ReadFileParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = readFileTool.Execute(ctx, &params)
				}

			case "grep_file":
				var params tools.GrepFileParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = grepFileTool.Execute(ctx, &params)
				}

			case "grep_directory":
				var params tools.GrepDirectoryParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = grepDirectoryTool.Execute(ctx, &params)
				}

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

// filterIssuesBySeverity filters issues based on minimum severity level
func filterIssuesBySeverity(issues []ReviewIssue, minSeverity string) []ReviewIssue {
	if minSeverity == "" || minSeverity == SeverityInfo {
		return issues
	}

	severityOrder := map[string]int{
		SeverityInfo:    0,
		SeverityWarning: 1,
		SeverityError:   2,
	}

	minLevel, ok := severityOrder[minSeverity]
	if !ok {
		return issues
	}

	filtered := make([]ReviewIssue, 0)
	for _, issue := range issues {
		issueLevel, ok := severityOrder[issue.Severity]
		if ok && issueLevel >= minLevel {
			filtered = append(filtered, issue)
		}
	}

	return filtered
}
