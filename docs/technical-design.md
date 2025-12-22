# GitBuddy-Go 技术方案设计文档

## 1. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Layer                            │
│                    (cobra framework)                        │
├─────────────────────────────────────────────────────────────┤
│                      Command Handlers                       │
│              commit / pr / report handlers                  │
├─────────────────────────────────────────────────────────────┤
│                       Agent Layer                           │
│                  (Eino Agent + Tools)                       │
├──────────────────┬──────────────────┬───────────────────────┤
│   Git Tools      │   Commit Tool    │    LLM Provider       │
│  (diff/log/...)  │  (execute commit)│    (多模型支持)        │
├──────────────────┴──────────────────┴───────────────────────┤
│                      Config Layer                           │
│                (viper configuration)                        │
├─────────────────────────────────────────────────────────────┤
│                       Git Layer                             │
│                  (git command executor)                     │
└─────────────────────────────────────────────────────────────┘
```

## 2. 项目结构

```
gitbuddy-go/
├── cmd/
│   └── gitbuddy/
│       └── main.go              # 程序入口
├── internal/
│   ├── cli/
│   │   ├── root.go              # 根命令（含全局选项）
│   │   ├── commit.go            # commit 子命令
│   │   ├── init.go              # init 子命令（初始化配置）
│   │   ├── models.go            # models 子命令（查看模型列表）
│   │   ├── version.go           # version 子命令
│   │   ├── pr.go                # pr 子命令（P1）
│   │   └── report.go            # report 子命令（P2）
│   ├── agent/
│   │   ├── agent.go             # Agent 封装
│   │   ├── prompts.go           # System Prompts
│   │   └── tools/
│   │       ├── git_diff.go      # git diff 工具
│   │       ├── git_status.go    # git status 工具
│   │       ├── git_log.go       # git log 工具
│   │       ├── git_commit.go    # git commit 工具
│   │       └── tools.go         # 工具注册
│   ├── llm/
│   │   ├── provider.go          # LLM Provider 接口
│   │   ├── factory.go           # Provider 工厂
│   │   ├── openai.go            # OpenAI 实现
│   │   ├── deepseek.go          # Deepseek 实现
│   │   ├── ollama.go            # Ollama 实现
│   │   ├── gemini.go            # Gemini 实现
│   │   └── grok.go              # Grok 实现
│   ├── git/
│   │   ├── executor.go          # git 命令执行器
│   │   └── executor_test.go
│   ├── config/
│   │   ├── config.go            # 配置加载
│   │   └── config_test.go
│   └── ui/
│       ├── confirm.go           # 确认交互
│       └── spinner.go           # 加载动画
├── pkg/
│   └── lang/
│       └── language.go          # 语言定义
├── go.mod
├── go.sum
├── .gitbuddy.yaml.example       # 配置文件示例
└── README.md
```

## 3. 核心模块设计

### 3.1 Git 执行器 (`internal/git/executor.go`)

```go
package git

import (
    "context"
)

// Executor git 命令执行器接口
type Executor interface {
    // DiffCached 获取暂存区的 diff
    DiffCached(ctx context.Context) (string, error)
    
    // DiffBranches 获取两个分支之间的 diff
    DiffBranches(ctx context.Context, base, head string) (string, error)
    
    // Status 获取当前状态
    Status(ctx context.Context) (string, error)
    
    // Log 获取提交日志
    Log(ctx context.Context, opts LogOptions) (string, error)
    
    // Commit 执行提交
    Commit(ctx context.Context, message string) error
    
    // CurrentBranch 获取当前分支名
    CurrentBranch(ctx context.Context) (string, error)
    
    // CurrentUser 获取当前 git 用户
    CurrentUser(ctx context.Context) (string, error)
}

type LogOptions struct {
    Author string
    Since  string
    Until  string
    Format string
    Count  int
}

// DefaultExecutor 默认实现
type DefaultExecutor struct {
    workDir string
}

func NewExecutor(workDir string) *DefaultExecutor {
    return &DefaultExecutor{workDir: workDir}
}
```

### 3.2 配置管理 (`internal/config/config.go`)

```go
package config

type Config struct {
    DefaultModel string                 `yaml:"default_model" mapstructure:"default_model"`
    Models       map[string]ModelConfig `yaml:"models" mapstructure:"models"`
    Language     string                 `yaml:"language" mapstructure:"language"`
}

type ModelConfig struct {
    Provider string `yaml:"provider" mapstructure:"provider"`
    APIKey   string `yaml:"api_key" mapstructure:"api_key"`
    Model    string `yaml:"model" mapstructure:"model"`
    BaseURL  string `yaml:"base_url" mapstructure:"base_url"`
}

// Load 加载配置（优先级：命令行 > 环境变量 > 配置文件 > 默认值）
func Load() (*Config, error)

// GetModel 获取指定的模型配置
func (c *Config) GetModel(modelName string) (*ModelConfig, error)
```

### 3.3 LLM Provider (`internal/llm/provider.go`)

```go
package llm

import (
    "context"
    "github.com/cloudwego/eino/components/model"
)

// Provider LLM 提供商接口
type Provider interface {
    // Name 返回提供商名称
    Name() string
    
    // CreateChatModel 创建 Eino ChatModel
    CreateChatModel(ctx context.Context) (model.ChatModel, error)
}

// ModelConfig 模型配置
type ModelConfig struct {
    Provider string `yaml:"provider" mapstructure:"provider"`
    APIKey   string `yaml:"api_key" mapstructure:"api_key"`
    Model    string `yaml:"model" mapstructure:"model"`
    BaseURL  string `yaml:"base_url" mapstructure:"base_url"`
}
```

### 3.4 Provider 工厂 (`internal/llm/factory.go`)

```go
package llm

import "fmt"

// ProviderFactory 创建 Provider 的工厂
type ProviderFactory struct{}

func NewProviderFactory() *ProviderFactory {
    return &ProviderFactory{}
}

func (f *ProviderFactory) Create(cfg ModelConfig) (Provider, error) {
    switch cfg.Provider {
    case "openai":
        return NewOpenAIProvider(cfg), nil
    case "deepseek":
        return NewDeepseekProvider(cfg), nil
    case "ollama":
        return NewOllamaProvider(cfg), nil
    case "gemini":
        return NewGeminiProvider(cfg), nil
    case "grok":
        return NewGrokProvider(cfg), nil
    default:
        return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
    }
}
```

### 3.5 各 Provider 实现

```go
// OpenAI Provider（也作为 Deepseek/Ollama/Grok 的基础）
type OpenAIProvider struct {
    cfg ModelConfig
}

func NewOpenAIProvider(cfg ModelConfig) *OpenAIProvider {
    return &OpenAIProvider{cfg: cfg}
}

func (p *OpenAIProvider) Name() string {
    return "openai"
}

func (p *OpenAIProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
    // 使用 Eino 的 OpenAI 组件
    return openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey:  p.cfg.APIKey,
        Model:   p.cfg.Model,
        BaseURL: p.cfg.BaseURL,
    })
}

// Deepseek Provider（基于 OpenAI 兼容接口）
type DeepseekProvider struct {
    *OpenAIProvider
}

func NewDeepseekProvider(cfg ModelConfig) *DeepseekProvider {
    if cfg.BaseURL == "" {
        cfg.BaseURL = "https://api.deepseek.com/v1"
    }
    return &DeepseekProvider{OpenAIProvider: NewOpenAIProvider(cfg)}
}

func (p *DeepseekProvider) Name() string {
    return "deepseek"
}

// Ollama Provider（基于 OpenAI 兼容接口）
type OllamaProvider struct {
    *OpenAIProvider
}

func NewOllamaProvider(cfg ModelConfig) *OllamaProvider {
    if cfg.BaseURL == "" {
        cfg.BaseURL = "http://localhost:11434/v1"
    }
    if cfg.APIKey == "" {
        cfg.APIKey = "ollama" // Ollama 不需要 API Key
    }
    return &OllamaProvider{OpenAIProvider: NewOpenAIProvider(cfg)}
}

func (p *OllamaProvider) Name() string {
    return "ollama"
}

// Grok Provider（基于 OpenAI 兼容接口）
type GrokProvider struct {
    *OpenAIProvider
}

func NewGrokProvider(cfg ModelConfig) *GrokProvider {
    if cfg.BaseURL == "" {
        cfg.BaseURL = "https://api.x.ai/v1"
    }
    return &GrokProvider{OpenAIProvider: NewOpenAIProvider(cfg)}
}

func (p *GrokProvider) Name() string {
    return "grok"
}

// Gemini Provider
type GeminiProvider struct {
    cfg ModelConfig
}

func NewGeminiProvider(cfg ModelConfig) *GeminiProvider {
    return &GeminiProvider{cfg: cfg}
}

func (p *GeminiProvider) Name() string {
    return "gemini"
}

func (p *GeminiProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
    // 使用 eino-ext 的 Gemini 组件
    // 参考: https://github.com/cloudwego/eino-ext/tree/main/components/model/gemini
    return gemini.NewChatModel(ctx, &gemini.ChatModelConfig{
        APIKey: p.cfg.APIKey,
        Model:  p.cfg.Model,
    })
}
```

### 3.6 Agent 封装 (`internal/agent/agent.go`)

```go
package agent

import (
    "context"
)

type CommitAgent struct {
    llmProvider llm.Provider
    gitExecutor git.Executor
}

type CommitRequest struct {
    Language string // 输出语言
    Context  string // 用户提供的上下文信息（可选）
}

type CommitResponse struct {
    Message     string // 完整的 commit message
    Title       string // commit 标题（首行）
    Body        string // commit body（可选）
    Type        string // commit 类型 (feat/fix/...)
    Scope       string // commit 范围（可选）
    Description string // 简短描述
}

func NewCommitAgent(provider llm.Provider, executor git.Executor) *CommitAgent {
    return &CommitAgent{
        llmProvider: provider,
        gitExecutor: executor,
    }
}

// GenerateCommitMessage 生成 commit message
func (a *CommitAgent) GenerateCommitMessage(ctx context.Context, req CommitRequest) (*CommitResponse, error)
```

### 3.7 Agent Tools 设计

#### GitDiffCached Tool
```go
// 工具名称: git_diff_cached
// 描述: 获取暂存区的 diff 内容
// 参数: 无
// 返回: diff 字符串
```

#### GitStatus Tool
```go
// 工具名称: git_status
// 描述: 获取当前 git 仓库状态
// 参数: 无
// 返回: status 字符串
```

#### GitLog Tool
```go
// 工具名称: git_log
// 描述: 获取最近的提交日志
// 参数: 
//   - count: 获取的条数（默认 5）
// 返回: log 字符串
```

#### SubmitCommit Tool (重要)
```go
// 工具名称: submit_commit
// 描述: 提交结构化的 commit 信息，确保 LLM 输出的内容只包含 commit 相关信息
// 参数:
//   - type: commit 类型 (required) - feat, fix, docs, style, refactor, perf, test, chore, build, ci, revert
//   - scope: commit 范围 (optional) - 如 auth, api, ui
//   - description: 简短描述 (required) - 祈使语气，不以句号结尾，50字符以内
//   - body: 详细描述 (optional) - 解释 what 和 why
//   - footer: 页脚 (optional) - 用于 breaking changes 或 issue 引用
// 返回: 格式化后的 commit message

// 使用 Tool 调用的原因：
// 1. 确保 LLM 输出的是结构化数据，而不是混杂其他描述文本
// 2. 可以在代码层面验证 commit 信息的格式
// 3. 便于后续处理和格式化
```

#### GitCommit Tool
```go
// 工具名称: git_commit
// 描述: 执行 git commit（在用户确认后调用）
// 参数:
//   - message: 完整的 commit message
// 返回: 执行结果
// 注意: 此工具在用户确认 submit_commit 的结果后才会被调用
```

### 3.8 System Prompt 设计

```go
const CommitSystemPrompt = `You are a Git commit message generator. Your task is to analyze code changes and generate commit messages following the Conventional Commits specification.

## Conventional Commits Format
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]

## Types
- feat: A new feature
- fix: A bug fix
- docs: Documentation only changes
- style: Changes that do not affect the meaning of the code
- refactor: A code change that neither fixes a bug nor adds a feature
- perf: A code change that improves performance
- test: Adding missing tests or correcting existing tests
- chore: Changes to the build process or auxiliary tools

## Rules
1. The description should be concise (50 chars or less preferred)
2. Use imperative mood ("add" not "added")
3. Do not end the description with a period
4. The body should explain what and why (not how)

## Output Language
Generate the commit message in: {{.Language}}

{{if .Context}}
## Additional Context
The developer has provided the following context for this change:
"{{.Context}}"

Please consider this context when generating the commit message. It provides important information that may not be obvious from the code diff alone.
{{end}}

## Process
1. First, use git_diff_cached tool to get the staged changes
2. Optionally use git_status or git_log for more context
3. Analyze the changes and the developer's context (if provided)
4. Use the submit_commit tool to submit the structured commit information

## IMPORTANT
- You MUST use the submit_commit tool to submit the commit information
- Do NOT output the commit message as plain text
- The submit_commit tool accepts structured parameters: type, scope, description, body, footer
- This ensures the commit message is properly formatted and validated
`
```

### 3.9 用户交互 (`internal/ui/`)

```go
package ui

// ConfirmCommit 显示 commit message 并等待用户确认
// 当前只支持 Y/N 确认，未来可能支持编辑功能
func ConfirmCommit(message string) (confirmed bool, err error)

// ShowSpinner 显示加载动画
func ShowSpinner(message string) func()

// StreamPrinter 流式输出打印器
type StreamPrinter struct {
    writer io.Writer
}

// PrintThinking 输出思考/规划信息
func (p *StreamPrinter) PrintThinking(content string)

// PrintToolCall 输出工具调用信息
func (p *StreamPrinter) PrintToolCall(toolName string, args string)

// PrintToolResult 输出工具执行结果
func (p *StreamPrinter) PrintToolResult(toolName string, result string)

// PrintContent 输出 LLM 生成的内容（流式）
func (p *StreamPrinter) PrintContent(content string)

// PrintStats 输出统计信息（token 消耗和用时）
func (p *StreamPrinter) PrintStats(stats *ExecutionStats)
```

### 3.10 流式输出设计

#### 3.10.1 执行统计信息

```go
package agent

// ExecutionStats 执行统计信息
type ExecutionStats struct {
    StartTime     time.Time     // 开始时间
    EndTime       time.Time     // 结束时间
    Duration      time.Duration // 总耗时
    PromptTokens  int           // 输入 token 数
    OutputTokens  int           // 输出 token 数
    TotalTokens   int           // 总 token 数
    ToolCalls     int           // 工具调用次数
}

func (s *ExecutionStats) String() string {
    return fmt.Sprintf(
        "Duration: %s | Tokens: %d (prompt: %d, output: %d) | Tool calls: %d",
        s.Duration.Round(time.Millisecond),
        s.TotalTokens,
        s.PromptTokens,
        s.OutputTokens,
        s.ToolCalls,
    )
}
```

#### 3.10.2 流式回调处理

```go
package agent

import (
    "github.com/cloudwego/eino/callbacks"
)

// StreamCallback 流式输出回调
type StreamCallback struct {
    printer *ui.StreamPrinter
    stats   *ExecutionStats
}

func NewStreamCallback(printer *ui.StreamPrinter) *StreamCallback {
    return &StreamCallback{
        printer: printer,
        stats:   &ExecutionStats{StartTime: time.Now()},
    }
}

// OnLLMStart LLM 开始调用
func (c *StreamCallback) OnLLMStart(ctx context.Context, input *schema.Message) {
    // 记录开始
}

// OnLLMContentChunk LLM 内容流式输出
func (c *StreamCallback) OnLLMContentChunk(ctx context.Context, chunk string) {
    c.printer.PrintContent(chunk)
}

// OnLLMEnd LLM 调用结束
func (c *StreamCallback) OnLLMEnd(ctx context.Context, output *schema.Message, usage *schema.TokenUsage) {
    c.stats.PromptTokens += usage.PromptTokens
    c.stats.OutputTokens += usage.CompletionTokens
    c.stats.TotalTokens += usage.TotalTokens
}

// OnToolStart 工具开始调用
func (c *StreamCallback) OnToolStart(ctx context.Context, toolName string, args string) {
    c.printer.PrintToolCall(toolName, args)
    c.stats.ToolCalls++
}

// OnToolEnd 工具调用结束
func (c *StreamCallback) OnToolEnd(ctx context.Context, toolName string, result string) {
    c.printer.PrintToolResult(toolName, result)
}

// GetStats 获取统计信息
func (c *StreamCallback) GetStats() *ExecutionStats {
    c.stats.EndTime = time.Now()
    c.stats.Duration = c.stats.EndTime.Sub(c.stats.StartTime)
    return c.stats
}
```

#### 3.10.3 终端输出样式

```
$ gitbuddy commit -c "添加用户认证功能"

🤔 Analyzing staged changes...

🔧 Calling tool: git_diff_cached
   ├─ Getting staged changes...
   └─ Done (15 files changed, +342 -28)

🔧 Calling tool: git_log
   ├─ args: {"count": 5}
   └─ Done

💭 Generating commit message...

Based on the changes, this commit adds user authentication functionality including:
- JWT token generation and validation
- Password hashing with bcrypt
- Login/logout API endpoints

📝 Generated commit message:
┌────────────────────────────────────────────────────────────┐
│ feat(auth): add user authentication with JWT               │
│                                                            │
│ - Implement JWT token generation and validation            │
│ - Add password hashing using bcrypt                        │
│ - Create login/logout API endpoints                        │
│ - Add auth middleware for protected routes                 │
└────────────────────────────────────────────────────────────┘

📊 Stats: Duration: 3.2s | Tokens: 1,234 (prompt: 892, output: 342) | Tool calls: 2

? Confirm commit? [Y/n]: 
```

### 3.11 版本信息命令 (`internal/cli/version.go`)

```go
package cli

import (
    "fmt"
    "runtime"
    
    "github.com/spf13/cobra"
)

// 编译时注入的版本信息
var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildTime = "unknown"
)

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print version information",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("GitBuddy %s\n", Version)
        fmt.Printf("  Git Commit: %s\n", GitCommit)
        fmt.Printf("  Build Time: %s\n", BuildTime)
        fmt.Printf("  Go Version: %s\n", runtime.Version())
        fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
    },
}
```

**编译时注入版本信息**：

```bash
go build -ldflags "-X main.Version=v1.0.0 -X main.GitCommit=$(git rev-parse HEAD) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o gitbuddy ./cmd/gitbuddy
```

**命令用法**：

```bash
$ gitbuddy version
GitBuddy v1.0.0
  Git Commit: abc1234
  Build Time: 2024-01-15T10:30:00Z
  Go Version: go1.21.5
  OS/Arch:    darwin/arm64
```

### 3.12 配置初始化命令 (`internal/cli/init.go`)

```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"
    
    "github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize GitBuddy configuration file",
    Long:  "Create a default .gitbuddy.yaml configuration file in the home directory",
    RunE:  runInit,
}

var forceOverwrite bool

func init() {
    initCmd.Flags().BoolVarP(&forceOverwrite, "force", "f", false, "Overwrite existing configuration file")
}

func runInit(cmd *cobra.Command, args []string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("failed to get home directory: %w", err)
    }
    
    configPath := filepath.Join(homeDir, ".gitbuddy.yaml")
    
    // 检查文件是否已存在
    if _, err := os.Stat(configPath); err == nil && !forceOverwrite {
        return fmt.Errorf("configuration file already exists: %s\nUse --force to overwrite", configPath)
    }
    
    // 写入默认配置
    if err := os.WriteFile(configPath, []byte(defaultConfigTemplate), 0644); err != nil {
        return fmt.Errorf("failed to write configuration file: %w", err)
    }
    
    fmt.Printf("✅ Configuration file created: %s\n", configPath)
    fmt.Println("\nNext steps:")
    fmt.Println("  1. Edit the configuration file to add your API keys")
    fmt.Println("  2. Set environment variables for API keys (recommended)")
    fmt.Println("  3. Run 'gitbuddy models list' to verify your configuration")
    
    return nil
}

const defaultConfigTemplate = `# GitBuddy Configuration File
# Documentation: https://github.com/huimingz/gitbuddy-go

# Default model to use (must match a key in the models section)
default_model: deepseek

# Model configurations
models:
  # Deepseek (recommended for Chinese users)
  deepseek:
    provider: deepseek
    api_key: ${DEEPSEEK_API_KEY}
    model: deepseek-chat
    # base_url: https://api.deepseek.com/v1  # optional, has default value

  # OpenAI GPT-4
  # gpt4:
  #   provider: openai
  #   api_key: ${OPENAI_API_KEY}
  #   model: gpt-4o

  # OpenAI GPT-4 Mini (cheaper)
  # gpt4-mini:
  #   provider: openai
  #   api_key: ${OPENAI_API_KEY}
  #   model: gpt-4o-mini

  # Local Ollama
  # ollama:
  #   provider: ollama
  #   model: qwen2.5:14b
  #   base_url: http://localhost:11434/v1

  # Google Gemini
  # gemini:
  #   provider: gemini
  #   api_key: ${GEMINI_API_KEY}
  #   model: gemini-1.5-pro

  # xAI Grok
  # grok:
  #   provider: grok
  #   api_key: ${XAI_API_KEY}
  #   model: grok-beta
  #   base_url: https://api.x.ai/v1

# Default output language (en, zh, zh-tw, ja, ko)
language: en
`
```

**命令用法**：

```bash
# 创建配置文件
$ gitbuddy init
✅ Configuration file created: /Users/xxx/.gitbuddy.yaml

Next steps:
  1. Edit the configuration file to add your API keys
  2. Set environment variables for API keys (recommended)
  3. Run 'gitbuddy models list' to verify your configuration

# 强制覆盖已有配置
$ gitbuddy init --force
```

### 3.13 模型列表命令 (`internal/cli/models.go`)

```go
// ModelsCmd 模型管理命令
var modelsCmd = &cobra.Command{
    Use:   "models",
    Short: "Manage and list configured models",
}

// ListModelsCmd 列出所有已配置的模型
var listModelsCmd = &cobra.Command{
    Use:   "list",
    Short: "List all configured models",
    RunE:  runListModels,
}

func runListModels(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }
    
    // 输出格式示例:
    // Available models:
    //   * deepseek (default)     - provider: deepseek, model: deepseek-chat
    //     gpt4                   - provider: openai, model: gpt-4o
    //     ollama-qwen            - provider: ollama, model: qwen2.5:14b
    //     gemini                 - provider: gemini, model: gemini-1.5-pro
    //     grok                   - provider: grok, model: grok-beta
    
    return nil
}
```

**命令用法**：

```bash
# 列出所有已配置的模型
gitbuddy models list

# 输出示例
Available models:
  * deepseek (default)     - provider: deepseek, model: deepseek-chat
    gpt4                   - provider: openai, model: gpt-4o
    gpt4-mini              - provider: openai, model: gpt-4o-mini
    ollama-qwen            - provider: ollama, model: qwen2.5:14b
    gemini                 - provider: gemini, model: gemini-1.5-pro
    grok                   - provider: grok, model: grok-beta
```

## 4. 配置文件设计

```yaml
# ~/.gitbuddy.yaml

# 默认使用的模型（对应 models 中的 key）
default_model: deepseek

# 模型配置列表
models:
  # Deepseek 配置
  deepseek:
    provider: deepseek
    api_key: ${DEEPSEEK_API_KEY}
    model: deepseek-chat
    base_url: https://api.deepseek.com/v1
  
  # OpenAI 配置
  gpt4:
    provider: openai
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o
    base_url: ""
  
  gpt4-mini:
    provider: openai
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini
  
  # Ollama 本地模型
  ollama-qwen:
    provider: ollama
    model: qwen2.5:14b
    base_url: http://localhost:11434/v1
  
  # Google Gemini
  gemini:
    provider: gemini
    api_key: ${GEMINI_API_KEY}
    model: gemini-1.5-pro
  
  # xAI Grok
  grok:
    provider: grok
    api_key: ${XAI_API_KEY}
    model: grok-beta
    base_url: https://api.x.ai/v1

# 默认语言
language: en
```

## 5. 工作流程

### 5.1 Commit 命令工作流

```
用户执行: gitbuddy commit [-c <context>] [-m <model>] [-l <lang>]
              │
              ▼
┌─────────────────────────────┐
│  1. 加载配置                 │
│     - 解析命令行参数          │
│     - 加载配置文件            │
│     - 确定使用的模型和语言     │
└─────────────────────────────┘
              │
              ▼
┌─────────────────────────────┐
│  2. 创建 LLM Provider       │
│     - 根据模型配置创建 Provider│
└─────────────────────────────┘
              │
              ▼
┌─────────────────────────────┐
│  3. 创建 CommitAgent        │
│     - 初始化 Eino Agent      │
│     - 注册 Git Tools         │
│     - 设置 System Prompt     │
└─────────────────────────────┘
              │
              ▼
┌─────────────────────────────┐
│  4. Agent 执行任务           │
│     - 调用 git_diff_cached   │
│     - 可选调用其他工具        │
│     - 生成 commit message    │
└─────────────────────────────┘
              │
              ▼
┌─────────────────────────────┐
│  5. 显示生成的 message       │
│     等待用户确认              │
└─────────────────────────────┘
              │
         ┌────┴────┐
         │ 确认?   │
         └────┬────┘
        Yes   │   No
         ▼    │    ▼
┌─────────┐   │  ┌─────────┐
│执行commit│   │  │  取消    │
└─────────┘   │  └─────────┘
              │
              ▼
┌─────────────────────────────┐
│  6. 显示结果                 │
└─────────────────────────────┘
```

## 6. 依赖库

| 库 | 用途 | 版本 |
|---|------|-----|
| github.com/cloudwego/eino | AI Agent 框架 | latest |
| github.com/cloudwego/eino-ext | Eino 扩展组件 | latest |
| github.com/spf13/cobra | CLI 框架 | v1.8+ |
| github.com/spf13/viper | 配置管理 | v1.18+ |
| github.com/fatih/color | 终端颜色输出 | v1.16+ |
| github.com/briandowns/spinner | 加载动画 | v1.23+ |

## 7. 错误处理

| 场景 | 处理方式 |
|-----|---------|
| 暂存区为空 | 提示用户先 `git add` |
| 不在 git 仓库 | 提示 "not a git repository" |
| 模型未配置 | 提示用户配置模型 |
| LLM API 失败 | 显示错误信息，建议检查配置 |
| 用户取消确认 | 正常退出，不执行 commit |
| git commit 失败 | 显示 git 错误信息 |

## 8. 测试策略

### 8.1 单元测试
- Git Executor：使用 mock 或临时 git 仓库
- Config：测试各优先级加载
- LLM Provider：测试工厂创建逻辑
- Agent Tools：mock LLM 响应

### 8.2 集成测试
- 完整的 commit 流程测试（使用临时仓库 + mock LLM）
- 配置文件加载测试

## 9. P0 阶段（Commit 功能）开发任务拆分

| 序号 | 任务 | 预估 |
|-----|------|-----|
| 1 | 项目初始化（go mod、依赖、目录结构） | 0.5h |
| 2 | Config 模块实现（支持多模型配置、代理支持） | 1.5h |
| 3 | LLM Provider 接口与工厂实现 | 1h |
| 4 | OpenAI/Deepseek/Ollama/Grok Provider 实现 | 1.5h |
| 5 | Gemini Provider 实现（使用 eino-ext） | 0.5h |
| 6 | Git Executor 实现 | 1h |
| 7 | Agent Tools 实现（git_diff_cached, git_status, git_log） | 1.5h |
| 8 | CommitAgent 实现 | 1.5h |
| 9 | git_commit Tool 实现（含用户确认，Y/N） | 1h |
| 10 | 流式输出与回调实现 | 1h |
| 11 | CLI root 命令实现（含 --debug 全局选项） | 0.5h |
| 12 | CLI init 命令实现 | 0.5h |
| 13 | CLI version 命令实现 | 0.5h |
| 14 | CLI commit 命令实现 | 1h |
| 15 | CLI models list 命令实现 | 0.5h |
| 16 | UI 交互（StreamPrinter、confirm） | 1h |
| 17 | 调试日志模块实现 | 0.5h |
| 18 | 集成测试与调试 | 1h |

## 10. 命令行接口设计

```bash
# 初始化配置
gitbuddy init [-f|--force]

# 版本信息
gitbuddy version

# Commit 生成
gitbuddy commit [-c <context>] [-m <model>] [-l <language>] [--debug]

# 模型列表
gitbuddy models list

# PR 描述生成（P1）
gitbuddy pr -b <branch> [-c <context>] [-m <model>] [-l <language>] [--debug]

# 开发报告（P2）
gitbuddy report -s <date> [-u <date>] [-a <name>] [-c <context>] [-m <model>] [-l <language>] [--debug]
```

### 10.1 选项说明

| 选项 | 长选项 | 说明 |
|-----|--------|-----|
| `-c` | `--context` | 提供额外的上下文信息 |
| `-m` | `--model` | 指定使用的模型 |
| `-l` | `--lang` | 指定输出语言 |
| `-b` | `--base` | PR 目标分支 |
| `-s` | `--since` | 报告开始日期 |
| `-u` | `--until` | 报告结束日期 |
| `-a` | `--author` | 报告作者 |
| `-f` | `--force` | 强制覆盖（用于 init 命令） |
| | `--debug` | 调试模式，输出详细日志 |

### 10.2 调试模式

使用 `--debug` 选项可以输出更详细的日志信息，便于排查问题：

```bash
$ gitbuddy commit --debug

[DEBUG] Loading configuration from /Users/xxx/.gitbuddy.yaml
[DEBUG] Using model: deepseek (provider: deepseek, model: deepseek-chat)
[DEBUG] Proxy: http://127.0.0.1:7890
[DEBUG] Creating LLM client...
[DEBUG] System prompt:
        You are a Git commit message generator...
[DEBUG] Sending request to LLM API...
[DEBUG] Request tokens: 892
...
```

调试模式输出的信息包括：
- 配置文件路径和加载的配置
- 使用的模型信息
- 代理设置
- 完整的 System Prompt
- API 请求和响应详情
- Token 使用详情
- 工具调用的完整参数和返回值

### 10.3 代理支持

GitBuddy 支持通过环境变量配置代理，用于访问 LLM API：

```bash
# HTTP 代理
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890

# 或者使用小写
export http_proxy=http://127.0.0.1:7890
export https_proxy=http://127.0.0.1:7890

# 不使用代理的地址（可选）
export NO_PROXY=localhost,127.0.0.1

# 然后运行命令
gitbuddy commit
```

代理读取优先级：
1. `HTTPS_PROXY` / `https_proxy`（用于 HTTPS 请求）
2. `HTTP_PROXY` / `http_proxy`（用于 HTTP 请求）
3. `NO_PROXY` / `no_proxy`（排除列表）

## 11. 技术说明

### 11.1 Eino 框架组件

本项目使用 CloudWeGo 的 Eino 框架及其扩展组件 [eino-ext](https://github.com/cloudwego/eino-ext)：

| 组件类型 | 使用的实现 |
|---------|-----------|
| ChatModel | OpenAI, Gemini, Ollama (via eino-ext) |
| Tool | 自定义 Git 工具 |
| Callbacks | 自定义流式回调 |

### 11.2 流式输出

LLM 的输出采用流式方式，实时在命令行界面显示：

1. **思考/规划信息**：显示 AI 的思考过程
2. **工具调用**：显示调用的工具名称和参数
3. **工具结果**：显示工具执行的结果摘要
4. **生成内容**：实时流式输出生成的内容
5. **统计信息**：最终显示 token 消耗和用时

### 11.3 用户确认机制

当前版本的 commit 确认只支持 Y/N 确认：
- `Y` / `y` / `Enter`：确认执行 commit
- `N` / `n` / `Ctrl+C`：取消操作

未来版本可能支持编辑功能。

### 11.4 终端输出格式

使用 emoji 和颜色增强可读性：

| 符号 | 含义 |
|-----|------|
| 🤔 | 分析/思考中 |
| 🔧 | 工具调用 |
| 💭 | 生成内容 |
| 📝 | 最终结果 |
| 📊 | 统计信息 |
| ✅ | 成功 |
| ❌ | 失败/取消 |

### 11.5 调试日志 (`internal/log/`)

```go
package log

import (
    "fmt"
    "os"
)

var debugMode = false

// SetDebugMode 设置调试模式
func SetDebugMode(enabled bool) {
    debugMode = enabled
}

// IsDebugMode 检查是否为调试模式
func IsDebugMode() bool {
    return debugMode
}

// Debug 输出调试信息（仅在调试模式下）
func Debug(format string, args ...interface{}) {
    if debugMode {
        fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
    }
}

// Info 输出普通信息
func Info(format string, args ...interface{}) {
    fmt.Printf(format+"\n", args...)
}

// Error 输出错误信息
func Error(format string, args ...interface{}) {
    fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
```

### 11.6 代理配置 (`internal/config/proxy.go`)

```go
package config

import (
    "net/http"
    "net/url"
    "os"
)

// GetProxyConfig 获取代理配置
func GetProxyConfig() *url.URL {
    // 按优先级检查环境变量
    proxyEnvVars := []string{
        "HTTPS_PROXY",
        "https_proxy", 
        "HTTP_PROXY",
        "http_proxy",
    }
    
    for _, env := range proxyEnvVars {
        if proxyURL := os.Getenv(env); proxyURL != "" {
            if parsed, err := url.Parse(proxyURL); err == nil {
                return parsed
            }
        }
    }
    
    return nil
}

// GetHTTPClient 获取配置了代理的 HTTP 客户端
func GetHTTPClient() *http.Client {
    transport := &http.Transport{
        Proxy: http.ProxyFromEnvironment,
    }
    
    return &http.Client{
        Transport: transport,
    }
}
```

