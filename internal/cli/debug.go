package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/huimingz/gitbuddy-go/internal/agent"
	"github.com/huimingz/gitbuddy-go/internal/agent/session"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/huimingz/gitbuddy-go/internal/ui"
	"github.com/spf13/cobra"
)

var (
	debugContext       string
	debugLanguage      string
	debugFiles         string
	debugInteractive   bool
	debugIssuesDir     string
	debugMaxIterations int
	debugResume        string
)

var debugCmd = &cobra.Command{
	Use:   "debug <issue-description>",
	Short: "Debug code issues with AI assistance",
	Long: `Debug code issues with AI assistance through systematic analysis.

This command will:
1. Analyze the issue description you provide
2. Explore the codebase using various tools
3. Identify root causes and potential fixes
4. Generate a detailed debugging report

The AI agent has access to:
- File system tools (list_directory, list_files, read_file)
- Search tools (grep_file, grep_directory)
- Git tools (git_status, git_diff_cached, git_log, git_show)
- Interactive feedback (with --interactive flag)

Examples:
  gitbuddy debug "Login fails with 500 error"
  gitbuddy debug "Memory leak in background worker" -c "Happens after 24h"
  gitbuddy debug "Test TestUserAuth is failing" --files "auth_test.go,auth.go"
  gitbuddy debug "API returns wrong data" --interactive
  gitbuddy debug "Performance issue" -l zh --interactive`,
	Args: cobra.ExactArgs(1),
	RunE: runDebug,
}

func init() {
	debugCmd.Flags().StringVarP(&debugContext, "context", "c", "", "Additional context to help AI understand the issue")
	debugCmd.Flags().StringVarP(&debugLanguage, "language", "l", "", "Output language (en, zh, ja, etc.)")
	debugCmd.Flags().StringVar(&debugFiles, "files", "", "Comma-separated list of files to focus on")
	debugCmd.Flags().BoolVarP(&debugInteractive, "interactive", "i", false, "Enable interactive mode (agent can ask for your input)")
	debugCmd.Flags().StringVar(&debugIssuesDir, "issues-dir", "./issues", "Directory to save debug reports")
	debugCmd.Flags().IntVar(&debugMaxIterations, "max-iterations", 0, "Maximum number of agent iterations (0 = use config default)")
	debugCmd.Flags().StringVar(&debugResume, "resume", "", "Resume from a previous session (session ID)")

	rootCmd.AddCommand(debugCmd)
}

func runDebug(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	startTime := time.Now()

	issue := args[0]
	if issue == "" {
		return fmt.Errorf("issue description cannot be empty")
	}

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log.DebugConfig("Configuration", cfg)

	// Get model configuration
	modelConfig, err := cfg.GetModel(modelName)
	if err != nil {
		return fmt.Errorf("failed to get model config: %w", err)
	}

	log.Debug("Using model: %s (provider: %s)", modelName, modelConfig.Provider)

	// Get language
	language := cfg.GetLanguage(debugLanguage)
	log.Debug("Using language: %s", language)

	// Get debug config
	debugCfg := cfg.GetDebugConfig()
	log.Debug("Max lines per read: %d", debugCfg.MaxLinesPerRead)
	log.Debug("Issues directory: %s", debugCfg.IssuesDir)
	log.Debug("Max iterations: %d", debugCfg.MaxIterations)

	// Override issues dir if specified
	issuesDir := debugIssuesDir
	if issuesDir == "./issues" && debugCfg.IssuesDir != "" {
		issuesDir = debugCfg.IssuesDir
	}

	// Override max iterations if specified
	maxIterations := debugMaxIterations
	if maxIterations <= 0 {
		maxIterations = debugCfg.MaxIterations
	}

	// Create LLM provider
	factory := llm.NewProviderFactory()
	provider, err := factory.Create(*modelConfig)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	log.Debug("LLM provider created successfully")

	// Get current working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create git executor
	gitExecutor := git.NewExecutor(workDir)

	// Parse files list
	var files []string
	if debugFiles != "" {
		files = strings.Split(debugFiles, ",")
		for i := range files {
			files[i] = strings.TrimSpace(files[i])
		}
	}

	// Create stream printer for output
	printer := ui.NewStreamPrinter(os.Stdout, ui.WithVerbose(debugMode))

	// Get retry and session config
	retryConfigPtr := cfg.GetRetryConfig()
	sessionConfig := cfg.GetSessionConfig()

	// Convert config.RetryConfig to llm.RetryConfig
	retryConfig := llm.RetryConfig{
		Enabled:     retryConfigPtr.Enabled,
		MaxAttempts: retryConfigPtr.MaxAttempts,
		BackoffBase: retryConfigPtr.BackoffBase,
		BackoffMax:  retryConfigPtr.BackoffMax,
	}

	// Create session manager
	sessionMgr := session.NewManager(sessionConfig.SaveDir)

	// Create debug agent
	debugAgent := agent.NewDebugAgent(agent.DebugAgentOptions{
		Language:        language,
		GitExecutor:     gitExecutor,
		LLMProvider:     provider,
		Printer:         printer,
		Output:          os.Stdout,
		Input:           os.Stdin,
		Debug:           debugMode,
		WorkDir:         workDir,
		IssuesDir:       issuesDir,
		MaxLinesPerRead: debugCfg.MaxLinesPerRead,
		RetryConfig:     retryConfig,
	})

	// Setup context with cancellation for Ctrl+C handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var currentSessionID string
	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  Received interrupt signal. Saving session...")
		cancel()

		// Give some time for graceful shutdown
		time.Sleep(500 * time.Millisecond)

		if currentSessionID != "" && sessionConfig.AutoSave {
			fmt.Printf("✓ Session saved: %s\n", currentSessionID)
			fmt.Printf("  Resume with: gitbuddy debug --resume %s\n", currentSessionID)
		}
		os.Exit(130) // Standard exit code for SIGINT
	}()

	// Check if resuming from a previous session
	var sess *session.Session
	if debugResume != "" {
		_ = printer.PrintInfo(fmt.Sprintf("Resuming session: %s", debugResume))

		loadedSession, err := sessionMgr.Load(debugResume)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}

		sess = loadedSession
		currentSessionID = sess.ID

		_ = printer.PrintSuccess(fmt.Sprintf("Session loaded (iterations: %d/%d)", sess.IterationCount, sess.MaxIterations))
	} else {
		// Print initial indicator
		_ = printer.PrintThinking("Starting debugging session...")
	}

	// Perform debugging
	req := agent.DebugRequest{
		Issue:                  issue,
		Language:               language,
		Context:                debugContext,
		Files:                  files,
		WorkDir:                workDir,
		IssuesDir:              issuesDir,
		MaxLines:               debugCfg.MaxLinesPerRead,
		MaxIterations:          maxIterations,
		Interactive:            debugInteractive,
		EnableCompression:      debugCfg.EnableCompression,
		CompressionThreshold:   debugCfg.CompressionThreshold,
		CompressionKeepRecent:  debugCfg.CompressionKeepRecent,
		ShowCompressionSummary: debugCfg.ShowCompressionSummary,
		Session:                sess,
	}

	response, err := debugAgent.Debug(ctx, req)

	// Save session on success or interruption
	if response != nil && response.SessionID != "" {
		currentSessionID = response.SessionID

		if sessionConfig.AutoSave {
			// Session should be saved by the agent itself
			_ = printer.PrintInfo(fmt.Sprintf("Session ID: %s", response.SessionID))
		}
	}

	if err != nil {
		if ctx.Err() == context.Canceled {
			// Interrupted by user
			return fmt.Errorf("debugging interrupted by user")
		}
		return fmt.Errorf("failed to debug issue: %w", err)
	}

	// Print the debug report
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📋 Debug Report")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
	fmt.Println(response.Report)
	fmt.Println()

	if response.FilePath != "" {
		fmt.Printf("✓ Report saved to: %s\n", response.FilePath)
		fmt.Println()
	}

	// Print stats
	endTime := time.Now()
	stats := &ui.ExecutionStats{
		StartTime:        startTime,
		EndTime:          endTime,
		PromptTokens:     response.PromptTokens,
		CompletionTokens: response.CompletionTokens,
		TotalTokens:      response.TotalTokens,
	}
	_ = printer.PrintStats(stats)

	return nil
}
