package tools

import (
	"context"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitLogParams represents the parameters for the git_log tool
type GitLogParams struct {
	// Count is the number of commits to retrieve (default: 5)
	Count int `json:"count,omitempty" jsonschema:"description=Number of commits to retrieve (default 5)"`
}

// GitLogTool is a tool for getting git log
type GitLogTool struct {
	executor git.Executor
}

// NewGitLogTool creates a new GitLogTool
func NewGitLogTool(executor git.Executor) *GitLogTool {
	return &GitLogTool{executor: executor}
}

// Name returns the tool name
func (t *GitLogTool) Name() string {
	return "git_log"
}

// Description returns the tool description
func (t *GitLogTool) Description() string {
	return `Get the recent commit history (git log).
This shows the recent commits in the repository, useful for understanding the project context and recent changes.
Parameters:
- count: Number of commits to retrieve (default: 5)`
}

// Execute runs the tool and returns the log
func (t *GitLogTool) Execute(ctx context.Context, params interface{}) (string, error) {
	opts := git.LogOptions{
		Count: 5, // default
	}

	// Parse params if provided
	if p, ok := params.(*GitLogParams); ok && p != nil {
		if p.Count > 0 {
			opts.Count = p.Count
		}
	}

	log, err := t.executor.Log(ctx, opts)
	if err != nil {
		return "", err
	}

	if log == "" {
		return "No commits found in this repository.", nil
	}

	return log, nil
}
