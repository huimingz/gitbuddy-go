package agent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// MessageModifierChain chains multiple message modifiers together
// Modifiers are applied in order from left to right
func MessageModifierChain(modifiers ...MessageModifier) MessageModifier {
	return func(messages []*schema.Message) []*schema.Message {
		result := messages
		for _, modifier := range modifiers {
			if modifier != nil {
				result = modifier(result)
			}
		}
		return result
	}
}

// LimitMessageTokens creates a modifier that limits the total token count
// by removing older messages (keeping system and first user message)
func LimitMessageTokens(maxTokens int) MessageModifier {
	return func(messages []*schema.Message) []*schema.Message {
		if len(messages) <= 2 {
			return messages
		}

		totalTokens := 0
		for _, msg := range messages {
			totalTokens += estimateTokenCount(msg.Content)
		}

		if totalTokens <= maxTokens {
			return messages
		}

		// Keep system message and first user message
		result := []*schema.Message{messages[0], messages[1]}

		// Add messages from the end until we reach the token limit
		budget := maxTokens - estimateTokenCount(messages[0].Content) - estimateTokenCount(messages[1].Content)

		for i := len(messages) - 1; i >= 2; i-- {
			msgTokens := estimateTokenCount(messages[i].Content)
			if budget >= msgTokens {
				result = append([]*schema.Message{messages[i]}, result[1:]...)
				result = append(result[:1], append([]*schema.Message{messages[i]}, result[1:]...)...)
				budget -= msgTokens
			} else {
				break
			}
		}

		return result
	}
}

// FilterToolResults creates a modifier that filters tool results by name
// Only tool results from specified tools will be kept
func FilterToolResults(keepTools ...string) MessageModifier {
	keepSet := make(map[string]bool)
	for _, tool := range keepTools {
		keepSet[tool] = true
	}

	return func(messages []*schema.Message) []*schema.Message {
		if len(keepSet) == 0 {
			return messages
		}

		result := make([]*schema.Message, 0, len(messages))

		// Map to track which tool call IDs to keep
		keepToolCallIDs := make(map[string]bool)

		// First pass: identify tool calls to keep
		for _, msg := range messages {
			if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					if keepSet[tc.Function.Name] {
						keepToolCallIDs[tc.ID] = true
					}
				}
			}
		}

		// Second pass: filter messages
		for _, msg := range messages {
			// Always keep system and user messages
			if msg.Role == schema.System || msg.Role == schema.User {
				result = append(result, msg)
				continue
			}

			// For assistant messages, filter tool calls
			if msg.Role == schema.Assistant {
				if len(msg.ToolCalls) == 0 {
					result = append(result, msg)
				} else {
					filteredToolCalls := make([]schema.ToolCall, 0)
					for _, tc := range msg.ToolCalls {
						if keepToolCallIDs[tc.ID] {
							filteredToolCalls = append(filteredToolCalls, tc)
						}
					}
					if len(filteredToolCalls) > 0 {
						newMsg := *msg
						newMsg.ToolCalls = filteredToolCalls
						result = append(result, &newMsg)
					}
				}
				continue
			}

			// For tool messages, only keep if the tool call ID is in keepToolCallIDs
			if msg.Role == schema.Tool {
				if keepToolCallIDs[msg.ToolCallID] {
					result = append(result, msg)
				}
				continue
			}

			// Keep other messages as-is
			result = append(result, msg)
		}

		return result
	}
}

// SummarizeToolResults creates a modifier that summarizes long tool results
// Tool results longer than maxLength will be truncated with a summary
func SummarizeToolResults(maxLength int) MessageModifier {
	return func(messages []*schema.Message) []*schema.Message {
		result := make([]*schema.Message, len(messages))

		for i, msg := range messages {
			if msg.Role == schema.Tool && len(msg.Content) > maxLength {
				// Create a summarized version
				summary := msg.Content[:maxLength]
				remaining := len(msg.Content) - maxLength

				newMsg := *msg
				newMsg.Content = fmt.Sprintf("%s\n\n[... %d more characters truncated for brevity ...]",
					summary, remaining)
				result[i] = &newMsg
			} else {
				result[i] = msg
			}
		}

		return result
	}
}

// AddContextToSystemMessage creates a modifier that appends context to the system message
// This is useful for adding dynamic context based on the conversation state
func AddContextToSystemMessage(contextFn func(messages []*schema.Message) string) MessageModifier {
	return func(messages []*schema.Message) []*schema.Message {
		if len(messages) == 0 {
			return messages
		}

		context := contextFn(messages)
		if context == "" {
			return messages
		}

		result := make([]*schema.Message, len(messages))
		copy(result, messages)

		// Modify the system message
		if result[0].Role == schema.System {
			newSystemMsg := *result[0]
			newSystemMsg.Content = result[0].Content + "\n\n" + context
			result[0] = &newSystemMsg
		}

		return result
	}
}

// HighlightRecentChanges creates a modifier that adds markers to recent messages
// This helps the LLM focus on the most recent context
func HighlightRecentChanges(recentCount int) MessageModifier {
	return func(messages []*schema.Message) []*schema.Message {
		if len(messages) <= recentCount {
			return messages
		}

		result := make([]*schema.Message, len(messages))
		copy(result, messages)

		// Mark recent messages
		startIdx := len(messages) - recentCount
		for i := startIdx; i < len(result); i++ {
			if result[i].Role == schema.User || result[i].Role == schema.Assistant {
				newMsg := *result[i]
				if i == startIdx {
					newMsg.Content = "[RECENT CONTEXT STARTS HERE]\n\n" + newMsg.Content
				}
				result[i] = &newMsg
			}
		}

		return result
	}
}

// DeduplicateMessages creates a modifier that removes duplicate consecutive messages
// This is useful when the same tool is called multiple times with the same result
func DeduplicateMessages() MessageModifier {
	return func(messages []*schema.Message) []*schema.Message {
		if len(messages) <= 1 {
			return messages
		}

		result := make([]*schema.Message, 0, len(messages))
		result = append(result, messages[0])

		for i := 1; i < len(messages); i++ {
			prev := result[len(result)-1]
			curr := messages[i]

			// Check if current message is duplicate of previous
			if prev.Role == curr.Role &&
				prev.Content == curr.Content &&
				len(prev.ToolCalls) == len(curr.ToolCalls) {
				// Skip duplicate
				continue
			}

			result = append(result, curr)
		}

		return result
	}
}

// CreateProgressContextModifier creates a modifier that adds execution progress to system message
// This helps the LLM understand how far along it is in the debugging process
func CreateProgressContextModifier(plan *ExecutionPlan, currentIteration, maxIterations int) MessageModifier {
	return AddContextToSystemMessage(func(messages []*schema.Message) string {
		var context strings.Builder

		context.WriteString("## Current Progress\n\n")
		context.WriteString(fmt.Sprintf("- Iteration: %d / %d\n", currentIteration, maxIterations))
		context.WriteString(fmt.Sprintf("- Messages in history: %d\n", len(messages)))

		if plan != nil && len(plan.Tasks) > 0 {
			completed := 0
			inProgress := 0
			pending := 0

			for _, task := range plan.Tasks {
				switch task.Status {
				case "completed":
					completed++
				case "in_progress":
					inProgress++
				case "pending":
					pending++
				}
			}

			context.WriteString(fmt.Sprintf("- Tasks: %d completed, %d in progress, %d pending\n",
				completed, inProgress, pending))

			if inProgress > 0 {
				context.WriteString("\nCurrent task(s):\n")
				for _, task := range plan.Tasks {
					if task.Status == "in_progress" {
						context.WriteString(fmt.Sprintf("  - %s\n", task.Description))
					}
				}
			}
		}

		return context.String()
	})
}
