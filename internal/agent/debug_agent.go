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
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/huimingz/gitbuddy-go/internal/agent/session"
	"github.com/huimingz/gitbuddy-go/internal/agent/tools"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// MessageModifier is a function that modifies messages before sending to LLM
// This allows for dynamic message processing, filtering, or augmentation
type MessageModifier func(messages []*schema.Message) []*schema.Message

// DebugRequest contains the input for debugging
type DebugRequest struct {
	Issue                  string           // Issue description from user
	Language               string           // Output language
	Context                string           // Additional context
	Files                  []string         // Specific files to investigate
	WorkDir                string           // Working directory
	IssuesDir              string           // Directory to save reports
	MaxLines               int              // Maximum lines per file read
	MaxIterations          int              // Maximum number of agent iterations
	Interactive            bool             // Enable interactive feedback
	EnableCompression      bool             // Enable message history compression
	CompressionThreshold   int              // Number of messages before compression
	CompressionKeepRecent  int              // Number of recent messages to keep after compression
	ShowCompressionSummary bool             // Show compression summary to user
	MessageModifier        MessageModifier  // Optional message modifier function
	Session                *session.Session // Optional session to resume from
}

// DebugResponse contains the result of debugging
type DebugResponse struct {
	Report           string
	FilePath         string // Path to saved report file
	SessionID        string // Session ID for resuming
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
	RetryConfig     llm.RetryConfig
}

// DebugPhase represents the current phase of the debugging process
type DebugPhase string

const (
	PhaseProblemDefinition   DebugPhase = "problem_definition"    // 定义问题阶段
	PhaseImpactAnalysis      DebugPhase = "impact_analysis"       // 影响范围分析
	PhaseRootCauseHypothesis DebugPhase = "root_cause_hypothesis" // 根因假设
	PhaseInvestigationPlan   DebugPhase = "investigation_plan"    // 制定排查计划
	PhaseExecution           DebugPhase = "execution"             // 执行排查
	PhaseVerification        DebugPhase = "verification"          // 验证结果
	PhaseReporting           DebugPhase = "reporting"             // 生成报告
)

// ExecutionPlan represents a dynamic plan for the debugging process
type ExecutionPlan struct {
	Tasks        []PlanTask
	CurrentPhase DebugPhase
	PhaseHistory []PhaseTransition
	LastUpdated  time.Time
}

// PhaseTransition records when the debugging phase changes
type PhaseTransition struct {
	FromPhase DebugPhase
	ToPhase   DebugPhase
	Timestamp time.Time
	Reason    string
}

// PlanTask represents a single task in the execution plan
type PlanTask struct {
	ID          string
	Description string
	Status      string // "pending", "in_progress", "completed", "skipped"
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// NewExecutionPlan creates a new execution plan
func NewExecutionPlan() *ExecutionPlan {
	return &ExecutionPlan{
		Tasks:        []PlanTask{},
		CurrentPhase: PhaseProblemDefinition,
		PhaseHistory: []PhaseTransition{},
		LastUpdated:  time.Now(),
	}
}

// TransitionToPhase transitions to a new debugging phase
func (p *ExecutionPlan) TransitionToPhase(newPhase string, reason string) {
	phase := DebugPhase(newPhase)

	if p.CurrentPhase == phase {
		return
	}

	transition := PhaseTransition{
		FromPhase: p.CurrentPhase,
		ToPhase:   phase,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	p.PhaseHistory = append(p.PhaseHistory, transition)
	p.CurrentPhase = phase
	p.LastUpdated = time.Now()
}

// GetCurrentPhase returns the current phase as a string
func (p *ExecutionPlan) GetCurrentPhase() string {
	return string(p.CurrentPhase)
}

// GetPhaseDescription returns a human-readable description of the current phase
func (p *ExecutionPlan) GetPhaseDescription() string {
	descriptions := map[DebugPhase]string{
		PhaseProblemDefinition:   "🔍 问题定义阶段 - 明确问题的症状、影响和背景",
		PhaseImpactAnalysis:      "📊 影响分析阶段 - 确定问题的影响范围和严重程度",
		PhaseRootCauseHypothesis: "💡 根因假设阶段 - 基于现有信息提出可能的根本原因",
		PhaseInvestigationPlan:   "📋 计划制定阶段 - 制定详细的排查计划",
		PhaseExecution:           "🔧 执行排查阶段 - 执行排查计划并收集证据",
		PhaseVerification:        "✅ 验证阶段 - 验证发现的根因和解决方案",
		PhaseReporting:           "📝 报告生成阶段 - 整理发现并生成报告",
	}

	if desc, ok := descriptions[p.CurrentPhase]; ok {
		return desc
	}
	return string(p.CurrentPhase)
}

// AddTask adds a new task to the plan
func (p *ExecutionPlan) AddTask(id, description string) {
	p.Tasks = append(p.Tasks, PlanTask{
		ID:          id,
		Description: description,
		Status:      "pending",
		CreatedAt:   time.Now(),
	})
	p.LastUpdated = time.Now()
}

// UpdateTask updates the status of a task
func (p *ExecutionPlan) UpdateTask(id, status string) bool {
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			oldStatus := p.Tasks[i].Status
			p.Tasks[i].Status = status
			if status == "completed" || status == "skipped" {
				now := time.Now()
				p.Tasks[i].CompletedAt = &now
			}
			p.LastUpdated = time.Now()
			return oldStatus != status
		}
	}
	return false
}

// RemoveTask removes a task from the plan
func (p *ExecutionPlan) RemoveTask(id string) bool {
	for i, task := range p.Tasks {
		if task.ID == id {
			p.Tasks = append(p.Tasks[:i], p.Tasks[i+1:]...)
			p.LastUpdated = time.Now()
			return true
		}
	}
	return false
}

// GetSummary returns a formatted summary of the plan
func (p *ExecutionPlan) GetSummary() string {
	var summary strings.Builder

	// Show current phase
	summary.WriteString(p.GetPhaseDescription())
	summary.WriteString("\n\n")

	if len(p.Tasks) == 0 {
		summary.WriteString("No tasks defined yet.")
		return summary.String()
	}

	summary.WriteString("📋 Current Tasks:\n")

	pending := 0
	inProgress := 0
	completed := 0
	skipped := 0

	for i, task := range p.Tasks {
		var statusIcon string
		switch task.Status {
		case "pending":
			statusIcon = "⏳"
			pending++
		case "in_progress":
			statusIcon = "🔄"
			inProgress++
		case "completed":
			statusIcon = "✅"
			completed++
		case "skipped":
			statusIcon = "⏭️"
			skipped++
		default:
			statusIcon = "❓"
		}

		summary.WriteString(fmt.Sprintf("  %d. %s %s\n", i+1, statusIcon, task.Description))
	}

	summary.WriteString(fmt.Sprintf("\nProgress: %d completed, %d in progress, %d pending", completed, inProgress, pending))
	if skipped > 0 {
		summary.WriteString(fmt.Sprintf(", %d skipped", skipped))
	}

	return summary.String()
}

// GetChanges compares with another plan and returns the changes
func (p *ExecutionPlan) GetChanges(old interface{}) []string {
	oldPlan, ok := old.(*ExecutionPlan)
	if !ok || oldPlan == nil {
		return []string{"Initial plan created"}
	}

	var changes []string

	// Check for new tasks
	oldTaskIDs := make(map[string]bool)
	for _, task := range oldPlan.Tasks {
		oldTaskIDs[task.ID] = true
	}

	for _, task := range p.Tasks {
		if !oldTaskIDs[task.ID] {
			changes = append(changes, fmt.Sprintf("➕ Added: %s", task.Description))
		}
	}

	// Check for removed tasks
	newTaskMap := make(map[string]PlanTask)
	for _, task := range p.Tasks {
		newTaskMap[task.ID] = task
	}

	for _, oldTask := range oldPlan.Tasks {
		if _, exists := newTaskMap[oldTask.ID]; !exists {
			changes = append(changes, fmt.Sprintf("➖ Removed: %s", oldTask.Description))
		}
	}

	// Check for status changes
	for _, newTask := range p.Tasks {
		for _, oldTask := range oldPlan.Tasks {
			if newTask.ID == oldTask.ID && newTask.Status != oldTask.Status {
				changes = append(changes, fmt.Sprintf("🔄 Status changed: %s (%s → %s)",
					newTask.Description, oldTask.Status, newTask.Status))
			}
		}
	}

	return changes
}

// Clone creates a deep copy of the execution plan
func (p *ExecutionPlan) Clone() interface{} {
	if p == nil {
		return (*ExecutionPlan)(nil)
	}

	clone := &ExecutionPlan{
		Tasks:        make([]PlanTask, len(p.Tasks)),
		CurrentPhase: p.CurrentPhase,
		PhaseHistory: make([]PhaseTransition, len(p.PhaseHistory)),
		LastUpdated:  p.LastUpdated,
	}

	for i, task := range p.Tasks {
		clone.Tasks[i] = PlanTask{
			ID:          task.ID,
			Description: task.Description,
			Status:      task.Status,
			CreatedAt:   task.CreatedAt,
		}
		if task.CompletedAt != nil {
			completedAt := *task.CompletedAt
			clone.Tasks[i].CompletedAt = &completedAt
		}
	}

	copy(clone.PhaseHistory, p.PhaseHistory)

	return clone
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

	printExecutionPlan := func(plan *ExecutionPlan) {
		if printer != nil {
			summary := plan.GetSummary()
			_ = printer.PrintInfo("\n" + summary + "\n")
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

	// Execution plan and phase management tools
	executionPlan := NewExecutionPlan()
	updateExecutionPlanTool := tools.NewUpdateExecutionPlanTool(executionPlan)
	transitionPhaseTool := tools.NewTransitionPhaseTool(executionPlan)

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
				"title":   {Type: schema.String, Desc: "Question title - concise summary of what you're asking", Required: true},
				"content": {Type: schema.String, Desc: "Question content - detailed background, current analysis state, or information the user needs to know", Required: true},
				"prompt":  {Type: schema.String, Desc: "Prompt text - guide the user on how to answer (e.g., 'Please select an option', 'Please enter the file path', 'Please describe the error scenario')", Required: true},
				"options": {Type: schema.Array, Desc: "Options list (optional) - if provided, user selects from options; if not provided, user enters free text. For choice questions, at least 2 options are required", Required: false},
			}),
		})

		// Print a reminder that interactive mode is enabled
		printInfo("🎯 Interactive mode enabled - Agent can request your feedback during analysis")
	} else {
		printInfo("ℹ️  Non-interactive mode - Agent will work autonomously without requesting feedback")
	}

	// Add execution plan tool
	toolInfos = append(toolInfos, &schema.ToolInfo{
		Name: "update_execution_plan",
		Desc: updateExecutionPlanTool.Description(),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action":      {Type: schema.String, Desc: "Action to perform: add, update, remove, or show", Required: true},
			"task_id":     {Type: schema.String, Desc: "Unique identifier for the task (required for update/remove)", Required: false},
			"description": {Type: schema.String, Desc: "Task description (required for add)", Required: false},
			"status":      {Type: schema.String, Desc: "Task status: pending, in_progress, completed, or skipped (required for update)", Required: false},
		}),
	})

	// Add phase transition tool
	toolInfos = append(toolInfos, &schema.ToolInfo{
		Name: "transition_phase",
		Desc: transitionPhaseTool.Description(),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"new_phase": {Type: schema.String, Desc: "The phase to transition to", Required: true},
			"reason":    {Type: schema.String, Desc: "Why you are transitioning to this phase", Required: true},
		}),
	})

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

	// Set up default message modifier if not provided
	if req.MessageModifier == nil {
		// Create a default modifier chain that:
		// 1. Adds progress context to help LLM understand where it is
		// 2. Summarizes very long tool results
		// 3. Deduplicates consecutive identical messages
		req.MessageModifier = MessageModifierChain(
			SummarizeToolResults(5000), // Limit tool results to 5000 chars
			DeduplicateMessages(),
		)
	}

	// Agent loop
	iterationCount := 0
	lastPlanSnapshot := executionPlan.Clone().(*ExecutionPlan)

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

		// Display execution plan at the start of each iteration (every 3 iterations or when changed)
		if iterationCount == 1 || iterationCount%3 == 0 {
			changes := executionPlan.GetChanges(lastPlanSnapshot)
			if len(changes) > 0 || iterationCount == 1 {
				printExecutionPlan(executionPlan)
				lastPlanSnapshot = executionPlan.Clone().(*ExecutionPlan)
			}
		}

		printProgress(fmt.Sprintf("Agent iteration %d...", iterationCount))

		// Apply message modifier with progress context (similar to Eino's MessageModifier)
		messagesToSend := messages
		if req.MessageModifier != nil {
			// Add progress context before applying user's modifier
			progressModifier := CreateProgressContextModifier(executionPlan, iterationCount, maxIterations)
			combinedModifier := MessageModifierChain(progressModifier, req.MessageModifier)
			messagesToSend = combinedModifier(messages)
			log.Debug("MessageModifier applied, messages count: %d -> %d", len(messages), len(messagesToSend))
		}

		// Stream LLM response with retry
		streamReader, err := llm.WithRetryResult(ctx, a.opts.RetryConfig, func() (*schema.StreamReader[*schema.Message], error) {
			return chatModel.Stream(ctx, messagesToSend)
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

			case "update_execution_plan":
				var params tools.UpdateExecutionPlanParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = updateExecutionPlanTool.Execute(ctx, &params)
				}

			case "transition_phase":
				var params tools.TransitionPhaseParams
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
					toolErr = fmt.Errorf("invalid parameters: %w", err)
				} else {
					result, toolErr = transitionPhaseTool.Execute(ctx, &params)
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

			// Display execution plan after certain tool executions
			if toolErr == nil {
				if tc.Function.Name == "update_execution_plan" {
					// Plan was updated, show the changes
					printExecutionPlan(executionPlan)
				} else if tc.Function.Name == "transition_phase" {
					// Phase transitioned, show the new phase and plan
					printExecutionPlan(executionPlan)
				}
			}
		}

		// Compress message history if enabled and threshold is reached
		if req.EnableCompression && len(messages) > req.CompressionThreshold {
			oldLen := len(messages)
			compressedMessages, summary, err := compressMessageHistoryWithLLM(ctx, chatModel, messages, req.CompressionKeepRecent)
			if err != nil {
				log.Debug("Failed to compress message history with LLM: %v", err)
				// Fallback to simple compression if LLM compression fails
				compressedMessages, summary = simpleCompressMessageHistory(messages, req.CompressionKeepRecent)
			}
			messages = compressedMessages

			// Show compression info
			printProgress(fmt.Sprintf("Message history compressed (%d -> %d messages)", oldLen, len(compressedMessages)))

			// Optionally show summary to user
			if req.ShowCompressionSummary && summary != "" {
				printInfo(fmt.Sprintf("\n📝 Compression Summary:\n%s\n", summary))
			}
		}
	}

	return nil, fmt.Errorf("agent loop exceeded maximum iterations")
}

// compressMessageHistoryWithLLM uses LLM to intelligently compress old message history
// while preserving key information and keeping recent messages intact
// Returns: compressed messages, summary text, error
// chatModel parameter should be the same model.ChatModel returned by CreateChatModel
func compressMessageHistoryWithLLM(ctx context.Context, chatModel interface{}, messages []*schema.Message, keepLastN int) ([]*schema.Message, string, error) {
	if len(messages) <= keepLastN+2 { // +2 for system message and first user message
		return messages, "", nil
	}

	// Structure: [system, first_user_msg, ...old messages to compress..., ...recent messages to keep...]
	systemMsg := messages[0]
	firstUserMsg := messages[1] // Keep the original task/goal
	oldMessages := messages[2 : len(messages)-keepLastN]
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
		return nil, "", fmt.Errorf("chat model does not have Stream method")
	}

	results := streamMethod.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(summaryMessages),
	})

	if len(results) != 2 {
		return nil, "", fmt.Errorf("unexpected Stream method signature")
	}

	// Check for error
	if !results[1].IsNil() {
		return nil, "", fmt.Errorf("failed to generate summary: %w", results[1].Interface().(error))
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
		return nil, "", fmt.Errorf("stream reader does not have Recv method")
	}

	for {
		results := recvMethod.Call(nil)
		if len(results) != 2 {
			return nil, "", fmt.Errorf("unexpected Recv method signature")
		}

		// Check for error
		if !results[1].IsNil() {
			err := results[1].Interface().(error)
			if err == io.EOF {
				break
			}
			return nil, "", fmt.Errorf("stream read error: %w", err)
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
		return nil, "", fmt.Errorf("empty summary generated")
	}

	// Build compressed message history
	// Keep: system message, first user message (task/goal), summary, recent messages
	compressed := []*schema.Message{
		systemMsg,
		firstUserMsg, // Keep the original task/goal
		{
			Role:    schema.User,
			Content: fmt.Sprintf("[Previous Session Summary]\n%s\n\n[Continuing from here...]", summaryText),
		},
	}
	compressed = append(compressed, recentMessages...)

	log.Debug("Compressed %d messages into summary, keeping first user message and %d recent messages", len(oldMessages), len(recentMessages))
	return compressed, summaryText, nil
}

// simpleCompressMessageHistory is a fallback that truncates old messages
// but adds a detailed summary message to preserve critical context
// Returns: compressed messages, summary text
func simpleCompressMessageHistory(messages []*schema.Message, keepLastN int) ([]*schema.Message, string) {
	if len(messages) <= keepLastN+2 { // +2 for system and first user message
		return messages, ""
	}

	// Structure: [system, first_user_msg, ...old messages..., ...recent messages...]
	systemMsg := messages[0]
	firstUserMsg := messages[1] // Keep the original task/goal
	oldMessages := messages[2 : len(messages)-keepLastN]
	recentMessages := messages[len(messages)-keepLastN:]

	// Build a detailed summary preserving key information
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString(fmt.Sprintf("[Note: %d earlier messages were compressed for context management]\n\n", len(oldMessages)))
	summaryBuilder.WriteString("=== Summary of Earlier Investigation ===\n\n")

	// Track tool usage and extract key findings
	toolUsageMap := make(map[string][]string) // tool name -> list of key findings
	var keyFindings []string
	var filesMentioned []string
	fileSet := make(map[string]bool)

	for i, msg := range oldMessages {
		// Extract tool calls and their results
		if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				toolName := tc.Function.Name

				// Extract parameters for context
				var params map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err == nil {
					// Extract file paths from various tool parameters
					if filePath, ok := params["file_path"].(string); ok && filePath != "" {
						if !fileSet[filePath] {
							filesMentioned = append(filesMentioned, filePath)
							fileSet[filePath] = true
						}
					}
					if dirPath, ok := params["directory"].(string); ok && dirPath != "" {
						if !fileSet[dirPath] {
							filesMentioned = append(filesMentioned, dirPath)
							fileSet[dirPath] = true
						}
					}

					// Create a brief description of the tool call
					briefDesc := fmt.Sprintf("%s", toolName)
					if pattern, ok := params["pattern"].(string); ok && pattern != "" {
						briefDesc += fmt.Sprintf(" (pattern: %s)", pattern)
					}
					if question, ok := params["question"].(string); ok && question != "" {
						briefDesc += fmt.Sprintf(" (question: %s)", truncateString(question, 50))
					}

					toolUsageMap[toolName] = append(toolUsageMap[toolName], briefDesc)
				}
			}
		}

		// Extract key findings from tool results (next message after assistant)
		if msg.Role == schema.Tool && i > 0 {
			content := msg.Content
			// If content is too long, extract key parts
			if len(content) > 500 {
				// Try to extract error messages, file paths, or important lines
				lines := strings.Split(content, "\n")
				var importantLines []string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					// Keep lines that look important
					if strings.Contains(line, "error") || strings.Contains(line, "Error") ||
						strings.Contains(line, "failed") || strings.Contains(line, "Failed") ||
						strings.Contains(line, ".go:") || strings.Contains(line, ".py:") ||
						strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") ||
						strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "def ") {
						importantLines = append(importantLines, line)
						if len(importantLines) >= 5 { // Limit to 5 important lines per tool result
							break
						}
					}
				}
				if len(importantLines) > 0 {
					keyFindings = append(keyFindings, strings.Join(importantLines, "\n  "))
				}
			} else if content != "" {
				// Keep short results as-is
				keyFindings = append(keyFindings, truncateString(content, 200))
			}
		}

		// Extract assistant's analysis and conclusions
		if msg.Role == schema.Assistant && msg.Content != "" {
			// Look for analysis patterns
			content := msg.Content
			if strings.Contains(content, "found") || strings.Contains(content, "discovered") ||
				strings.Contains(content, "issue") || strings.Contains(content, "problem") ||
				strings.Contains(content, "conclusion") || strings.Contains(content, "summary") {
				keyFindings = append(keyFindings, truncateString(content, 200))
			}
		}
	}

	// Write tool usage summary
	if len(toolUsageMap) > 0 {
		summaryBuilder.WriteString("## Tools Used:\n")
		totalCalls := 0
		for tool, calls := range toolUsageMap {
			summaryBuilder.WriteString(fmt.Sprintf("- %s: %d calls\n", tool, len(calls)))
			totalCalls += len(calls)
			// Show first few calls as examples
			for i, call := range calls {
				if i >= 3 { // Limit to 3 examples per tool
					summaryBuilder.WriteString(fmt.Sprintf("  ... and %d more\n", len(calls)-i))
					break
				}
				summaryBuilder.WriteString(fmt.Sprintf("  • %s\n", call))
			}
		}
		summaryBuilder.WriteString(fmt.Sprintf("\nTotal tool calls: %d\n\n", totalCalls))
	}

	// Write files investigated
	if len(filesMentioned) > 0 {
		summaryBuilder.WriteString("## Files/Directories Investigated:\n")
		for i, file := range filesMentioned {
			if i >= 10 { // Limit to 10 files
				summaryBuilder.WriteString(fmt.Sprintf("... and %d more\n", len(filesMentioned)-i))
				break
			}
			summaryBuilder.WriteString(fmt.Sprintf("- %s\n", file))
		}
		summaryBuilder.WriteString("\n")
	}

	// Write key findings
	if len(keyFindings) > 0 {
		summaryBuilder.WriteString("## Key Findings & Analysis:\n")
		for i, finding := range keyFindings {
			if i >= 8 { // Limit to 8 findings
				summaryBuilder.WriteString(fmt.Sprintf("... and %d more findings\n", len(keyFindings)-i))
				break
			}
			summaryBuilder.WriteString(fmt.Sprintf("%d. %s\n\n", i+1, finding))
		}
	}

	summaryBuilder.WriteString("=== End of Summary ===\n")
	summaryBuilder.WriteString("\nContinuing investigation with recent context...\n")

	summaryText := summaryBuilder.String()

	// Build compressed message history
	// Keep: system message, first user message (task/goal), summary, recent messages
	compressed := []*schema.Message{
		systemMsg,
		firstUserMsg, // Keep the original task/goal
		{
			Role:    schema.User,
			Content: summaryText,
		},
	}
	compressed = append(compressed, recentMessages...)

	log.Debug("Simple compression: %d messages -> %d messages (kept first user message and %d recent)", len(messages), len(compressed), len(recentMessages))
	return compressed, summaryText
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
