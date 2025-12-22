package tools

import (
	"context"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitStatusTool is a tool for getting git status
type GitStatusTool struct {
	executor git.Executor
}

// NewGitStatusTool creates a new GitStatusTool
func NewGitStatusTool(executor git.Executor) *GitStatusTool {
	return &GitStatusTool{executor: executor}
}

// Name returns the tool name
func (t *GitStatusTool) Name() string {
	return "git_status"
}

// Description returns the tool description
func (t *GitStatusTool) Description() string {
	return `Get the current git repository status (git status).
This shows the state of the working directory and staging area, including:
- Files that are staged for commit
- Files that have been modified but not staged
- Untracked files`
}

// Execute runs the tool and returns the status
func (t *GitStatusTool) Execute(ctx context.Context, params interface{}) (string, error) {
	return t.executor.Status(ctx)
}
