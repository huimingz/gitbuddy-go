package tools

import (
	"context"
	"fmt"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitLogDateParams represents the parameters for the git_log_date tool
type GitLogDateParams struct {
	// Since is the start date (e.g., "2024-01-01")
	Since string `json:"since" jsonschema:"description=Start date in YYYY-MM-DD format (e.g., 2024-01-01)"`
	// Until is the end date (optional, defaults to today)
	Until string `json:"until,omitempty" jsonschema:"description=End date in YYYY-MM-DD format (optional, defaults to today)"`
	// Author is the author name filter (optional)
	Author string `json:"author,omitempty" jsonschema:"description=Filter by author name (optional)"`
}

// GitLogDateTool is a tool for getting commit log within a date range
type GitLogDateTool struct {
	executor git.Executor
}

// NewGitLogDateTool creates a new GitLogDateTool
func NewGitLogDateTool(executor git.Executor) *GitLogDateTool {
	return &GitLogDateTool{executor: executor}
}

// Name returns the tool name
func (t *GitLogDateTool) Name() string {
	return "git_log_date"
}

// Description returns the tool description
func (t *GitLogDateTool) Description() string {
	return `Get the commit log within a date range (git log --since --until).
This shows all commits within the specified date range.
Useful for generating development reports for a specific period.
Parameters:
- since: Start date in YYYY-MM-DD format (required)
- until: End date in YYYY-MM-DD format (optional, defaults to today)
- author: Filter by author name (optional)`
}

// Execute runs the tool and returns the log
func (t *GitLogDateTool) Execute(ctx context.Context, params interface{}) (string, error) {
	p, ok := params.(*GitLogDateParams)
	if !ok || p == nil {
		return "", fmt.Errorf("invalid parameters: expected GitLogDateParams")
	}

	if p.Since == "" {
		return "", fmt.Errorf("since date is required")
	}

	opts := git.LogOptions{
		Since:  p.Since,
		Until:  p.Until,
		Author: p.Author,
		Format: "%h|%s|%ad",
	}

	log, err := t.executor.Log(ctx, opts)
	if err != nil {
		return "", err
	}

	if log == "" {
		return fmt.Sprintf("No commits found between %s and %s", p.Since, p.Until), nil
	}

	return log, nil
}
