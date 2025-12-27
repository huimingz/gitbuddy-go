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

// ReportRequest contains the input for report generation
type ReportRequest struct {
	Since    string // Start date
	Until    string // End date
	Author   string // Author name
	Language string // Output language
	Context  string // Additional context from user
}

// ReportInfo contains structured report information
type ReportInfo struct {
	Title       string
	Period      string
	Author      string
	Summary     string
	Features    []string
	Fixes       []string
	Refactoring []string
	Other       []string
	Highlights  string
	NextSteps   string
}

// FormatReport formats the report as markdown
func (r *ReportInfo) FormatReport() string {
	var sb strings.Builder

	if r.Title != "" {
		sb.WriteString("# ")
		sb.WriteString(r.Title)
		sb.WriteString("\n\n")
	}

	if r.Period != "" || r.Author != "" {
		if r.Period != "" {
			sb.WriteString("**Period:** ")
			sb.WriteString(r.Period)
			sb.WriteString("\n")
		}
		if r.Author != "" {
			sb.WriteString("**Author:** ")
			sb.WriteString(r.Author)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if r.Summary != "" {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(r.Summary)
		sb.WriteString("\n\n")
	}

	if len(r.Features) > 0 {
		sb.WriteString("## New Features\n\n")
		for _, feature := range r.Features {
			sb.WriteString("- ")
			sb.WriteString(feature)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(r.Fixes) > 0 {
		sb.WriteString("## Bug Fixes\n\n")
		for _, fix := range r.Fixes {
			sb.WriteString("- ")
			sb.WriteString(fix)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(r.Refactoring) > 0 {
		sb.WriteString("## Refactoring & Improvements\n\n")
		for _, item := range r.Refactoring {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(r.Other) > 0 {
		sb.WriteString("## Other Work\n\n")
		for _, item := range r.Other {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if r.Highlights != "" {
		sb.WriteString("## Highlights\n\n")
		sb.WriteString(r.Highlights)
		sb.WriteString("\n\n")
	}

	if r.NextSteps != "" {
		sb.WriteString("## Next Steps\n\n")
		sb.WriteString(r.NextSteps)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// ReportResponse contains the result of report generation
type ReportResponse struct {
	ReportInfo       *ReportInfo
	Content          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// GetTitle returns the report title
func (r *ReportResponse) GetTitle() string {
	if r.ReportInfo != nil {
		return r.ReportInfo.Title
	}
	return ""
}

// GetContent returns the report content
func (r *ReportResponse) GetContent() string {
	return r.Content
}

// ReportAgentOptions contains configuration for ReportAgent
type ReportAgentOptions struct {
	Language    string
	GitExecutor git.Executor
	LLMProvider llm.Provider
	Printer     *ui.StreamPrinter
	Output      io.Writer
	Debug       bool
	RetryConfig llm.RetryConfig
}

// ReportAgent generates development reports using LLM
type ReportAgent struct {
	opts ReportAgentOptions
}

// NewReportAgent creates a new ReportAgent
func NewReportAgent(opts ReportAgentOptions) *ReportAgent {
	if opts.Language == "" {
		opts.Language = "en"
	}
	return &ReportAgent{opts: opts}
}

// SubmitReportParams represents the structured report information from LLM
type SubmitReportParams struct {
	Title       string   `json:"title"`
	Period      string   `json:"period"`
	Author      string   `json:"author,omitempty"`
	Summary     string   `json:"summary"`
	Features    []string `json:"features,omitempty"`
	Fixes       []string `json:"fixes,omitempty"`
	Refactoring []string `json:"refactoring,omitempty"`
	Other       []string `json:"other,omitempty"`
	Highlights  string   `json:"highlights,omitempty"`
	NextSteps   string   `json:"next_steps,omitempty"`
}

// ToReportInfo converts SubmitReportParams to ReportInfo
func (p *SubmitReportParams) ToReportInfo() *ReportInfo {
	return &ReportInfo{
		Title:       p.Title,
		Period:      p.Period,
		Author:      p.Author,
		Summary:     p.Summary,
		Features:    p.Features,
		Fixes:       p.Fixes,
		Refactoring: p.Refactoring,
		Other:       p.Other,
		Highlights:  p.Highlights,
		NextSteps:   p.NextSteps,
	}
}

// BuildReportSystemPrompt builds the system prompt for report generation
func BuildReportSystemPrompt(language, context, since, until, author string) string {
	tmpl, err := template.New("report_prompt").Parse(ReportSystemPrompt)
	if err != nil {
		return ReportSystemPrompt
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Language": language,
		"Context":  context,
		"Since":    since,
		"Until":    until,
		"Author":   author,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return ReportSystemPrompt
	}
	return buf.String()
}

// GenerateReport generates a development report using agent loop
func (a *ReportAgent) GenerateReport(ctx context.Context, req ReportRequest) (*ReportResponse, error) {
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
	gitLogDateTool := tools.NewGitLogDateTool(a.opts.GitExecutor)
	gitStatusTool := tools.NewGitStatusTool(a.opts.GitExecutor)

	// Define tool schemas
	toolInfos := []*schema.ToolInfo{
		{
			Name: "git_log_date",
			Desc: gitLogDateTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"since":  {Type: schema.String, Desc: "Start date in YYYY-MM-DD format", Required: true},
				"until":  {Type: schema.String, Desc: "End date in YYYY-MM-DD format (optional)", Required: false},
				"author": {Type: schema.String, Desc: "Filter by author name (optional)", Required: false},
			}),
		},
		{
			Name:        "git_status",
			Desc:        gitStatusTool.Description(),
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		},
		{
			Name: "submit_report",
			Desc: "Submit the structured development report. Call this when you have analyzed the commits and are ready to generate the report.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"title":       {Type: schema.String, Desc: "Report title", Required: true},
				"period":      {Type: schema.String, Desc: "Time period covered", Required: true},
				"author":      {Type: schema.String, Desc: "Author name (optional)", Required: false},
				"summary":     {Type: schema.String, Desc: "Executive summary", Required: true},
				"features":    {Type: schema.Array, ElemInfo: &schema.ParameterInfo{Type: schema.String}, Desc: "New features", Required: false},
				"fixes":       {Type: schema.Array, ElemInfo: &schema.ParameterInfo{Type: schema.String}, Desc: "Bug fixes", Required: false},
				"refactoring": {Type: schema.Array, ElemInfo: &schema.ParameterInfo{Type: schema.String}, Desc: "Refactoring work", Required: false},
				"other":       {Type: schema.Array, ElemInfo: &schema.ParameterInfo{Type: schema.String}, Desc: "Other work", Required: false},
				"highlights":  {Type: schema.String, Desc: "Key highlights (optional)", Required: false},
				"next_steps":  {Type: schema.String, Desc: "Planned next steps (optional)", Required: false},
			}),
		},
	}

	// Bind tools to chat model
	if err := chatModel.BindTools(toolInfos); err != nil {
		return nil, fmt.Errorf("failed to bind tools: %w", err)
	}

	// Build system prompt
	systemPrompt := BuildReportSystemPrompt(req.Language, req.Context, req.Since, req.Until, req.Author)
	printInfo(fmt.Sprintf("Generating report: %s to %s", req.Since, req.Until))
	if req.Author != "" {
		printInfo(fmt.Sprintf("Author: %s", req.Author))
	}

	// Initial messages
	userMsg := fmt.Sprintf("Please generate a development report for the period from %s to %s.", req.Since, req.Until)
	if req.Author != "" {
		userMsg += fmt.Sprintf(" Focus on commits by author: %s.", req.Author)
	}
	userMsg += " Use the available tools to analyze the commit history."

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userMsg},
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
				var params SubmitReportParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse submit_report arguments: %v", err)
					continue
				}

				// Set author if not provided by LLM
				if params.Author == "" && req.Author != "" {
					params.Author = req.Author
				}

				reportInfo := params.ToReportInfo()
				printSuccess("Development report generated successfully")

				return &ReportResponse{
					ReportInfo:       reportInfo,
					Content:          reportInfo.FormatReport(),
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				}, nil
			}

			// Execute other tools
			var result string
			var toolErr error

			switch tc.Function.Name {
			case "git_log_date":
				var params tools.GitLogDateParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = gitLogDateTool.Execute(ctx, &params)
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
