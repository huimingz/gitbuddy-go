package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/huimingz/gitbuddy-go/internal/agent/session"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage agent sessions",
	Long: `Manage agent sessions for resuming interrupted executions.

Available subcommands:
  list   - List all saved sessions
  show   - Show details of a specific session
  delete - Delete a session
  clean  - Clean up old sessions`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	Long: `List all saved sessions with their basic information.

Examples:
  gitbuddy sessions list`,
	RunE: runSessionsList,
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show details of a specific session",
	Long: `Show detailed information about a specific session.

Examples:
  gitbuddy sessions show debug-20240101-120000-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsShow,
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a session",
	Long: `Delete a specific session by its ID.

Examples:
  gitbuddy sessions delete debug-20240101-120000-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsDelete,
}

var (
	sessionsCleanMaxSessions int
)

var sessionsCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up old sessions",
	Long: `Clean up old sessions, keeping only the most recent ones.

Examples:
  gitbuddy sessions clean --max 10`,
	RunE: runSessionsClean,
}

func init() {
	sessionsCleanCmd.Flags().IntVar(&sessionsCleanMaxSessions, "max", 0, "Maximum number of sessions to keep (0 = use config default)")

	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsCleanCmd)
	rootCmd.AddCommand(sessionsCmd)
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sessionConfig := cfg.GetSessionConfig()
	mgr := session.NewManager(sessionConfig.SaveDir)

	sessions, err := mgr.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions found.")
		return nil
	}

	// Print sessions in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION ID\tAGENT\tCREATED\tUPDATED\tITERATIONS")
	fmt.Fprintln(w, "----------\t-----\t-------\t-------\t----------")

	for _, s := range sessions {
		createdTime := s.CreatedAt.Format("2006-01-02 15:04")
		updatedTime := s.UpdatedAt.Format("2006-01-02 15:04")
		iterations := fmt.Sprintf("%d/%d", s.Iterations, s.MaxIterations)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			s.ID, s.AgentType, createdTime, updatedTime, iterations)
	}

	w.Flush()

	fmt.Printf("\nTotal: %d session(s)\n", len(sessions))
	fmt.Printf("Session directory: %s\n", sessionConfig.SaveDir)

	return nil
}

func runSessionsShow(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sessionConfig := cfg.GetSessionConfig()
	mgr := session.NewManager(sessionConfig.SaveDir)

	sess, err := mgr.Load(sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Print session details
	fmt.Println("Session Details")
	fmt.Println("===============")
	fmt.Printf("ID:              %s\n", sess.ID)
	fmt.Printf("Agent Type:      %s\n", sess.AgentType)
	fmt.Printf("Created:         %s\n", sess.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:         %s\n", sess.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("Iterations:      %d / %d\n", sess.IterationCount, sess.MaxIterations)
	fmt.Printf("Messages:        %d\n", len(sess.Messages))

	if sess.TokenUsage.TotalTokens > 0 {
		fmt.Printf("Token Usage:\n")
		fmt.Printf("  Prompt:        %d\n", sess.TokenUsage.PromptTokens)
		fmt.Printf("  Completion:    %d\n", sess.TokenUsage.CompletionTokens)
		fmt.Printf("  Total:         %d\n", sess.TokenUsage.TotalTokens)
	}

	if len(sess.Metadata) > 0 {
		fmt.Printf("Metadata:\n")
		for k, v := range sess.Metadata {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	fmt.Println()
	fmt.Printf("Resume with: gitbuddy %s --resume %s\n", sess.AgentType, sess.ID)

	return nil
}

func runSessionsDelete(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sessionConfig := cfg.GetSessionConfig()
	mgr := session.NewManager(sessionConfig.SaveDir)

	// Check if session exists
	if !mgr.Exists(sessionID) {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Delete session
	if err := mgr.Delete(sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Printf("✓ Session deleted: %s\n", sessionID)

	return nil
}

func runSessionsClean(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sessionConfig := cfg.GetSessionConfig()
	mgr := session.NewManager(sessionConfig.SaveDir)

	// Determine max sessions to keep
	maxSessions := sessionsCleanMaxSessions
	if maxSessions <= 0 {
		maxSessions = sessionConfig.MaxSessions
	}

	// Clean up old sessions
	if err := mgr.CleanupOld(maxSessions); err != nil {
		return fmt.Errorf("failed to clean up sessions: %w", err)
	}

	fmt.Printf("✓ Cleaned up old sessions (kept %d most recent)\n", maxSessions)

	return nil
}
