package agent

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMessageModifierChain(t *testing.T) {
	modifier1 := func(messages []*schema.Message) []*schema.Message {
		result := make([]*schema.Message, len(messages))
		for i, msg := range messages {
			newMsg := *msg
			newMsg.Content = msg.Content + " [mod1]"
			result[i] = &newMsg
		}
		return result
	}

	modifier2 := func(messages []*schema.Message) []*schema.Message {
		result := make([]*schema.Message, len(messages))
		for i, msg := range messages {
			newMsg := *msg
			newMsg.Content = msg.Content + " [mod2]"
			result[i] = &newMsg
		}
		return result
	}

	chain := MessageModifierChain(modifier1, modifier2)

	messages := []*schema.Message{
		{Role: schema.User, Content: "test"},
	}

	result := chain(messages)

	if !strings.Contains(result[0].Content, "[mod1]") {
		t.Error("Expected modifier1 to be applied")
	}
	if !strings.Contains(result[0].Content, "[mod2]") {
		t.Error("Expected modifier2 to be applied")
	}
	if !strings.HasSuffix(result[0].Content, "[mod2]") {
		t.Error("Expected modifier2 to be applied after modifier1")
	}
}

func TestSummarizeToolResults(t *testing.T) {
	modifier := SummarizeToolResults(50)

	longContent := strings.Repeat("a", 100)
	messages := []*schema.Message{
		{Role: schema.System, Content: "system"},
		{Role: schema.Tool, Content: longContent, ToolCallID: "1"},
		{Role: schema.Tool, Content: "short", ToolCallID: "2"},
	}

	result := modifier(messages)

	// System message should be unchanged
	if result[0].Content != "system" {
		t.Errorf("Expected system message unchanged, got: %s", result[0].Content)
	}

	// Long tool result should be truncated
	// The truncation adds a message, so total length might be slightly longer
	// but the original content should be cut to maxLength
	if !strings.Contains(result[1].Content, "truncated") {
		t.Error("Expected truncation message")
	}
	// Check that the first part is exactly 50 chars (maxLength)
	firstPart := strings.Split(result[1].Content, "\n\n[...")[0]
	if len(firstPart) != 50 {
		t.Errorf("Expected first part to be exactly 50 chars, got %d", len(firstPart))
	}

	// Short tool result should be unchanged
	if result[2].Content != "short" {
		t.Errorf("Expected short tool result unchanged, got: %s", result[2].Content)
	}
}

func TestDeduplicateMessages(t *testing.T) {
	modifier := DeduplicateMessages()

	messages := []*schema.Message{
		{Role: schema.System, Content: "system"},
		{Role: schema.User, Content: "hello"},
		{Role: schema.User, Content: "hello"}, // duplicate
		{Role: schema.Assistant, Content: "hi"},
		{Role: schema.User, Content: "bye"},
	}

	result := modifier(messages)

	if len(result) != 4 {
		t.Errorf("Expected 4 messages after deduplication, got %d", len(result))
	}

	// Check that the duplicate was removed
	userCount := 0
	for _, msg := range result {
		if msg.Role == schema.User && msg.Content == "hello" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Errorf("Expected 1 'hello' message, got %d", userCount)
	}
}

func TestFilterToolResults(t *testing.T) {
	modifier := FilterToolResults("read_file", "grep_file")

	messages := []*schema.Message{
		{Role: schema.System, Content: "system"},
		{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "1", Function: schema.FunctionCall{Name: "read_file"}},
				{ID: "2", Function: schema.FunctionCall{Name: "list_files"}},
				{ID: "3", Function: schema.FunctionCall{Name: "grep_file"}},
			},
		},
		{Role: schema.Tool, Content: "file content", ToolCallID: "1"},
		{Role: schema.Tool, Content: "file list", ToolCallID: "2"},
		{Role: schema.Tool, Content: "grep result", ToolCallID: "3"},
	}

	result := modifier(messages)

	// System message should be kept
	if result[0].Role != schema.System {
		t.Error("Expected system message to be kept")
	}

	// Assistant message should have filtered tool calls
	if len(result[1].ToolCalls) != 2 {
		t.Errorf("Expected 2 tool calls, got %d", len(result[1].ToolCalls))
	}

	// Only read_file and grep_file tool results should be kept
	toolResultCount := 0
	for _, msg := range result {
		if msg.Role == schema.Tool {
			toolResultCount++
			if msg.ToolCallID != "1" && msg.ToolCallID != "3" {
				t.Errorf("Unexpected tool result with ID: %s", msg.ToolCallID)
			}
		}
	}
	if toolResultCount != 2 {
		t.Errorf("Expected 2 tool results, got %d", toolResultCount)
	}
}

func TestAddContextToSystemMessage(t *testing.T) {
	modifier := AddContextToSystemMessage(func(messages []*schema.Message) string {
		return "Additional context"
	})

	messages := []*schema.Message{
		{Role: schema.System, Content: "Original system message"},
		{Role: schema.User, Content: "user message"},
	}

	result := modifier(messages)

	if !strings.Contains(result[0].Content, "Original system message") {
		t.Error("Expected original system message to be preserved")
	}
	if !strings.Contains(result[0].Content, "Additional context") {
		t.Error("Expected additional context to be added")
	}
	if result[1].Content != "user message" {
		t.Error("Expected user message to be unchanged")
	}
}

func TestHighlightRecentChanges(t *testing.T) {
	modifier := HighlightRecentChanges(2)

	messages := []*schema.Message{
		{Role: schema.System, Content: "system"},
		{Role: schema.User, Content: "old message 1"},
		{Role: schema.Assistant, Content: "old response 1"},
		{Role: schema.User, Content: "recent message 1"},
		{Role: schema.Assistant, Content: "recent response 1"},
	}

	result := modifier(messages)

	// Old messages should be unchanged
	if strings.Contains(result[1].Content, "RECENT") {
		t.Error("Expected old message to not be marked as recent")
	}

	// Recent messages should be marked
	if !strings.Contains(result[3].Content, "RECENT CONTEXT STARTS HERE") {
		t.Error("Expected first recent message to be marked")
	}
}

func TestCreateProgressContextModifier(t *testing.T) {
	plan := NewExecutionPlan()
	plan.AddTask("task1", "Test task 1")
	plan.AddTask("task2", "Test task 2")
	plan.UpdateTask("task1", "completed")
	plan.UpdateTask("task2", "in_progress")

	modifier := CreateProgressContextModifier(plan, 5, 30)

	messages := []*schema.Message{
		{Role: schema.System, Content: "Original system"},
		{Role: schema.User, Content: "user message"},
	}

	result := modifier(messages)

	systemContent := result[0].Content

	if !strings.Contains(systemContent, "Current Progress") {
		t.Error("Expected progress context to be added")
	}
	if !strings.Contains(systemContent, "Iteration: 5 / 30") {
		t.Error("Expected iteration info")
	}
	if !strings.Contains(systemContent, "1 completed") {
		t.Error("Expected task completion info")
	}
	if !strings.Contains(systemContent, "1 in progress") {
		t.Error("Expected in-progress task info")
	}
	if !strings.Contains(systemContent, "Test task 2") {
		t.Error("Expected current task description")
	}
}
