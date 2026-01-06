package agent

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/huimingz/gitbuddy-go/internal/agent/session"
	"github.com/huimingz/gitbuddy-go/internal/agent/tools"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// ChatRequest contains the input for chat command
type ChatRequest struct {
	Query                 string             // User query or message
	Language              string             // Output language
	WorkDir               string             // Working directory
	MaxIterations         int                // Maximum number of agent iterations
	EnableCompression     bool               // Enable message history compression
	CompressionThreshold  int                // Number of messages before compression
	CompressionKeepRecent int                // Number of recent messages to keep after compression
	Session               *session.Session   // Optional session to resume from
	PreGeneratedSessionID string             // Optional pre-generated session ID
	OnStreamChunk         func(chunk string) // Callback for streaming chunks
}

// ChatResponse contains the result of chat
type ChatResponse struct {
	Response         string // AI response text
	MessageCount     int    // Number of messages in conversation
	IterationCount   int    // Number of agent iterations
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	SessionID        string // Session ID for resuming
}

// ChatAgentOptions contains configuration for ChatAgent
type ChatAgentOptions struct {
	Language        string
	GitExecutor     git.Executor
	LLMProvider     llm.Provider
	Printer         *ui.StreamPrinter
	Output          io.Writer
	Input           io.Reader
	Debug           bool
	WorkDir         string
	MaxLinesPerRead int
	RetryConfig     llm.RetryConfig
	SessionManager  *session.Manager
}

// ChatAgent is an AI agent for interactive chat with tool support
type ChatAgent struct {
	options       ChatAgentOptions
	messages      []*schema.Message
	toolInstances map[string]interface{}
}

// NewChatAgent creates a new ChatAgent
func NewChatAgent(options ChatAgentOptions) *ChatAgent {
	return &ChatAgent{
		options:       options,
		messages:      []*schema.Message{},
		toolInstances: make(map[string]interface{}),
	}
}

// Chat performs a chat interaction with the AI agent
// It uses the LLM provider to generate responses and can invoke tools
func (a *ChatAgent) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("chat request is required")
	}

	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Initialize message history from session if resuming
	if req.Session != nil && len(req.Session.Messages) > 0 {
		a.messages = make([]*schema.Message, 0, len(req.Session.Messages))
		for _, msg := range req.Session.Messages {
			a.messages = append(a.messages, &schema.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	} else {
		a.messages = []*schema.Message{
			{
				Role:    schema.System,
				Content: a.getSystemPrompt(req.Language),
			},
		}
	}

	// Create or get LLM provider
	chatModel, err := a.options.LLMProvider.CreateChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	// Initialize tools
	if err := a.initializeTools(ctx, req.WorkDir); err != nil {
		return nil, fmt.Errorf("failed to initialize tools: %w", err)
	}

	// Build tool infos for the chat model
	toolInfos := a.buildToolInfos()

	// Bind tools to chat model
	if err := chatModel.BindTools(toolInfos); err != nil {
		return nil, fmt.Errorf("failed to bind tools: %w", err)
	}

	// Set max iterations
	maxIterations := req.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	// Add user query to messages
	a.messages = append(a.messages, &schema.Message{
		Role:    schema.User,
		Content: req.Query,
	})

	// Run the chat loop
	var iterationCount int
	var promptTokens, completionTokens, totalTokens int

	for iterationCount = 0; iterationCount < maxIterations; iterationCount++ {
		// Stream LLM response
		streamReader, err := chatModel.Stream(ctx, a.messages)
		if err != nil {
			return nil, fmt.Errorf("failed to stream response: %w", err)
		}

		if streamReader == nil {
			break
		}

		// Collect the response by accumulating all content chunks
		var responseContent strings.Builder
		var response *schema.Message

		for {
			msg, err := streamReader.Recv()
			if err != nil {
				break
			}
			if msg != nil {
				response = msg
				if msg.Content != "" {
					// Append content to accumulate all chunks
					responseContent.WriteString(msg.Content)
					// Call streaming callback if provided
					if req.OnStreamChunk != nil {
						req.OnStreamChunk(msg.Content)
					}
				}
			}
		}

		// Close the stream
		streamReader.Close()

		// Create complete response message with accumulated content
		if response != nil && responseContent.Len() > 0 {
			response.Content = responseContent.String()
			a.messages = append(a.messages, response)
		}

		// If there are no tool calls, the agent has finished
		if response == nil || len(response.ToolCalls) == 0 {
			break
		}

		// Process tool calls (for now, just add them to messages)
		// Actual tool execution would happen here
		for _, toolCall := range response.ToolCalls {
			a.messages = append(a.messages, &schema.Message{
				Role:       schema.Tool,
				Content:    "Tool execution result",
				ToolCallID: toolCall.ID,
			})
		}
	}

	// Compress message history if needed
	if req.EnableCompression && len(a.messages) > req.CompressionThreshold {
		a.compressMessages(req.CompressionKeepRecent)
	}

	// Save session if session manager is available
	sessionID := req.PreGeneratedSessionID
	if a.options.SessionManager != nil && sessionID != "" {
		sess := &session.Session{
			ID:             sessionID,
			AgentType:      "chat",
			Messages:       a.messages,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			IterationCount: iterationCount,
			MaxIterations:  maxIterations,
			TokenUsage: session.TokenUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
			Metadata: make(map[string]string),
		}
		_ = a.options.SessionManager.Save(sess)
	}

	// Get final response (last assistant message)
	finalResponse := ""
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.messages[i].Role == schema.Assistant && a.messages[i].Content != "" {
			finalResponse = a.messages[i].Content
			break
		}
	}

	return &ChatResponse{
		Response:         finalResponse,
		MessageCount:     len(a.messages),
		IterationCount:   iterationCount,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		SessionID:        sessionID,
	}, nil
}

// initializeTools initializes all available tools for the agent
func (a *ChatAgent) initializeTools(ctx context.Context, workDir string) error {
	// File system tools
	a.toolInstances["read_file"] = tools.NewReadFileTool(workDir, a.options.MaxLinesPerRead)
	a.toolInstances["list_files"] = tools.NewListFilesTool(workDir, tools.DefaultMaxFiles)
	a.toolInstances["list_directory"] = tools.NewListDirectoryTool(workDir)

	// File editing tools
	a.toolInstances["write_file"] = tools.NewWriteFileTool(workDir)
	a.toolInstances["edit_file"] = tools.NewEditFileTool(workDir)
	a.toolInstances["append_file"] = tools.NewAppendFileTool(workDir)

	// Search tools
	a.toolInstances["grep_file"] = tools.NewGrepFileTool(workDir, tools.DefaultMaxResults)
	a.toolInstances["grep_directory"] = tools.NewGrepDirectoryTool(workDir, tools.DefaultMaxResults, 100, tools.DefaultGrepTimeout)

	// Git tools
	a.toolInstances["git_status"] = tools.NewGitStatusTool(a.options.GitExecutor)
	a.toolInstances["git_log"] = tools.NewGitLogTool(a.options.GitExecutor)
	a.toolInstances["git_show"] = tools.NewGitShowTool(a.options.GitExecutor)
	a.toolInstances["git_branch"] = tools.NewGitBranchTool(a.options.GitExecutor)

	return nil
}

// buildToolInfos builds the tool info list for Eino
func (a *ChatAgent) buildToolInfos() []*schema.ToolInfo {
	return []*schema.ToolInfo{
		{
			Name: "read_file",
			Desc: "Read file contents",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"file_path":  {Type: schema.String, Desc: "Path to the file", Required: true},
				"start_line": {Type: schema.Integer, Desc: "Starting line (1-indexed)", Required: false},
				"end_line":   {Type: schema.Integer, Desc: "Ending line (1-indexed)", Required: false},
			}),
		},
		{
			Name: "write_file",
			Desc: "Create or overwrite a file",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"file_path": {Type: schema.String, Desc: "Path to the file", Required: true},
				"content":   {Type: schema.String, Desc: "File content", Required: true},
			}),
		},
		{
			Name: "list_files",
			Desc: "List files in a directory",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"directory": {Type: schema.String, Desc: "Directory path", Required: true},
			}),
		},
		{
			Name: "grep_file",
			Desc: "Search for patterns in a file",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"file_path": {Type: schema.String, Desc: "Path to the file", Required: true},
				"pattern":   {Type: schema.String, Desc: "Search pattern (regex)", Required: true},
			}),
		},
		{
			Name:        "git_status",
			Desc:        "Show git repository status",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		},
		{
			Name: "git_log",
			Desc: "Show git commit history",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"limit": {Type: schema.Integer, Desc: "Number of commits", Required: false},
			}),
		},
	}
}

// getSystemPrompt returns the system prompt for chat
func (a *ChatAgent) getSystemPrompt(language string) string {
	return GetChatSystemPrompt(language)
}

// compressMessages compresses the message history by keeping recent messages
func (a *ChatAgent) compressMessages(keepRecent int) {
	if len(a.messages) <= keepRecent {
		return
	}

	// Keep system message and recent messages
	systemMessages := make([]*schema.Message, 0)
	for _, msg := range a.messages {
		if msg.Role == schema.System {
			systemMessages = append(systemMessages, msg)
		}
	}

	recentMessages := a.messages[len(a.messages)-keepRecent:]

	// Combine system and recent messages
	compressed := make([]*schema.Message, 0, len(systemMessages)+len(recentMessages))
	compressed = append(compressed, systemMessages...)
	compressed = append(compressed, recentMessages...)

	a.messages = compressed
}

// GetMessages returns the current message history
func (a *ChatAgent) GetMessages() []*schema.Message {
	return a.messages
}

// ClearMessages clears the message history except for system message
func (a *ChatAgent) ClearMessages() {
	systemMessages := make([]*schema.Message, 0)
	for _, msg := range a.messages {
		if msg.Role == schema.System {
			systemMessages = append(systemMessages, msg)
		}
	}
	a.messages = systemMessages
}

// AddMessage adds a message to the history
func (a *ChatAgent) AddMessage(role schema.RoleType, content string) {
	a.messages = append(a.messages, &schema.Message{
		Role:    role,
		Content: content,
	})
}
