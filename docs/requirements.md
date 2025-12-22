# GitBuddy-Go 需求分析文档

## 1. 项目概述

GitBuddy-Go 是一个面向程序员的智能命令行工具，利用 AI Agent 能力辅助日常 Git 工作流程，包括生成规范的 commit 信息、PR 描述和开发报告。

## 2. 开发优先级

| 优先级 | 功能 | 说明 |
|-------|------|------|
| P0 | Commit 生成 | 首先实现 |
| P1 | PR 描述生成 | 第二阶段 |
| P2 | 开发报告生成 | 第三阶段 |

## 3. 核心功能分析

### 3.1 Commit 生成功能（P0 - 首先实现）

**用户场景**：
- 用户完成代码修改并将文件添加到暂存区
- 执行 `gitbuddy commit` 命令
- 工具自动分析暂存区的 diff 内容
- AI Agent 生成符合 Conventional Commits 规范的 commit message
- 用户确认后执行实际的 git commit

**功能要点**：
1. 自动获取 `git diff --cached` 内容
2. Agent 可能需要调用其他 git 命令辅助分析（如 `git status`、`git log` 等）
3. 生成的 commit message 需符合 Conventional Commits 规范：
   - 格式：`<type>[optional scope]: <description>`
   - 可选的 body 和 footer
   - type 包括：feat, fix, docs, style, refactor, test, chore 等
4. 交互式确认机制（用户可修改或直接确认）
5. 确认后执行 `git commit -m "message"`（message 中可包含换行符以支持 body）
6. 支持指定输出语言
7. 支持提供额外上下文信息（--context）

**输入**：
- `--context`：额外的上下文信息（可选）
- `--lang`：输出语言（可选，默认英文）
- `--model`：指定使用的模型（可选）

**输出**：规范的 commit message，用户确认后执行 commit

### 3.2 PR 描述生成功能（P1 - 第二阶段）

**用户场景**：
- 用户在功能分支开发完成
- 执行 `gitbuddy pr --base main`（指定目标分支）
- 工具分析当前分支与目标分支的差异
- 生成 PR 标题和详细描述
- 用户复制输出内容到 Web 端使用

**功能要点**：
1. 获取当前分支与目标分支的 diff（`git diff base..HEAD`）
2. 获取相关的 commit log（`git log base..HEAD`）
3. 生成内容包括：
   - PR 标题（简洁描述主要变更）
   - 变更说明（What & Why）
   - 主要改动点列表
   - 可能的影响范围
4. 格式化输出，便于复制
5. 支持指定输出语言
6. 支持提供额外上下文信息

**输入**：
- `--base`：目标分支名称（必填，如 main, develop）
- `--context`：额外的上下文信息（可选）
- `--lang`：输出语言（可选，默认英文）
- `--model`：指定使用的模型（可选）

**输出**：PR 标题 + PR 描述内容（终端输出）

### 3.3 开发报告生成功能（P2 - 第三阶段）

**用户场景**：
- 用户需要编写周报/月报
- 执行 `gitbuddy report --since "2024-01-15" --until "2024-01-21"`
- 工具分析指定时间段内的 commit 记录
- 生成结构化的开发报告

**功能要点**：
1. 根据作者和日期范围过滤 commit（`git log --author="name" --since="date" --until="date"`）
2. 分析 commit 内容，归类整理
3. 生成报告内容：
   - 时间范围
   - 主要完成的功能/任务
   - 修复的问题
   - 代码统计（可选）
4. 支持周报和月报格式
5. 支持指定输出语言
6. 支持提供额外上下文信息

**输入**：
- `--author`：作者名称（可选，默认当前 git 用户）
- `--since`：开始日期
- `--until`：结束日期（可选，默认今天）
- `--context`：额外的上下文信息（可选）
- `--lang`：输出语言（可选，默认英文）
- `--model`：指定使用的模型（可选）

**输出**：结构化的开发报告（终端输出）

## 4. 语言配置功能

### 4.1 支持的语言

| 语言代码 | 语言名称 |
|---------|---------|
| `en` | English（默认） |
| `zh` | 中文（简体） |
| `zh-tw` | 中文（繁體） |
| `ja` | 日本語 |
| `ko` | 한국어 |

### 4.2 语言配置优先级

1. **命令行参数**：`--lang` 或 `-l`（最高优先级）
2. **配置文件**：`.gitbuddy.yaml` 中的 `language` 字段
3. **环境变量**：`GITBUDDY_LANG`
4. **默认值**：`en`（英文）

## 5. 多模型支持

### 5.1 支持的模型提供商

| Provider | 说明 | API 兼容性 |
|----------|------|-----------|
| `openai` | OpenAI GPT 系列 | OpenAI API |
| `deepseek` | Deepseek 模型 | OpenAI 兼容 |
| `ollama` | 本地 Ollama | OpenAI 兼容 |
| `gemini` | Google Gemini | Google AI API |
| `grok` | xAI Grok | OpenAI 兼容 |

### 5.2 模型选择优先级

1. **命令行参数**：`--model` 或 `-m`（最高优先级）
2. **环境变量**：`GITBUDDY_MODEL`
3. **配置文件**：`default_model` 字段
4. **内置默认**：如果都没配置，报错提示用户配置

## 6. Context（上下文）功能

### 6.1 使用场景

- 代码 diff 本身可能无法完全表达开发者的意图
- 需要提供额外的背景信息帮助 AI 生成更准确的结果
- 强调某些改动的重点或目的

### 6.2 使用示例

```bash
# 场景1：简单的代码修改，但有特殊上下文
gitbuddy commit -c "这是为了修复线上紧急bug #1234"

# 场景2：重构代码，需要说明目的
gitbuddy commit -c "重构认证模块以支持多租户"

# 场景3：多个改动混在一起，需要强调重点
gitbuddy commit -c "主要是添加缓存功能，其他是顺手修的小问题"

# 场景4：PR 描述需要额外上下文
gitbuddy pr -b main -c "这个PR是Q1性能优化计划的一部分"

# 场景5：报告需要强调某些内容
gitbuddy report -s 2024-01-15 -c "重点突出安全相关的改进"
```

## 7. 命令设计

```bash
# 初始化配置文件
gitbuddy init [-f|--force]

# 查看版本信息
gitbuddy version

# 查看已配置的模型列表
gitbuddy models list

# Commit 生成
gitbuddy commit [-c <context>] [-m <model>] [-l <language>] [--debug]

# PR 描述生成
gitbuddy pr -b <branch> [-c <context>] [-m <model>] [-l <language>] [--debug]

# 开发报告
gitbuddy report -s <date> [-u <date>] [-a <name>] [-c <context>] [-m <model>] [-l <language>] [--debug]
```

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

## 8. 配置文件设计

配置文件位置：`~/.gitbuddy.yaml` 或项目根目录 `.gitbuddy.yaml`

```yaml
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

## 9. 非功能需求

1. **用户体验**：
   - 清晰的命令行提示
   - 流式输出（实时显示 AI 思考和生成过程）
   - 工具调用信息展示
   - Token 消耗和用时统计
   - 友好的错误提示

2. **安全性**：
   - commit 操作需要用户确认（Y/N）
   - 不自动推送代码
   - API Key 通过环境变量配置（推荐）

3. **可配置性**：
   - LLM 配置（多模型支持）
   - 输出语言偏好
   - 可通过配置文件持久化设置
   - 支持代理配置（通过环境变量）

4. **调试能力**：
   - `--debug` 模式输出详细日志
   - 包含配置、Prompt、API 请求等信息

## 10. 代理支持

GitBuddy 支持通过环境变量配置代理，用于访问 LLM API：

```bash
# HTTP 代理
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890

# 不使用代理的地址（可选）
export NO_PROXY=localhost,127.0.0.1
```

## 11. 流式输出

LLM 的输出采用流式方式，实时在命令行界面显示：

1. **思考/规划信息**：显示 AI 的思考过程
2. **工具调用**：显示调用的工具名称和参数
3. **工具结果**：显示工具执行的结果摘要
4. **生成内容**：实时流式输出生成的内容
5. **统计信息**：最终显示 token 消耗和用时

