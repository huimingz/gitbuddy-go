package cli

import (
	"github.com/huimingz/gitbuddy-go/internal/log"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	debugMode  bool
	configFile string
	modelName  string

	// Version info
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gitbuddy",
	Short: "AI-powered Git assistant for developers",
	Long: `GitBuddy is an AI-powered command-line tool that helps developers with:
  - Generating conventional commit messages
  - Creating PR descriptions
  - Generating development reports

Use "gitbuddy [command] --help" for more information about a command.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set debug mode before any command runs
		if debugMode {
			log.SetDebugMode(true)
			log.Debug("Debug mode enabled")
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets version information from build flags
func SetVersionInfo(v, commit, time string) {
	version = v
	gitCommit = commit
	buildTime = time
}

// GetVersionInfo returns version information
func GetVersionInfo() (string, string, string) {
	return version, gitCommit, buildTime
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug mode for verbose output")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file path (default: ~/.gitbuddy.yaml)")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "LLM model to use (overrides config)")
}
