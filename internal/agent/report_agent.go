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
	Title       string   // Report title
	Period      string   // Time period covered
	Author      string   // Author name
	Summary     string   // Executive summary
	Features    []string // New features developed
	Fixes       []string // Bugs fixed
	Refactoring []string // Refactoring work
	Other       []string // Other work
	Highlights  string   // Key highlights
	NextSteps   string   // Planned next steps (optional)
}

// FormatReport formats the report as markdown
func (r *ReportInfo) FormatReport() string {
	var sb strings.Builder

	// Header
	if r.Title != "" {
		sb.WriteString("# ")
		sb.WriteString(r.Title)
		sb.WriteString("\n\n")
	}

	// Metadata
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

	// Summary
	if r.Summary != "" {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(r.Summary)
		sb.WriteString("\n\n")
	}

	// Features
	if len(r.Features) > 0 {
		sb.WriteString("## New Features\n\n")
		for _, feature := range r.Features {
			sb.WriteString("- ")
			sb.WriteString(feature)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Bug Fixes
	if len(r.Fixes) > 0 {
		sb.WriteString("## Bug Fixes\n\n")
		for _, fix := range r.Fixes {
			sb.WriteString("- ")
			sb.WriteString(fix)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Refactoring
	if len(r.Refactoring) > 0 {
		sb.WriteString("## Refactoring & Improvements\n\n")
		for _, item := range r.Refactoring {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Other
	if len(r.Other) > 0 {
		sb.WriteString("## Other Work\n\n")
		for _, item := range r.Other {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Highlights
	if r.Highlights != "" {
		sb.WriteString("## Highlights\n\n")
		sb.WriteString(r.Highlights)
		sb.WriteString("\n\n")
	}

	// Next Steps
	if r.NextSteps != "" {
		sb.WriteString("## Next Steps\n\n")
		sb.WriteString(r.NextSteps)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// ReportResponse contains the result of report generation
type ReportResponse struct {
	ReportInfo       *ReportInfo // Structured report information
	Content          string      // Complete formatted report
	PromptTokens     int         // Number of tokens in the prompt
	CompletionTokens int         // Number of tokens in the completion
	TotalTokens      int         // Total tokens used
}

// GetTitle returns the report title (implements ui.ReportDisplayer)
func (r *ReportResponse) GetTitle() string {
	if r.ReportInfo != nil {
		return r.ReportInfo.Title
	}
	return ""
}

// GetContent returns the report content (implements ui.ReportDisplayer)
func (r *ReportResponse) GetContent() string {
	return r.Content
}

// ReportAgentOptions contains configuration for ReportAgent
type ReportAgentOptions struct {
	Language    string            // Output language (default: "en")
	GitExecutor git.Executor      // Git executor for running git commands
	LLMProvider llm.Provider      // LLM provider for generating messages
	Printer     *ui.StreamPrinter // Stream printer for output (optional)
	Output      io.Writer         // Output writer (used if Printer is nil)
	Debug       bool              // Enable debug mode
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
func BuildReportSystemPrompt(language, context string) string {
	tmpl, err := template.New("report_prompt").Parse(ReportSystemPrompt)
	if err != nil {
		return ReportSystemPrompt
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Language": language,
		"Context":  context,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return ReportSystemPrompt
	}
	return buf.String()
}

// GenerateReport generates a development report based on commit history
func (a *ReportAgent) GenerateReport(ctx context.Context, req ReportRequest) (*ReportResponse, error) {
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

	// Step 1: Get commit history
	printStep(1, fmt.Sprintf("Fetching commits from %s to %s...", req.Since, req.Until))

	var commitLog string
	if a.opts.GitExecutor != nil {
		printToolCall("git_log")
		logOpts := git.LogOptions{
			Since:  req.Since,
			Until:  req.Until,
			Format: "%h|%s|%ad",
		}
		if req.Author != "" {
			logOpts.Author = req.Author
		}
		commitLog, err = a.opts.GitExecutor.Log(ctx, logOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit log: %w", err)
		}
		if commitLog == "" {
			return nil, fmt.Errorf("no commits found for the specified period")
		}

		// Count commits
		commitCount := len(strings.Split(commitLog, "\n"))
		printSuccess(fmt.Sprintf("Found %d commits", commitCount))
	}

	// Build system prompt
	printStep(2, "Analyzing commits and generating report...")
	systemPrompt := BuildReportSystemPrompt(req.Language, req.Context)
	printInfo(fmt.Sprintf("Language: %s", req.Language))
	log.Debug("System prompt built")

	// Build user message
	var userMessage strings.Builder
	userMessage.WriteString("## Report Period\n")
	userMessage.WriteString(fmt.Sprintf("- Start: %s\n", req.Since))
	userMessage.WriteString(fmt.Sprintf("- End: %s\n", req.Until))
	if req.Author != "" {
		userMessage.WriteString(fmt.Sprintf("- Author: %s\n", req.Author))
	}
	userMessage.WriteString("\n")

	userMessage.WriteString("## Commit History\n")
	userMessage.WriteString("```\n")
	userMessage.WriteString(commitLog)
	userMessage.WriteString("\n```\n")

	// Define the submit_report tool for structured output
	submitReportTool := &schema.ToolInfo{
		Name: "submit_report",
		Desc: "Submit the structured development report. Use this to output the report in a structured format.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title": {
				Type:     schema.String,
				Desc:     "Report title (e.g., 'Weekly Development Report' or 'Monthly Progress Report')",
				Required: true,
			},
			"period": {
				Type:     schema.String,
				Desc:     "Time period covered (e.g., 'January 15-21, 2024')",
				Required: true,
			},
			"author": {
				Type:     schema.String,
				Desc:     "Author name (optional)",
				Required: false,
			},
			"summary": {
				Type:     schema.String,
				Desc:     "Executive summary of the work done during this period",
				Required: true,
			},
			"features": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "List of new features developed",
				Required: false,
			},
			"fixes": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "List of bugs fixed",
				Required: false,
			},
			"refactoring": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "List of refactoring and improvement work",
				Required: false,
			},
			"other": {
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "Other work not fitting above categories",
				Required: false,
			},
			"highlights": {
				Type:     schema.String,
				Desc:     "Key highlights or achievements (optional)",
				Required: false,
			},
			"next_steps": {
				Type:     schema.String,
				Desc:     "Planned next steps (optional)",
				Required: false,
			},
		}),
	}

	// Bind tool to chat model
	if err := chatModel.BindTools([]*schema.ToolInfo{submitReportTool}); err != nil {
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

			if toolCall.Function.Name == "submit_report" {
				var params SubmitReportParams
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					log.Debug("Failed to parse tool call arguments: %v", err)
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
		}
	}

	return nil, fmt.Errorf("failed to generate report: no valid response from LLM")
}
