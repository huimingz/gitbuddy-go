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
	reportSince    string
	reportUntil    string
	reportAuthor   string
	reportContext  string
	reportLanguage string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate development report",
	Long:  `Generate a structured development report based on commit history within a date range.`,
	RunE:  runReport,
}

func init() {
	reportCmd.Flags().StringVarP(&reportSince, "since", "s", "", "Start date (required, e.g., 2024-01-15)")
	reportCmd.Flags().StringVarP(&reportUntil, "until", "u", "", "End date (optional, defaults to today)")
	reportCmd.Flags().StringVarP(&reportAuthor, "author", "a", "", "Author name (optional, defaults to current git user)")
	reportCmd.Flags().StringVarP(&reportContext, "context", "c", "", "Additional context to help AI generate better report")
	reportCmd.Flags().StringVarP(&reportLanguage, "language", "l", "", "Output language (en, zh, ja, etc.)")

	_ = reportCmd.MarkFlagRequired("since")

	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
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
	language := cfg.GetLanguage(reportLanguage)
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

	// Get author - default to current git user
	author := reportAuthor
	if author == "" {
		author, err = gitExecutor.CurrentUser(ctx)
		if err != nil {
			log.Debug("Failed to get current git user: %v", err)
			// Continue without author filter
		}
	}

	// Get until date - default to today
	until := reportUntil
	if until == "" {
		until = time.Now().Format("2006-01-02")
	}

	// Create stream printer for output
	printer := ui.NewStreamPrinter(os.Stdout, ui.WithVerbose(debugMode))

	// Create Report agent
	reportAgent := agent.NewReportAgent(agent.ReportAgentOptions{
		Language:    language,
		GitExecutor: gitExecutor,
		LLMProvider: provider,
		Printer:     printer,
		Debug:       debugMode,
	})

	// Print initial indicator
	_ = printer.PrintThinking("Starting development report generation...")

	// Generate report
	req := agent.ReportRequest{
		Since:    reportSince,
		Until:    until,
		Author:   author,
		Language: language,
		Context:  reportContext,
	}

	response, err := reportAgent.GenerateReport(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Print the generated report
	err = ui.ShowReport(response, os.Stdout)
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
