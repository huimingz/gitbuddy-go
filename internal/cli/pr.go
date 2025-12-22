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
	prBaseBranch string
	prContext    string
	prLanguage   string
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Generate PR description",
	Long:  `Generate a pull request title and description based on the diff between current branch and target branch.`,
	RunE:  runPR,
}

func init() {
	prCmd.Flags().StringVarP(&prBaseBranch, "base", "b", "", "Target branch to compare against (required)")
	prCmd.Flags().StringVarP(&prContext, "context", "c", "", "Additional context to help AI generate better description")
	prCmd.Flags().StringVarP(&prLanguage, "language", "l", "", "Output language (en, zh, ja, etc.)")

	_ = prCmd.MarkFlagRequired("base")

	rootCmd.AddCommand(prCmd)
}

func runPR(cmd *cobra.Command, args []string) error {
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
	language := cfg.GetLanguage(prLanguage)
	log.Debug("Using language: %s", language)

	// Create LLM provider
	factory := llm.NewProviderFactory()
	provider, err := factory.Create(*modelConfig)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	log.Debug("LLM provider created successfully")

	// Create git executor
	workDir, _ := os.Getwd()
	gitExecutor := git.NewExecutor(workDir)

	// Get current branch
	currentBranch, err := gitExecutor.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if base branch exists
	if prBaseBranch == currentBranch {
		return fmt.Errorf("base branch cannot be the same as current branch (%s)", currentBranch)
	}

	// Create stream printer for output
	printer := ui.NewStreamPrinter(os.Stdout, ui.WithVerbose(debugMode))

	// Create PR agent
	prAgent := agent.NewPRAgent(agent.PRAgentOptions{
		Language:    language,
		GitExecutor: gitExecutor,
		LLMProvider: provider,
		Printer:     printer,
		Debug:       debugMode,
	})

	// Print initial indicator
	_ = printer.PrintThinking("Starting PR description generation...")

	// Generate PR description
	req := agent.PRRequest{
		BaseBranch: prBaseBranch,
		HeadBranch: currentBranch,
		Language:   language,
		Context:    prContext,
	}

	response, err := prAgent.GeneratePRDescription(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate PR description: %w", err)
	}

	// Print the generated PR description
	err = ui.ShowPRDescription(response, os.Stdout)
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
