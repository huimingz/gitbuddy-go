package cli

import (
	"context"
	"fmt"
	"os"
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
	commitContext  string
	commitLanguage string
	commitAutoYes  bool
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Generate and create a commit",
	Long: `Generate a commit message using AI based on staged changes.

This command will:
1. Analyze your staged changes (git diff --cached)
2. Generate a commit message following Conventional Commits
3. Ask for confirmation before committing

Examples:
  gitbuddy commit
  gitbuddy commit -c "Bug fix for user authentication"
  gitbuddy commit --language zh
  gitbuddy commit -m deepseek`,
	RunE: runCommit,
}

func init() {
	commitCmd.Flags().StringVarP(&commitContext, "context", "c", "", "Additional context to help AI generate better message")
	commitCmd.Flags().StringVarP(&commitLanguage, "language", "l", "", "Output language (en, zh, ja, etc.)")
	commitCmd.Flags().BoolVarP(&commitAutoYes, "yes", "y", false, "Auto-confirm the commit without prompting")
	rootCmd.AddCommand(commitCmd)
}

func runCommit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	startTime := time.Now()

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log.DebugConfig("Configuration", cfg)

	// Get model name (CLI flag > config default)
	model := modelName
	if model == "" {
		model = cfg.DefaultModel
	}

	// Get model config
	modelConfig, err := cfg.GetModel(model)
	if err != nil {
		return fmt.Errorf("failed to get model config: %w", err)
	}

	log.Debug("Using model: %s (provider: %s)", model, modelConfig.Provider)

	// Get language (CLI flag > config > default)
	language := cfg.GetLanguage(commitLanguage)

	log.Debug("Using language: %s", language)

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create git executor
	gitExec := git.NewExecutor(cwd)

	// Check if there are staged changes
	diff, err := gitExec.DiffCached(ctx)
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

	// Create LLM provider
	factory := llm.NewProviderFactory()
	provider, err := factory.Create(*modelConfig)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	log.Debug("LLM provider created successfully")

	// Get retry config
	retryConfigPtr := cfg.GetRetryConfig()

	// Convert config.RetryConfig to llm.RetryConfig
	retryConfig := llm.RetryConfig{
		Enabled:     retryConfigPtr.Enabled,
		MaxAttempts: retryConfigPtr.MaxAttempts,
		BackoffBase: retryConfigPtr.BackoffBase,
		BackoffMax:  retryConfigPtr.BackoffMax,
	}

	// Setup stream printer
	printer := ui.NewStreamPrinter(os.Stdout, ui.WithVerbose(debugMode))

	// Create commit agent with printer for progress output
	agentOpts := agent.CommitAgentOptions{
		Language:    language,
		GitExecutor: gitExec,
		LLMProvider: provider,
		Printer:     printer,
		Output:      os.Stdout,
		Debug:       debugMode,
		RetryConfig: retryConfig,
	}

	commitAgent, err := agent.NewCommitAgent(agentOpts)
	if err != nil {
		return fmt.Errorf("failed to create commit agent: %w", err)
	}

	// Print initial indicator
	_ = printer.PrintThinking("Starting commit message generation...")

	// Generate commit message
	req := agent.CommitRequest{
		Language: language,
		Context:  commitContext,
	}

	response, err := commitAgent.GenerateCommitMessage(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate commit message: %w", err)
	}

	// Use response from agent
	var commitMessage string
	if response != nil && response.CommitInfo != nil {
		commitMessage = response.CommitInfo.Message()
	} else {
		// Fallback - this shouldn't happen with proper agent implementation
		return fmt.Errorf("no commit message generated")
	}

	// Print the generated commit message
	err = ui.ShowCommitMessage(commitMessage, os.Stdout)
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

	// Ask for confirmation (default is Yes)
	if !commitAutoYes {
		confirmed, err := ui.ConfirmWithDefault("\nDo you want to commit with this message?", true, os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Commit cancelled.")
			return nil
		}
	}

	// Execute commit
	err = gitExec.Commit(ctx, commitMessage)
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	fmt.Println("\n✅ Commit created successfully!")
	return nil
}
