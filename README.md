# GitBuddy-Go

<p align="center">
  <strong>🤖 AI-Powered Git Workflow Assistant</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#usage">Usage</a> •
  <a href="#supported-llms">Supported LLMs</a>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README_zh.md">简体中文</a> |
  <a href="README_ja.md">日本語</a>
</p>

---

GitBuddy-Go is an AI-powered command-line tool that automates and enhances your daily Git workflows. It uses Large Language Models (LLMs) with an agentic approach to intelligently analyze your code changes and generate high-quality commit messages, PR descriptions, and development reports.

## Features

- **🎯 Smart Commit Messages**: Automatically generates [Conventional Commits](https://www.conventionalcommits.org/) compliant messages by analyzing staged changes
- **📝 PR Description Generator**: Creates comprehensive pull request descriptions with summary, changes, motivation, and impact analysis
- **🔍 Code Review**: AI-powered code review that identifies bugs, security issues, performance problems, and style suggestions
- **🐛 Issue Debugging**: Interactive AI assistant that systematically analyzes and debugs code issues
- **💬 AI Chat Assistant**: General-purpose conversational AI with access to code and Git tools for flexible exploration
- **📊 Development Reports**: Generates structured weekly/monthly development reports from commit history
- **🔄 Automatic Retry**: Smart retry mechanism with exponential backoff for handling transient LLM API failures
- **💾 Session Management**: Save and resume long-running debug/review sessions with Ctrl+C support
- **🌍 Multi-Language Support**: Generate output in any language (English, Chinese, Japanese, etc.)
- **🔧 Multiple LLM Providers**: Supports OpenAI, DeepSeek, Ollama, Grok, and Google Gemini
- **📡 Real-time Streaming**: See the AI's analysis process in real-time with streaming output
- **🤖 Agentic Workflow**: LLM autonomously uses Git tools to gather context before generating output

## Installation

### Using Go Install (Recommended)

```bash
go install github.com/huimingz/gitbuddy-go/cmd/gitbuddy@latest
```

This will install `gitbuddy` to your `$GOPATH/bin` directory. Make sure it's in your `PATH`.

### From Source

```bash
git clone https://github.com/huimingz/gitbuddy-go.git
cd gitbuddy-go
go build -o gitbuddy ./cmd/gitbuddy
```

### Requirements

- Go 1.21 or later
- Git

## Quick Start

1. **Initialize configuration**:

```bash
gitbuddy init
```

This creates a configuration file at `~/.gitbuddy.yaml`.

2. **Configure your LLM provider** (edit `~/.gitbuddy.yaml`):

```yaml
default_model: deepseek

models:
  deepseek:
    provider: deepseek
    api_key: your-api-key-here
    model: deepseek-chat

language: en  # or zh, ja, etc.
```

3. **Generate a commit message**:

```bash
# Stage your changes first
git add .

# Generate and commit
gitbuddy commit
```

## Configuration

GitBuddy uses a YAML configuration file. The default location is `~/.gitbuddy.yaml`.

### Configuration File Example

```yaml
# Default model to use
default_model: deepseek

# Available models
models:
  deepseek:
    provider: deepseek
    api_key: sk-your-api-key
    model: deepseek-chat
    base_url: https://api.deepseek.com/v1  # optional

  openai:
    provider: openai
    api_key: sk-your-openai-key
    model: gpt-4o

  ollama:
    provider: ollama
    model: qwen2.5:14b
    base_url: http://localhost:11434/v1

  gemini:
    provider: gemini
    api_key: your-gemini-api-key
    model: gemini-2.0-flash

# Default output language
language: en

# Code review settings (optional)
review:
  max_lines_per_read: 1000      # Maximum lines to read per file operation
  grep_max_file_size: 10        # Maximum file size for grep in MB
  grep_timeout: 10              # Grep operation timeout in seconds
  grep_max_results: 100         # Maximum number of grep results

# Debug settings (optional)
debug:
  issues_dir: ./issues           # Directory to save debug reports
  max_iterations: 50             # Maximum agent iterations before asking to continue
  enable_compression: true       # Enable message history compression
  compression_threshold: 20      # Compress when message count exceeds this
  compression_keep_recent: 10    # Number of recent messages to keep after compression
  show_compression_summary: false # Show compression summary to user (default: false)
  max_lines_per_read: 1000       # Maximum lines to read per file operation
  grep_max_file_size: 10         # Maximum file size for grep in MB
  grep_timeout: 10               # Grep operation timeout in seconds
  grep_max_results: 100          # Maximum number of grep results

# Retry settings (optional)
retry:
  enabled: true                  # Enable automatic retry for LLM API calls
  max_attempts: 3                # Maximum number of retry attempts
  backoff_base: 1.0              # Base backoff duration in seconds
  backoff_max: 30.0              # Maximum backoff duration in seconds

# Session settings (optional)
session:
  save_dir: ~/.gitbuddy/sessions # Directory to save session files
  auto_save: true                # Automatically save sessions on interruption
  max_sessions: 50               # Maximum number of sessions to keep
```

### Configuration Priority

1. Command-line flags (highest priority)
2. Configuration file
3. Environment variables
4. Default values

## Usage

### Generate Commit Message

```bash
# Basic usage - analyzes staged changes and generates commit message
gitbuddy commit

# Specify language
gitbuddy commit -l zh

# Provide additional context
gitbuddy commit -c "This fixes the login bug reported in issue #123"

# Use a specific model
gitbuddy commit -m openai

# Auto-confirm without prompting
gitbuddy commit -y
```

### Generate PR Description

```bash
# Compare current branch with main
gitbuddy pr --base main

# Specify language and context
gitbuddy pr --base develop -l zh -c "Performance optimization for API endpoints"

# Use a specific model
gitbuddy pr --base main -m gemini
```

### Generate Development Report

```bash
# Generate report for date range
gitbuddy report --since 2024-12-01 --until 2024-12-31

# Filter by author
gitbuddy report --since 2024-12-01 --author "john@example.com"

# Specify language
gitbuddy report --since 2024-12-01 -l zh
```

### Code Review

```bash
# Review all staged changes
gitbuddy review

# Review with additional context
gitbuddy review -c "This is an authentication module"

# Review specific files only
gitbuddy review --files "auth.go,crypto.go"

# Only show errors (filter out warnings and info)
gitbuddy review --severity error

# Focus on security and performance issues
gitbuddy review --focus security,performance

# Review in Chinese
gitbuddy review -l zh
```

The review command identifies:
- 🔴 **Errors**: Bugs, crashes, critical issues
- 🟡 **Warnings**: Potential bugs, performance issues
- 🔵 **Info**: Style suggestions, refactoring opportunities

### Debug Issues

```bash
# Debug a specific issue with AI assistance
gitbuddy debug "Login fails with 500 error"

# Provide additional context
gitbuddy debug "Memory leak in background worker" -c "Happens after 24 hours of runtime"

# Focus on specific files
gitbuddy debug "Test TestUserAuth is failing" --files "auth_test.go,auth.go"

# Enable interactive mode (agent can ask for your input)
gitbuddy debug "API returns wrong data" --interactive

# Debug in Chinese with interactive mode
gitbuddy debug "性能问题" -l zh --interactive

# Specify custom issues directory
gitbuddy debug "Database connection timeout" --issues-dir ./debug-reports

# Set maximum iterations
gitbuddy debug "Complex issue" --max-iterations 50

# In interactive mode, you'll be asked if you want to continue when max iterations is reached

# Resume a previously interrupted session
gitbuddy debug --resume debug-20240127-120000-abc123
```

The debug command:
- 🔍 **Systematically analyzes** the issue using file system, search, and Git tools
- 🤖 **Autonomously explores** the codebase to understand the problem
- 💬 **Interactively asks** for your input when needed (with `--interactive` flag)
- 📋 **Generates detailed reports** with root cause analysis and fix suggestions
- 💾 **Saves reports** to the `./issues` directory for future reference
- 🔄 **Supports session resume**: Press Ctrl+C to interrupt, then resume later with `--resume`

### Session Management

```bash
# List all saved sessions
gitbuddy sessions list

# Show details of a specific session
gitbuddy sessions show debug-20240127-120000-abc123

# Delete a session
gitbuddy sessions delete debug-20240127-120000-abc123

# Clean up old sessions, keeping only the 10 most recent
gitbuddy sessions clean --max 10
```

Sessions are automatically saved when you interrupt a debug or review command with Ctrl+C. You can resume them later using the `--resume` flag.

### Other Commands

```bash
# Show version information
gitbuddy version

# List configured models
gitbuddy models list

# Initialize configuration file
gitbuddy init
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `~/.gitbuddy.yaml`) |
| `--debug` | Enable debug mode for verbose output |
| `-m, --model` | Specify which LLM model to use |

## Supported LLMs

| Provider | Models | Notes |
|----------|--------|-------|
| **DeepSeek** | deepseek-chat, deepseek-reasoner | Recommended for best price/performance |
| **OpenAI** | gpt-4o, gpt-4o-mini, gpt-3.5-turbo | Requires OpenAI API key |
| **Ollama** | Any local model | Runs locally, no API key needed |
| **Grok** | grok-beta | Requires xAI API key |
| **Gemini** | gemini-2.0-flash, gemini-1.5-pro | Requires Google AI API key |

## How It Works

GitBuddy uses an **agentic approach** where the LLM autonomously decides which Git commands to execute:

1. **For Commit Messages**:
   - LLM calls `git status` to get an overview
   - LLM calls `git diff --cached` to analyze changes
   - Optionally calls `git log` for context
   - Generates structured commit message via `submit_commit` tool

2. **For PR Descriptions**:
   - LLM calls `git log` to see commits between branches
   - LLM calls `git diff` to analyze code changes
   - Generates PR description via `submit_pr` tool

3. **For Reports**:
   - LLM calls `git log` with date filters
   - Analyzes and categorizes commits
   - Generates report via `submit_report` tool

4. **For Code Review**:
   - LLM calls `git diff --cached` to analyze staged changes
   - LLM uses `grep_file` to quickly locate specific functions or patterns in files
   - LLM uses `grep_directory` to find code patterns across multiple files
   - LLM calls `read_file` to examine complete source code context when needed
   - Identifies bugs, security issues, performance problems
   - Generates review via `submit_review` tool

This agentic approach allows the LLM to gather exactly the context it needs, resulting in more accurate and relevant output.

## Automatic Retry and Error Handling

GitBuddy includes intelligent retry mechanisms to handle transient LLM API failures:

- **Smart Error Classification**: Automatically distinguishes between retryable errors (network issues, timeouts, 503, 429) and non-retryable errors (400, 401, context exceeded)
- **Exponential Backoff**: Implements exponential backoff strategy to avoid overwhelming the API
- **Configurable Retries**: Customize retry behavior through configuration (max attempts, backoff duration)
- **User-Friendly Messages**: Clear feedback when retries are happening

When an LLM API call fails with a retryable error, GitBuddy will automatically retry with increasing delays between attempts.

## Debug Mode

Enable debug mode to see detailed information:

```bash
gitbuddy commit --debug
```

This shows:
- Configuration details
- LLM provider and model being used
- Tool calls and their results
- Retry attempts and backoff timings
- Token usage statistics
- Execution time

## Proxy Support

GitBuddy respects standard proxy environment variables:

```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [CloudWeGo Eino](https://github.com/cloudwego/eino) - AI Agent framework
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management

