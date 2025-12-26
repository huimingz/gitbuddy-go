package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/cloudwego/eino/schema"

	"github.com/huimingz/gitbuddy-go/internal/agent/tools"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// DebugRequest contains the input for debugging
type DebugRequest struct {
	Issue                 string   // Issue description from user
	Language              string   // Output language
	Context               string   // Additional context
	Files                 []string // Specific files to investigate
	WorkDir               string   // Working directory
	IssuesDir             string   // Directory to save reports
	MaxLines              int      // Maximum lines per file read
	MaxIterations         int      // Maximum number of agent iterations
	Interactive           bool     // Enable interactive feedback
	EnableCompression     bool     // Enable message history compression
	CompressionThreshold  int      // Number of messages before compression
	CompressionKeepRecent int      // Number of recent messages to keep after compression
}

// DebugResponse contains the result of debugging
type DebugResponse struct {
	Report           string
	FilePath         string // Path to saved report file
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// DebugAgentOptions contains configuration for DebugAgent
type DebugAgentOptions struct {
	Language        string
	GitExecutor     git.Executor
	LLMProvider     llm.Provider
	Printer         *ui.StreamPrinter
	Output          io.Writer
	Input           io.Reader
	Debug           bool
	WorkDir         string
	IssuesDir       string
	MaxLinesPerRead int
}

// DebugAgent performs code debugging using LLM
type DebugAgent struct {
	opts DebugAgentOptions
}

// NewDebugAgent creates a new DebugAgent
func NewDebugAgent(opts DebugAgentOptions) *DebugAgent {
	if opts.Language == "" {
		opts.Language = "en"
	}
	if opts.MaxLinesPerRead <= 0 {
		opts.MaxLinesPerRead = tools.DefaultMaxLinesPerRead
	}
	if opts.IssuesDir == "" {
		opts.IssuesDir = "./issues"
	}
	if opts.Input == nil {
		opts.Input = os.Stdin
	}
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	return &DebugAgent{opts: opts}
}

// BuildDebugSystemPrompt builds the system prompt for debugging
func BuildDebugSystemPrompt(language, context, issue, files string) string {
	tmpl, err := template.New("debug_prompt").Parse(DebugSystemPrompt)
	if err != nil {
		return DebugSystemPrompt
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Language": language,
		"Context":  context,
		"Issue":    issue,
		"Files":    files,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return DebugSystemPrompt
	}
	return buf.String()
}

// Debug performs interactive debugging
func (a *DebugAgent) Debug(ctx context.Context, req DebugRequest) (*DebugResponse, error) {
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
	workDir := req.WorkDir
	if workDir == "" {
		workDir = a.opts.WorkDir
	}

	issuesDir := req.IssuesDir
	if issuesDir == "" {
		issuesDir = a.opts.IssuesDir
	}

	maxLines := req.MaxLines
	if maxLines <= 0 {
		maxLines = a.opts.MaxLinesPerRead
	}

	// File system tools
	listDirectoryTool := tools.NewListDirectoryTool(workDir)
	listFilesTool := tools.NewListFilesTool(workDir, tools.DefaultMaxFiles)
	readFileTool := tools.NewReadFileTool(workDir, maxLines)

	// Search tools
	grepFileTool := tools.NewGrepFileTool(workDir, tools.DefaultMaxFileSize)
	grepDirectoryTool := tools.NewGrepDirectoryTool(workDir, tools.DefaultMaxFileSize, tools.DefaultMaxResults, tools.DefaultGrepTimeout)

	// Git tools
	gitStatusTool := tools.NewGitStatusTool(a.opts.GitExecutor)
	gitDiffCachedTool := tools.NewGitDiffCachedTool(a.opts.GitExecutor)
	gitLogTool := tools.NewGitLogTool(a.opts.GitExecutor)
	gitShowTool := tools.NewGitShowTool(a.opts.GitExecutor)

	// Interactive and reporting tools
	requestFeedbackTool := tools.NewRequestFeedbackTool(a.opts.Input, a.opts.Output)
	submitReportTool := tools.NewSubmitReportTool(issuesDir)

	// Define tool schemas
	toolInfos := []*schema.ToolInfo{
		{
			Name: "list_directory",
			Desc: listDirectoryTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"path":        {Type: schema.String, Desc: "Directory path to list", Required: true},
				"show_hidden": {Type: schema.Boolean, Desc: "Show hidden files", Required: false},
				"recursive":   {Type: schema.Boolean, Desc: "List subdirectories recursively", Required: false},
				"max_depth":   {Type: schema.Integer, Desc: "Maximum depth for recursive listing", Required: false},
			}),
		},
		{
			Name: "list_files",
			Desc: listFilesTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"pattern":      {Type: schema.String, Desc: "Glob pattern to match files (e.g., '*.go', '**/*.py')", Required: true},
				"path":         {Type: schema.String, Desc: "Base path to search from", Required: true},
				"exclude_dirs": {Type: schema.Array, Desc: "Directories to exclude (e.g., ['node_modules', '.git'])", Required: false},
				"max_results":  {Type: schema.Integer, Desc: "Maximum number of results", Required: false},
			}),
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
				"max_count": {Type: schema.Integer, Desc: "Maximum number of commits to show", Required: false},
				"since":     {Type: schema.String, Desc: "Show commits more recent than a specific date", Required: false},
			}),
		},
		{
			Name: "git_show",
			Desc: gitShowTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"commit": {Type: schema.String, Desc: "Commit hash or reference to show", Required: true},
			}),
		},
		{
			Name: "submit_report",
			Desc: submitReportTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"title":   {Type: schema.String, Desc: "Report title", Required: true},
				"content": {Type: schema.String, Desc: "Full report content in markdown format", Required: true},
			}),
		},
	}

	// Add request_feedback tool only if interactive mode is enabled
	if req.Interactive {
		toolInfos = append(toolInfos, &schema.ToolInfo{
			Name: "request_feedback",
			Desc: requestFeedbackTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"question": {Type: schema.String, Desc: "Question to ask the user", Required: true},
				"options":  {Type: schema.Array, Desc: "Array of options for the user to choose from", Required: true},
				"context":  {Type: schema.String, Desc: "Additional context for the question", Required: false},
			}),
		})
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

	// Build system prompt
	systemPrompt := BuildDebugSystemPrompt(req.Language, req.Context, req.Issue, filesStr)
	printInfo("Starting debugging session...")

	// Initial messages
	userMessage := fmt.Sprintf("Please help me debug this issue: %s", req.Issue)
	if len(req.Files) > 0 {
		userMessage += fmt.Sprintf("\n\nFocus on these files: %s", filesStr)
	}
	if req.Context != "" {
		userMessage += fmt.Sprintf("\n\nAdditional context: %s", req.Context)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userMessage},
	}

	var promptTokens, completionTokens, totalTokens int

	// Use configured max iterations, default to 30 if not set
	maxIterations := req.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30
	}

	// Agent loop
	iterationCount := 0
	for {
		iterationCount++

		// Check if we've exceeded max iterations
		if iterationCount > maxIterations {
			printProgress(fmt.Sprintf("Reached maximum iterations (%d)", maxIterations))

			// Ask user if they want to continue (only in interactive mode)
			if req.Interactive {
				fmt.Fprintf(a.opts.Output, "\n")
				shouldContinue, err := ui.ConfirmWithDefault("Continue debugging for another batch of iterations?", false, a.opts.Input, a.opts.Output)
				if err != nil || !shouldContinue {
					return nil, fmt.Errorf("debugging stopped after %d iterations", iterationCount-1)
				}
				// Reset max iterations for another batch
				maxIterations = iterationCount + 30
				printProgress("Continuing debugging session...")
			} else {
				return nil, fmt.Errorf("agent loop exceeded maximum iterations (%d)", maxIterations)
			}
		}

		printProgress(fmt.Sprintf("Agent iteration %d...", iterationCount))

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

			// Check if it's the final submit_report call
			if tc.Function.Name == "submit_report" {
				var params tools.SubmitReportParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse submit_report arguments: %v", err)
					continue
				}

				// Execute submit_report to save the report
				result, err := submitReportTool.Execute(ctx, &params)
				if err != nil {
					return nil, fmt.Errorf("failed to submit report: %w", err)
				}

				// Parse result to get file path
				var reportResult tools.DebugReport
				if err := json.Unmarshal([]byte(result), &reportResult); err != nil {
					log.Debug("Failed to parse report result: %v", err)
				}

				printSuccess("Debugging session completed successfully")

				return &DebugResponse{
					Report:           params.Content,
					FilePath:         reportResult.FilePath,
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				}, nil
			}

			// Execute other tools
			var result string
			var toolErr error

			switch tc.Function.Name {
			case "list_directory":
				var params tools.ListDirectoryParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = listDirectoryTool.Execute(ctx, &params)
				}

			case "list_files":
				var params tools.ListFilesParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = listFilesTool.Execute(ctx, &params)
				}

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

			case "git_status":
				result, toolErr = gitStatusTool.Execute(ctx, nil)

			case "git_diff_cached":
				result, toolErr = gitDiffCachedTool.Execute(ctx, nil)

			case "git_log":
				var params tools.GitLogParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = gitLogTool.Execute(ctx, &params)
				}

			case "git_show":
				var params tools.GitShowParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = gitShowTool.Execute(ctx, &params)
				}

			case "request_feedback":
				if !req.Interactive {
					toolErr = fmt.Errorf("interactive mode is not enabled")
				} else {
					var params tools.RequestFeedbackParams
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
						toolErr = fmt.Errorf("invalid parameters: %w", err)
					} else {
						result, toolErr = requestFeedbackTool.Execute(ctx, &params)
					}
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

		// Compress message history if enabled and threshold is reached
		if req.EnableCompression && len(messages) > req.CompressionThreshold {
			compressedMessages, err := compressMessageHistoryWithLLM(ctx, chatModel, messages, req.CompressionKeepRecent)
			if err != nil {
				log.Debug("Failed to compress message history with LLM: %v", err)
				// Fallback to simple compression if LLM compression fails
				messages = simpleCompressMessageHistory(messages, req.CompressionKeepRecent)
			} else {
				messages = compressedMessages
			}
			printProgress(fmt.Sprintf("Message history compressed (%d -> %d messages)", len(messages), len(compressedMessages)))
		}
	}

	return nil, fmt.Errorf("agent loop exceeded maximum iterations")
}

// compressMessageHistoryWithLLM uses LLM to intelligently compress old message history
// while preserving key information and keeping recent messages intact
// chatModel parameter should be the same model.ChatModel returned by CreateChatModel
func compressMessageHistoryWithLLM(ctx context.Context, chatModel interface{}, messages []*schema.Message, keepLastN int) ([]*schema.Message, error) {
	if len(messages) <= keepLastN+1 { // +1 for system message
		return messages, nil
	}

	// Structure: [system, ...old messages to compress..., ...recent messages to keep...]
	systemMsg := messages[0]
	oldMessages := messages[1 : len(messages)-keepLastN]
	recentMessages := messages[len(messages)-keepLastN:]

	// Build a summary request for the old messages
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("Please summarize the following debugging session history. ")
	summaryBuilder.WriteString("Focus on:\n")
	summaryBuilder.WriteString("1. Key findings and observations\n")
	summaryBuilder.WriteString("2. Important tool results and their implications\n")
	summaryBuilder.WriteString("3. Decisions made and reasoning\n")
	summaryBuilder.WriteString("4. Current understanding of the issue\n\n")
	summaryBuilder.WriteString("Keep the summary concise but preserve all critical information.\n\n")
	summaryBuilder.WriteString("History to summarize:\n---\n")

	// Format old messages for summarization
	for _, msg := range oldMessages {
		switch msg.Role {
		case schema.User:
			summaryBuilder.WriteString(fmt.Sprintf("USER: %s\n", msg.Content))
		case schema.Assistant:
			summaryBuilder.WriteString(fmt.Sprintf("ASSISTANT: %s\n", msg.Content))
			if len(msg.ToolCalls) > 0 {
				summaryBuilder.WriteString("  Tool calls: ")
				toolNames := make([]string, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					toolNames = append(toolNames, tc.Function.Name)
				}
				summaryBuilder.WriteString(strings.Join(toolNames, ", "))
				summaryBuilder.WriteString("\n")
			}
		case schema.Tool:
			// Truncate long tool results
			content := msg.Content
			if len(content) > 500 {
				content = content[:500] + "... (truncated)"
			}
			summaryBuilder.WriteString(fmt.Sprintf("TOOL RESULT: %s\n", content))
		}
	}
	summaryBuilder.WriteString("---\n")

	// Call LLM to generate summary - use dynamic type to avoid import issues
	summaryMessages := []*schema.Message{
		{
			Role:    schema.User,
			Content: summaryBuilder.String(),
		},
	}

	// Use reflection to call Stream method dynamically
	streamMethod := reflect.ValueOf(chatModel).MethodByName("Stream")
	if !streamMethod.IsValid() {
		return nil, fmt.Errorf("chat model does not have Stream method")
	}

	results := streamMethod.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(summaryMessages),
	})

	if len(results) != 2 {
		return nil, fmt.Errorf("unexpected Stream method signature")
	}

	// Check for error
	if !results[1].IsNil() {
		return nil, fmt.Errorf("failed to generate summary: %w", results[1].Interface().(error))
	}

	streamReader := results[0].Interface()

	// Close the stream reader when done
	defer func() {
		closeMethod := reflect.ValueOf(streamReader).MethodByName("Close")
		if closeMethod.IsValid() {
			closeMethod.Call(nil)
		}
	}()

	// Collect the summary from stream using reflection
	var summary strings.Builder
	recvMethod := reflect.ValueOf(streamReader).MethodByName("Recv")
	if !recvMethod.IsValid() {
		return nil, fmt.Errorf("stream reader does not have Recv method")
	}

	for {
		results := recvMethod.Call(nil)
		if len(results) != 2 {
			return nil, fmt.Errorf("unexpected Recv method signature")
		}

		// Check for error
		if !results[1].IsNil() {
			err := results[1].Interface().(error)
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("stream read error: %w", err)
		}

		// Extract content from chunk
		chunk := results[0]

		// If chunk is a pointer, dereference it
		if chunk.Kind() == reflect.Ptr {
			if chunk.IsNil() {
				continue
			}
			chunk = chunk.Elem()
		}

		// Try to get Content field
		if chunk.Kind() == reflect.Struct {
			contentField := chunk.FieldByName("Content")
			if contentField.IsValid() && contentField.Kind() == reflect.String {
				if content := contentField.String(); content != "" {
					summary.WriteString(content)
				}
			}
		}
	}

	summaryText := summary.String()
	if summaryText == "" {
		return nil, fmt.Errorf("empty summary generated")
	}

	// Build compressed message history
	compressed := []*schema.Message{
		systemMsg,
		{
			Role:    schema.User,
			Content: fmt.Sprintf("[Previous Session Summary]\n%s\n\n[Continuing from here...]", summaryText),
		},
	}
	compressed = append(compressed, recentMessages...)

	log.Debug("Compressed %d messages into summary, keeping %d recent messages", len(oldMessages), len(recentMessages))
	return compressed, nil
}

// simpleCompressMessageHistory is a fallback that truncates old messages
// but adds a summary message to preserve context
func simpleCompressMessageHistory(messages []*schema.Message, keepLastN int) []*schema.Message {
	if len(messages) <= keepLastN+1 {
		return messages
	}

	// Structure: [system, ...old messages..., ...recent messages...]
	systemMsg := messages[0]
	oldMessages := messages[1 : len(messages)-keepLastN]
	recentMessages := messages[len(messages)-keepLastN:]

	// Build a simple text summary of old messages
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString(fmt.Sprintf("[Note: %d earlier messages were compressed for context management]\n\n", len(oldMessages)))
	summaryBuilder.WriteString("Summary of earlier investigation:\n")

	// Extract key information from old messages
	toolCallCount := 0
	var toolsUsed []string
	toolUsageMap := make(map[string]int)

	for _, msg := range oldMessages {
		if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				toolName := tc.Function.Name
				toolUsageMap[toolName]++
				toolCallCount++
			}
		}
	}

	// List tools used
	for tool, count := range toolUsageMap {
		toolsUsed = append(toolsUsed, fmt.Sprintf("%s (%d times)", tool, count))
	}

	if len(toolsUsed) > 0 {
		summaryBuilder.WriteString(fmt.Sprintf("- Tools used: %s\n", strings.Join(toolsUsed, ", ")))
		summaryBuilder.WriteString(fmt.Sprintf("- Total tool calls: %d\n", toolCallCount))
	}

	summaryBuilder.WriteString("\nContinuing from the most recent context...\n")

	// Build compressed message history
	compressed := []*schema.Message{
		systemMsg,
		{
			Role:    schema.User,
			Content: summaryBuilder.String(),
		},
	}
	compressed = append(compressed, recentMessages...)

	log.Debug("Simple compression: %d messages -> %d messages (kept %d recent)", len(messages), len(compressed), len(recentMessages))
	return compressed
}
