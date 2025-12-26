package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// UpdateExecutionPlanTool allows the agent to manage its execution plan
type UpdateExecutionPlanTool struct {
	plan ExecutionPlanManager
}

// ExecutionPlanManager is an interface for managing the execution plan
type ExecutionPlanManager interface {
	AddTask(id, description string)
	UpdateTask(id, status string) bool
	RemoveTask(id string) bool
	GetSummary() string
	GetChanges(old interface{}) []string
	Clone() interface{}
}

// UpdateExecutionPlanParams defines the parameters for updating the execution plan
type UpdateExecutionPlanParams struct {
	Action      string `json:"action"`
	TaskID      string `json:"task_id,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

// NewUpdateExecutionPlanTool creates a new UpdateExecutionPlanTool
func NewUpdateExecutionPlanTool(plan ExecutionPlanManager) *UpdateExecutionPlanTool {
	return &UpdateExecutionPlanTool{
		plan: plan,
	}
}

// Description returns the tool description
func (t *UpdateExecutionPlanTool) Description() string {
	return `Manage the investigation execution plan dynamically. Use this to:
- Create initial investigation tasks at the start (action: "add")
- Mark tasks as in_progress when you start them (action: "update", status: "in_progress")
- Mark tasks as completed when finished (action: "update", status: "completed")
- Add new tasks as you discover new areas to investigate (action: "add")
- Remove tasks that become irrelevant (action: "remove")
- Show the current plan status (action: "show")

The system will automatically display plan changes to the user.

Parameters:
- action (required): "add", "update", "remove", or "show"
- task_id (required for update/remove): Unique identifier for the task
- description (required for add): Task description
- status (required for update): "pending", "in_progress", "completed", or "skipped"`
}

// Execute executes the tool
func (t *UpdateExecutionPlanTool) Execute(ctx context.Context, params *UpdateExecutionPlanParams) (string, error) {
	switch params.Action {
	case "add":
		if params.TaskID == "" || params.Description == "" {
			return "", fmt.Errorf("task_id and description are required for add action")
		}

		// Clone the old plan for comparison
		oldPlan := t.plan.Clone()

		t.plan.AddTask(params.TaskID, params.Description)

		// Get changes
		changes := t.plan.GetChanges(oldPlan)

		result := fmt.Sprintf("Task added successfully.\n\nChanges:\n")
		for _, change := range changes {
			result += fmt.Sprintf("- %s\n", change)
		}
		result += "\n" + t.plan.GetSummary()

		return result, nil

	case "update":
		if params.TaskID == "" || params.Status == "" {
			return "", fmt.Errorf("task_id and status are required for update action")
		}

		// Clone the old plan for comparison
		oldPlan := t.plan.Clone()

		changed := t.plan.UpdateTask(params.TaskID, params.Status)
		if !changed {
			return fmt.Sprintf("Task '%s' not found or status unchanged.\n\n%s", params.TaskID, t.plan.GetSummary()), nil
		}

		// Get changes
		changes := t.plan.GetChanges(oldPlan)

		result := fmt.Sprintf("Task status updated successfully.\n\nChanges:\n")
		for _, change := range changes {
			result += fmt.Sprintf("- %s\n", change)
		}
		result += "\n" + t.plan.GetSummary()

		return result, nil

	case "remove":
		if params.TaskID == "" {
			return "", fmt.Errorf("task_id is required for remove action")
		}

		// Clone the old plan for comparison
		oldPlan := t.plan.Clone()

		removed := t.plan.RemoveTask(params.TaskID)
		if !removed {
			return fmt.Sprintf("Task '%s' not found.\n\n%s", params.TaskID, t.plan.GetSummary()), nil
		}

		// Get changes
		changes := t.plan.GetChanges(oldPlan)

		result := fmt.Sprintf("Task removed successfully.\n\nChanges:\n")
		for _, change := range changes {
			result += fmt.Sprintf("- %s\n", change)
		}
		result += "\n" + t.plan.GetSummary()

		return result, nil

	case "show":
		return t.plan.GetSummary(), nil

	default:
		return "", fmt.Errorf("unknown action: %s (must be add, update, remove, or show)", params.Action)
	}
}

// ExecuteString executes the tool with string parameters
func (t *UpdateExecutionPlanTool) ExecuteString(params string) (string, error) {
	var p UpdateExecutionPlanParams
	if err := json.Unmarshal([]byte(params), &p); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	return t.Execute(context.Background(), &p)
}
