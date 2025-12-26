package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

// mockExecutionPlan is a mock implementation of ExecutionPlanManager for testing
type mockExecutionPlan struct {
	tasks       []mockTask
	lastUpdated time.Time
}

type mockTask struct {
	ID          string
	Description string
	Status      string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

func newMockExecutionPlan() *mockExecutionPlan {
	return &mockExecutionPlan{
		tasks:       []mockTask{},
		lastUpdated: time.Now(),
	}
}

func (m *mockExecutionPlan) AddTask(id, description string) {
	m.tasks = append(m.tasks, mockTask{
		ID:          id,
		Description: description,
		Status:      "pending",
		CreatedAt:   time.Now(),
	})
	m.lastUpdated = time.Now()
}

func (m *mockExecutionPlan) UpdateTask(id, status string) bool {
	for i := range m.tasks {
		if m.tasks[i].ID == id {
			oldStatus := m.tasks[i].Status
			m.tasks[i].Status = status
			if status == "completed" || status == "skipped" {
				now := time.Now()
				m.tasks[i].CompletedAt = &now
			}
			m.lastUpdated = time.Now()
			return oldStatus != status
		}
	}
	return false
}

func (m *mockExecutionPlan) RemoveTask(id string) bool {
	for i, task := range m.tasks {
		if task.ID == id {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			m.lastUpdated = time.Now()
			return true
		}
	}
	return false
}

func (m *mockExecutionPlan) GetSummary() string {
	if len(m.tasks) == 0 {
		return "No execution plan yet."
	}

	var summary strings.Builder
	summary.WriteString("Current Plan:\n")
	for i, task := range m.tasks {
		summary.WriteString(strings.Repeat(" ", 2))
		summary.WriteString(strings.Repeat(" ", len(strings.TrimSpace(strings.Split(summary.String(), "\n")[0]))))
		summary.WriteString(task.ID)
		summary.WriteString(": ")
		summary.WriteString(task.Description)
		summary.WriteString(" [")
		summary.WriteString(task.Status)
		summary.WriteString("]")
		if i < len(m.tasks)-1 {
			summary.WriteString("\n")
		}
	}
	return summary.String()
}

func (m *mockExecutionPlan) GetChanges(old interface{}) []string {
	oldPlan, ok := old.(*mockExecutionPlan)
	if !ok || oldPlan == nil {
		return []string{"Initial plan created"}
	}

	var changes []string

	// Check for new tasks
	oldTaskIDs := make(map[string]bool)
	for _, task := range oldPlan.tasks {
		oldTaskIDs[task.ID] = true
	}

	for _, task := range m.tasks {
		if !oldTaskIDs[task.ID] {
			changes = append(changes, "Added: "+task.Description)
		}
	}

	// Check for removed tasks
	newTaskMap := make(map[string]mockTask)
	for _, task := range m.tasks {
		newTaskMap[task.ID] = task
	}

	for _, oldTask := range oldPlan.tasks {
		if _, exists := newTaskMap[oldTask.ID]; !exists {
			changes = append(changes, "Removed: "+oldTask.Description)
		}
	}

	// Check for status changes
	for _, newTask := range m.tasks {
		for _, oldTask := range oldPlan.tasks {
			if newTask.ID == oldTask.ID && newTask.Status != oldTask.Status {
				changes = append(changes, "Status changed: "+newTask.Description)
			}
		}
	}

	return changes
}

func (m *mockExecutionPlan) Clone() interface{} {
	if m == nil {
		return (*mockExecutionPlan)(nil)
	}

	clone := &mockExecutionPlan{
		tasks:       make([]mockTask, len(m.tasks)),
		lastUpdated: m.lastUpdated,
	}

	for i, task := range m.tasks {
		clone.tasks[i] = mockTask{
			ID:          task.ID,
			Description: task.Description,
			Status:      task.Status,
			CreatedAt:   task.CreatedAt,
		}
		if task.CompletedAt != nil {
			completedAt := *task.CompletedAt
			clone.tasks[i].CompletedAt = &completedAt
		}
	}

	return clone
}

func TestUpdateExecutionPlanTool_AddTask(t *testing.T) {
	plan := newMockExecutionPlan()
	tool := NewUpdateExecutionPlanTool(plan)

	params := &UpdateExecutionPlanParams{
		Action:      "add",
		TaskID:      "task1",
		Description: "Investigate authentication flow",
	}

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Task added successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	if !strings.Contains(result, "Added: Investigate authentication flow") {
		t.Errorf("Expected change message, got: %s", result)
	}

	if len(plan.tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(plan.tasks))
	}

	if plan.tasks[0].ID != "task1" {
		t.Errorf("Expected task ID 'task1', got '%s'", plan.tasks[0].ID)
	}
}

func TestUpdateExecutionPlanTool_UpdateTask(t *testing.T) {
	plan := newMockExecutionPlan()
	plan.AddTask("task1", "Investigate authentication flow")
	tool := NewUpdateExecutionPlanTool(plan)

	params := &UpdateExecutionPlanParams{
		Action: "update",
		TaskID: "task1",
		Status: "in_progress",
	}

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Task status updated successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	if plan.tasks[0].Status != "in_progress" {
		t.Errorf("Expected status 'in_progress', got '%s'", plan.tasks[0].Status)
	}
}

func TestUpdateExecutionPlanTool_RemoveTask(t *testing.T) {
	plan := newMockExecutionPlan()
	plan.AddTask("task1", "Investigate authentication flow")
	plan.AddTask("task2", "Check database queries")
	tool := NewUpdateExecutionPlanTool(plan)

	params := &UpdateExecutionPlanParams{
		Action: "remove",
		TaskID: "task1",
	}

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Task removed successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	if len(plan.tasks) != 1 {
		t.Errorf("Expected 1 task remaining, got %d", len(plan.tasks))
	}

	if plan.tasks[0].ID != "task2" {
		t.Errorf("Expected remaining task to be 'task2', got '%s'", plan.tasks[0].ID)
	}
}

func TestUpdateExecutionPlanTool_ShowPlan(t *testing.T) {
	plan := newMockExecutionPlan()
	plan.AddTask("task1", "Investigate authentication flow")
	plan.AddTask("task2", "Check database queries")
	tool := NewUpdateExecutionPlanTool(plan)

	params := &UpdateExecutionPlanParams{
		Action: "show",
	}

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Current Plan") {
		t.Errorf("Expected plan summary, got: %s", result)
	}
}

func TestUpdateExecutionPlanTool_AddTaskMissingParams(t *testing.T) {
	plan := newMockExecutionPlan()
	tool := NewUpdateExecutionPlanTool(plan)

	params := &UpdateExecutionPlanParams{
		Action: "add",
		TaskID: "task1",
		// Missing description
	}

	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error for missing description")
	}
}

func TestUpdateExecutionPlanTool_UpdateTaskMissingParams(t *testing.T) {
	plan := newMockExecutionPlan()
	tool := NewUpdateExecutionPlanTool(plan)

	params := &UpdateExecutionPlanParams{
		Action: "update",
		TaskID: "task1",
		// Missing status
	}

	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error for missing status")
	}
}

func TestUpdateExecutionPlanTool_InvalidAction(t *testing.T) {
	plan := newMockExecutionPlan()
	tool := NewUpdateExecutionPlanTool(plan)

	params := &UpdateExecutionPlanParams{
		Action: "invalid",
	}

	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error for invalid action")
	}

	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("Expected 'unknown action' error, got: %v", err)
	}
}
