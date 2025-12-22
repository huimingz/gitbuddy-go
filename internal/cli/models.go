package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage LLM models",
	Long:  `Commands for managing and listing configured LLM models.`,
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured models",
	Long:  `List all LLM models configured in the configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(cfg.Models) == 0 {
			fmt.Println("No models configured.")
			fmt.Println("\nRun 'gitbuddy init' to create a configuration file.")
			return nil
		}

		bold := color.New(color.Bold)
		green := color.New(color.FgGreen)
		cyan := color.New(color.FgCyan)

		bold.Println("Configured Models:")
		fmt.Println()

		for name, model := range cfg.Models {
			// Check if this is the default model
			isDefault := name == cfg.DefaultModel
			defaultMark := ""
			if isDefault {
				defaultMark = " (default)"
			}

			if isDefault {
				green.Printf("  ✓ %s%s\n", name, defaultMark)
			} else {
				fmt.Printf("    %s\n", name)
			}

			cyan.Printf("      Provider: %s\n", model.Provider)
			cyan.Printf("      Model:    %s\n", model.Model)
			if model.BaseURL != "" {
				cyan.Printf("      Base URL: %s\n", model.BaseURL)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	modelsCmd.AddCommand(modelsListCmd)
	rootCmd.AddCommand(modelsCmd)

	// Suppress unused variable warning
	_ = os.Stdout
}
