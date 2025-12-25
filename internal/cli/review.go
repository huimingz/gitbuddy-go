package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/huimingz/gitbuddy-go/internal/agent"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/huimingz/gitbuddy-go/internal/ui"
	"github.com/spf13/cobra"
)

var (
	reviewContext  string
	reviewLanguage string
	reviewFiles    string
	reviewSeverity string
	reviewFocus    string
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review staged code changes",
	Long: `Review staged code changes using AI to identify potential issues.

This command will:
1. Analyze your staged changes (git diff --cached)
2. Identify bugs, security issues, performance problems, and style issues
3. Provide suggestions for improvement

Examples:
  gitbuddy review
  gitbuddy review -c "This is an authentication module"
  gitbuddy review --files "auth.go,crypto.go"
  gitbuddy review --severity error
  gitbuddy review --focus security,performance
  gitbuddy review -l zh --focus security`,
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().StringVarP(&reviewContext, "context", "c", "", "Additional context to help AI understand the code")
	reviewCmd.Flags().StringVarP(&reviewLanguage, "language", "l", "", "Output language (en, zh, ja, etc.)")
	reviewCmd.Flags().StringVar(&reviewFiles, "files", "", "Comma-separated list of files to review (default: all staged files)")
	reviewCmd.Flags().StringVar(&reviewSeverity, "severity", "", "Minimum severity level to report (error, warning, info)")
	reviewCmd.Flags().StringVar(&reviewFocus, "focus", "", "Comma-separated focus areas (security, performance, style)")

	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	startTime := time.Now()

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
	language := cfg.GetLanguage(reviewLanguage)
	log.Debug("Using language: %s", language)

	// Get review config
	reviewCfg := cfg.GetReviewConfig()
	log.Debug("Max lines per read: %d", reviewCfg.MaxLinesPerRead)

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

	// Check if there are staged changes
	diff, err := gitExecutor.DiffCached(ctx)
	if err != nil {
		return fmt.Errorf("failed to get staged changes: %w", err)
	}

	if diff == "" {
		fmt.Println("No staged changes found.")
		fmt.Println("\nTo stage changes, use:")
		fmt.Println("  git add <file>")
		fmt.Println("  git add -A")
		return nil
	}

	// Parse files list
	var files []string
	if reviewFiles != "" {
		files = strings.Split(reviewFiles, ",")
		for i := range files {
			files[i] = strings.TrimSpace(files[i])
		}
	}

	// Parse focus areas
	var focus []string
	if reviewFocus != "" {
		focus = strings.Split(reviewFocus, ",")
		for i := range focus {
			focus[i] = strings.TrimSpace(focus[i])
		}
	}

	// Validate severity
	if reviewSeverity != "" {
		validSeverities := map[string]bool{
			agent.SeverityError:   true,
			agent.SeverityWarning: true,
			agent.SeverityInfo:    true,
		}
		if !validSeverities[reviewSeverity] {
			return fmt.Errorf("invalid severity level: %s (valid: error, warning, info)", reviewSeverity)
		}
	}

	// Create stream printer for output
	printer := ui.NewStreamPrinter(os.Stdout, ui.WithVerbose(debugMode))

	// Create review agent
	reviewAgent := agent.NewReviewAgent(agent.ReviewAgentOptions{
		Language:        language,
		GitExecutor:     gitExecutor,
		LLMProvider:     provider,
		Printer:         printer,
		Debug:           debugMode,
		WorkDir:         workDir,
		MaxLinesPerRead: reviewCfg.MaxLinesPerRead,
	})

	// Print initial indicator
	_ = printer.PrintThinking("Starting code review...")

	// Perform review
	req := agent.ReviewRequest{
		Language: language,
		Context:  reviewContext,
		Files:    files,
		Severity: reviewSeverity,
		Focus:    focus,
		WorkDir:  workDir,
		MaxLines: reviewCfg.MaxLinesPerRead,
	}

	response, err := reviewAgent.Review(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to perform code review: %w", err)
	}

	// Print the review results
	err = ui.ShowReviewResult(response, os.Stdout)
	if err != nil {
		return err
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
