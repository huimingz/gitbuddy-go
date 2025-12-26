# GitBuddy-Go

<p align="center">
  <strong>🤖 AI 驱动的 Git 工作流助手</strong>
</p>

<p align="center">
  <a href="#功能特性">功能特性</a> •
  <a href="#安装">安装</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#配置">配置</a> •
  <a href="#使用方法">使用方法</a> •
  <a href="#支持的-llm">支持的 LLM</a>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README_zh.md">简体中文</a> |
  <a href="README_ja.md">日本語</a>
</p>

---

GitBuddy-Go 是一个 AI 驱动的命令行工具，用于自动化和增强日常 Git 工作流程。它采用大语言模型（LLM）的 Agent 方式，智能分析代码变更，生成高质量的 commit 信息、PR 描述和开发报告。

## 功能特性

- **🎯 智能 Commit 信息**: 自动分析暂存区变更，生成符合 [Conventional Commits](https://www.conventionalcommits.org/) 规范的提交信息
- **📝 PR 描述生成器**: 创建包含摘要、变更内容、动机和影响分析的完整 PR 描述
- **🔍 代码审查**: AI 驱动的代码审查，识别 bugs、安全隐患、性能问题和代码风格建议
- **🐛 问题排查**: 交互式 AI 助手，系统化地分析和调试代码问题
- **📊 开发报告**: 根据提交历史生成结构化的周报/月报
- **🌍 多语言支持**: 支持任意语言输出（中文、英文、日文等）
- **🔧 多 LLM 支持**: 支持 OpenAI、DeepSeek、Ollama、Grok 和 Google Gemini
- **📡 实时流式输出**: 实时查看 AI 的分析过程
- **🤖 Agent 工作流**: LLM 自主调用 Git 工具收集上下文，再生成输出

## 安装

### 使用 Go Install（推荐）

```bash
go install github.com/huimingz/gitbuddy-go/cmd/gitbuddy@latest
```

这会将 `gitbuddy` 安装到 `$GOPATH/bin` 目录。请确保该目录已添加到 `PATH` 环境变量中。

### 从源码构建

```bash
git clone https://github.com/huimingz/gitbuddy-go.git
cd gitbuddy-go
go build -o gitbuddy ./cmd/gitbuddy
```

### 环境要求

- Go 1.21 或更高版本
- Git

## 快速开始

1. **初始化配置**:

```bash
gitbuddy init
```

这会在 `~/.gitbuddy.yaml` 创建配置文件。

2. **配置 LLM 提供商**（编辑 `~/.gitbuddy.yaml`）:

```yaml
default_model: deepseek

models:
  deepseek:
    provider: deepseek
    api_key: your-api-key-here
    model: deepseek-chat

language: zh  # 默认使用中文
```

3. **生成 commit 信息**:

```bash
# 先暂存变更
git add .

# 生成并提交
gitbuddy commit
```

## 配置

GitBuddy 使用 YAML 配置文件，默认位置为 `~/.gitbuddy.yaml`。

### 配置文件示例

```yaml
# 默认使用的模型
default_model: deepseek

# 可用模型配置
models:
  deepseek:
    provider: deepseek
    api_key: sk-your-api-key
    model: deepseek-chat
    base_url: https://api.deepseek.com/v1  # 可选

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

# 默认输出语言
language: zh

# 代码审查设置（可选）
review:
  max_lines_per_read: 1000      # 每次文件操作最多读取的行数
  grep_max_file_size: 10        # grep 最大文件大小（MB）
  grep_timeout: 10              # grep 操作超时时间（秒）
  grep_max_results: 100         # grep 最大结果数量

# 问题排查设置（可选）
debug:
  issues_dir: ./issues          # 保存调试报告的目录
  max_lines_per_read: 1000      # 每次文件操作最多读取的行数
  grep_max_file_size: 10        # grep 最大文件大小（MB）
  grep_timeout: 10              # grep 操作超时时间（秒）
  grep_max_results: 100         # grep 最大结果数量
```

### 配置优先级

1. 命令行参数（最高优先级）
2. 配置文件
3. 环境变量
4. 默认值

## 使用方法

### 生成 Commit 信息

```bash
# 基本用法 - 分析暂存区变更并生成 commit 信息
gitbuddy commit

# 指定输出语言
gitbuddy commit -l zh

# 提供额外上下文
gitbuddy commit -c "修复了 issue #123 中报告的登录问题"

# 使用指定模型
gitbuddy commit -m openai

# 自动确认，无需提示
gitbuddy commit -y
```

### 生成 PR 描述

```bash
# 与 main 分支比较
gitbuddy pr --base main

# 指定语言和上下文
gitbuddy pr --base develop -l zh -c "API 接口性能优化"

# 使用指定模型
gitbuddy pr --base main -m gemini
```

### 生成开发报告

```bash
# 生成指定日期范围的报告
gitbuddy report --since 2024-12-01 --until 2024-12-31

# 按作者筛选
gitbuddy report --since 2024-12-01 --author "john@example.com"

# 指定语言
gitbuddy report --since 2024-12-01 -l zh
```

### 代码审查

```bash
# 审查所有暂存区变更
gitbuddy review

# 提供额外上下文
gitbuddy review -c "这是一个用户认证模块"

# 只审查指定文件
gitbuddy review --files "auth.go,crypto.go"

# 只显示错误级别问题
gitbuddy review --severity error

# 重点关注安全和性能问题
gitbuddy review --focus security,performance

# 使用中文输出
gitbuddy review -l zh
```

代码审查会识别以下类型的问题：
- 🔴 **错误**: bugs、崩溃、关键问题
- 🟡 **警告**: 潜在 bugs、性能问题
- 🔵 **建议**: 代码风格、重构建议

### 问题排查

```bash
# 使用 AI 辅助排查特定问题
gitbuddy debug "登录时返回 500 错误"

# 提供额外上下文
gitbuddy debug "后台任务内存泄漏" -c "运行 24 小时后出现"

# 重点关注特定文件
gitbuddy debug "测试 TestUserAuth 失败" --files "auth_test.go,auth.go"

# 启用交互式模式（Agent 可以询问你的意见）
gitbuddy debug "API 返回错误数据" --interactive

# 使用中文进行交互式排查
gitbuddy debug "性能问题" -l zh --interactive

# 指定自定义报告保存目录
gitbuddy debug "数据库连接超时" --issues-dir ./debug-reports
```

问题排查功能：
- 🔍 **系统化分析**: 使用文件系统、搜索和 Git 工具系统化分析问题
- 🤖 **自主探索**: 自主探索代码库以理解问题
- 💬 **交互式询问**: 在需要时询问你的意见（使用 `--interactive` 标志）
- 📋 **生成详细报告**: 生成包含根本原因分析和修复建议的详细报告
- 💾 **保存报告**: 将报告保存到 `./issues` 目录以供将来参考

### 其他命令

```bash
# 显示版本信息
gitbuddy version

# 列出已配置的模型
gitbuddy models list

# 初始化配置文件
gitbuddy init
```

### 全局参数

| 参数 | 说明 |
|------|------|
| `--config` | 配置文件路径（默认：`~/.gitbuddy.yaml`） |
| `--debug` | 启用调试模式，输出详细日志 |
| `-m, --model` | 指定使用的 LLM 模型 |

## 支持的 LLM

| 提供商 | 模型 | 说明 |
|--------|------|------|
| **DeepSeek** | deepseek-chat, deepseek-reasoner | 推荐，性价比最高 |
| **OpenAI** | gpt-4o, gpt-4o-mini, gpt-3.5-turbo | 需要 OpenAI API 密钥 |
| **Ollama** | 任意本地模型 | 本地运行，无需 API 密钥 |
| **Grok** | grok-beta | 需要 xAI API 密钥 |
| **Gemini** | gemini-2.0-flash, gemini-1.5-pro | 需要 Google AI API 密钥 |

## 工作原理

GitBuddy 采用 **Agent 方式**，LLM 自主决定执行哪些 Git 命令：

1. **生成 Commit 信息时**:
   - LLM 调用 `git status` 获取概览
   - LLM 调用 `git diff --cached` 分析变更
   - 可选调用 `git log` 获取上下文
   - 通过 `submit_commit` 工具生成结构化的 commit 信息

2. **生成 PR 描述时**:
   - LLM 调用 `git log` 查看分支间的提交
   - LLM 调用 `git diff` 分析代码变更
   - 通过 `submit_pr` 工具生成 PR 描述

3. **生成报告时**:
   - LLM 调用 `git log` 并应用日期过滤
   - 分析并分类提交
   - 通过 `submit_report` 工具生成报告

4. **代码审查时**:
   - LLM 调用 `git diff --cached` 分析暂存区变更
   - LLM 使用 `grep_file` 快速定位文件中的特定函数或模式
   - LLM 使用 `grep_directory` 在多个文件中查找代码模式
   - LLM 在需要时调用 `read_file` 读取完整的源代码上下文
   - 识别 bugs、安全隐患、性能问题
   - 通过 `submit_review` 工具生成审查报告

这种 Agent 方式让 LLM 能够准确获取所需的上下文，从而生成更准确、更相关的输出。

## 调试模式

启用调试模式查看详细信息：

```bash
gitbuddy commit --debug
```

调试模式会显示：
- 配置详情
- 使用的 LLM 提供商和模型
- 工具调用及其结果
- Token 使用统计
- 执行时间

## 代理支持

GitBuddy 支持标准的代理环境变量：

```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

## 贡献

欢迎贡献代码！请随时提交 Pull Request。

## 许可证

MIT License - 详见 [LICENSE](LICENSE)。

## 致谢

- [CloudWeGo Eino](https://github.com/cloudwego/eino) - AI Agent 框架
- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [Viper](https://github.com/spf13/viper) - 配置管理

