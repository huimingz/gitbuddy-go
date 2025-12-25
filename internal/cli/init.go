package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultConfigTemplate = `# GitBuddy Configuration File
# See: https://github.com/huimingz/gitbuddy-go

# Default language for generated content (en, zh, ja, etc.)
language: en

# Default model to use (must match a key in the models section)
default_model: deepseek

# LLM Model configurations
models:
  # Deepseek (recommended)
  deepseek:
    provider: deepseek
    api_key: ${DEEPSEEK_API_KEY}
    model: deepseek-chat
    # base_url: https://api.deepseek.com  # optional, uses default
  
  # OpenAI
  # openai:
  #   provider: openai
  #   api_key: ${OPENAI_API_KEY}
  #   model: gpt-4o
  #   base_url: https://api.openai.com/v1
  
  # Ollama (local)
  # ollama:
  #   provider: ollama
  #   model: llama3.2
  #   base_url: http://localhost:11434
  
  # Google Gemini
  # gemini:
  #   provider: gemini
  #   api_key: ${GOOGLE_API_KEY}
  #   model: gemini-2.0-flash-exp
  
  # xAI Grok
  # grok:
  #   provider: grok
  #   api_key: ${XAI_API_KEY}
  #   model: grok-beta

# PR description template (optional)
# Provide a plain text template that the LLM will use as a format example
# pr_template:
#   # Option 1: Inline template
#   template: |
#     ## Summary
#     Brief overview of the changes
#     
#     ## Changes
#     - Change 1
#     - Change 2
#     
#     ## Why
#     Motivation for the changes
#   
#   # Option 2: Load from file
#   # file: ~/.gitbuddy-pr-template.txt
`

var (
	initForce bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize GitBuddy configuration",
	Long: `Create a default configuration file (~/.gitbuddy.yaml).

This command creates a template configuration file with example settings
for various LLM providers. Edit the file to add your API keys and customize settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		configPath := filepath.Join(homeDir, ".gitbuddy.yaml")

		// Check if file exists
		if _, err := os.Stat(configPath); err == nil && !initForce {
			return fmt.Errorf("config file already exists: %s\nUse --force to overwrite", configPath)
		}

		// Write config file
		err = os.WriteFile(configPath, []byte(defaultConfigTemplate), 0600)
		if err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}

		fmt.Printf("✅ Configuration file created: %s\n", configPath)
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Edit the config file and add your API keys")
		fmt.Println("  2. Set environment variables for sensitive keys (recommended)")
		fmt.Println("  3. Run 'gitbuddy commit' to generate a commit message")

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing config file")
	rootCmd.AddCommand(initCmd)
}
