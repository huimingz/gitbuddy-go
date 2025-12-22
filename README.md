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
- **📊 Development Reports**: Generates structured weekly/monthly development reports from commit history
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

This agentic approach allows the LLM to gather exactly the context it needs, resulting in more accurate and relevant output.

## Debug Mode

Enable debug mode to see detailed information:

```bash
gitbuddy commit --debug
```

This shows:
- Configuration details
- LLM provider and model being used
- Tool calls and their results
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

