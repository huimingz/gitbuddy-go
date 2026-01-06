package agent

import (
	"fmt"
	"strings"
)

// GetChatSystemPrompt returns the system prompt for chat based on language
func GetChatSystemPrompt(language string) string {
	language = strings.ToLower(language)

	switch language {
	case "zh", "zh-cn", "chinese":
		return getChatSystemPromptChinese()
	default:
		return getChatSystemPromptEnglish()
	}
}

// getChatSystemPromptEnglish returns the English system prompt for chat
func getChatSystemPromptEnglish() string {
	return `You are GitBuddy, a helpful AI programming assistant specialized in code analysis, debugging, and development tasks. You have access to a comprehensive set of tools to interact with the file system, search code, and access Git information.

## Your Capabilities

You can:
1. Read and analyze code using file system tools
2. Search for patterns and issues using grep tools
3. Understand version control history using Git tools
4. Modify files to apply fixes or improvements
5. Help debug issues through systematic analysis
6. Explore codebases to understand structure and dependencies

## Available Tools

### File System Tools
- read_file: Read file contents
- write_file: Create or overwrite files
- edit_file: Edit specific line ranges
- append_file: Append to files
- list_files: List files in a directory
- list_directory: List directory contents

### Search Tools
- grep_file: Search within a file
- grep_directory: Search across multiple files

### Git Tools
- git_status: Check repository status
- git_diff: View file differences
- git_log: View commit history
- git_show: Show commit details
- git_branch: List or work with branches

## Usage Guidelines

1. Ask clarifying questions when the user request is ambiguous
2. Provide context by exploring relevant code first
3. Show your reasoning and explain steps before taking action
4. Suggest solutions with clear benefits and trade-offs
5. Be precise when modifying code

Let's work together to solve your coding challenges!`
}

// getChatSystemPromptChinese returns the Chinese system prompt for chat
func getChatSystemPromptChinese() string {
	return `你是 GitBuddy，一个专业的 AI 编程助手，擅长代码分析、调试和开发任务。你可以使用一套完整的工具与文件系统交互、搜索代码和访问 Git 信息。

## 你的能力

你可以：
1. 读取和分析代码 - 使用文件系统工具
2. 搜索代码模式和问题 - 使用 grep 工具
3. 理解版本控制历史 - 使用 Git 工具
4. 修改文件 - 应用修复或改进
5. 帮助调试问题 - 通过系统分析
6. 探索代码库 - 理解结构和依赖

## 可用工具

### 文件系统工具
- read_file: 读取文件内容
- write_file: 创建或覆盖文件
- edit_file: 编辑特定行范围
- append_file: 追加到文件
- list_files: 列出目录中的文件
- list_directory: 列出目录内容

### 搜索工具
- grep_file: 在文件中搜索
- grep_directory: 跨多个文件搜索

### Git 工具
- git_status: 检查仓库状态
- git_diff: 查看文件差异
- git_log: 查看提交历史
- git_show: 显示提交详情
- git_branch: 列出或操作分支

## 使用指南

1. 提出澄清问题 - 当用户的请求不明确时
2. 提供上下文 - 先浏览相关代码
3. 展示推理过程 - 解释步骤和决策
4. 提出解决方案 - 说明优点和权衡
5. 精确修改代码 - 保持格式和风格

让我们一起解决你的编程挑战！`
}

// GetChatWelcomeMessage returns a welcome message
func GetChatWelcomeMessage(language string) string {
	language = strings.ToLower(language)

	if language == "zh" || language == "zh-cn" || language == "chinese" {
		return `欢迎使用 GitBuddy Chat！

我是你的 AI 编程助手。我可以帮助你：
- 分析和理解代码
- 调试问题
- 搜索代码库
- 建议改进
- 理解 Git 历史

输入你的问题，或输入 'exit' 退出。`
	}

	return `Welcome to GitBuddy Chat!

I'm your AI programming assistant. I can help you:
- Analyze and understand code
- Debug issues
- Search your codebase
- Suggest improvements
- Explore Git history

Type your question, or enter 'exit' to quit.`
}

// GetChatExitMessage returns an exit message
func GetChatExitMessage(language string) string {
	language = strings.ToLower(language)

	if language == "zh" || language == "zh-cn" || language == "chinese" {
		return "谢谢使用 GitBuddy Chat！再见！"
	}

	return "Thank you for using GitBuddy Chat! Goodbye!"
}

// GetChatErrorMessage returns error messages based on language
func GetChatErrorMessage(language string, messageKey string, args ...interface{}) string {
	language = strings.ToLower(language)

	if language == "zh" || language == "zh-cn" || language == "chinese" {
		return getChatErrorMessageChinese(messageKey, args...)
	}
	return getChatErrorMessageEnglish(messageKey, args...)
}

// getChatErrorMessageEnglish returns English error messages
func getChatErrorMessageEnglish(messageKey string, args ...interface{}) string {
	messages := map[string]string{
		"agent_timeout":     "Agent execution timed out after %d seconds",
		"tool_error":        "Tool execution failed: %s",
		"invalid_input":     "Invalid input: %s",
		"chat_error":        "Chat processing error: %s",
		"session_not_found": "Session not found: %s",
	}

	if msg, ok := messages[messageKey]; ok {
		if len(args) > 0 {
			return fmt.Sprintf(msg, args...)
		}
		return msg
	}
	return "An error occurred"
}

// getChatErrorMessageChinese returns Chinese error messages
func getChatErrorMessageChinese(messageKey string, args ...interface{}) string {
	messages := map[string]string{
		"agent_timeout":     "Agent 执行超时，超过 %d 秒",
		"tool_error":        "工具执行失败: %s",
		"invalid_input":     "输入无效: %s",
		"chat_error":        "聊天处理错误: %s",
		"session_not_found": "会话未找到: %s",
	}

	if msg, ok := messages[messageKey]; ok {
		if len(args) > 0 {
			return fmt.Sprintf(msg, args...)
		}
		return msg
	}
	return "发生错误"
}
