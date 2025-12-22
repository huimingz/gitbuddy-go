package tools

import (
	"context"
	"fmt"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitBranchTool is a tool for getting branch information
type GitBranchTool struct {
	executor git.Executor
}

// NewGitBranchTool creates a new GitBranchTool
func NewGitBranchTool(executor git.Executor) *GitBranchTool {
	return &GitBranchTool{executor: executor}
}

// Name returns the tool name
func (t *GitBranchTool) Name() string {
	return "git_branch"
}

// Description returns the tool description
func (t *GitBranchTool) Description() string {
	return `Get information about git branches.
This shows the current branch and lists all local and remote branches with their latest commit info.
Useful for understanding the branch context of the current changes.`
}

// Execute runs the tool and returns branch information
func (t *GitBranchTool) Execute(ctx context.Context, params interface{}) (string, error) {
	// Get current branch
	currentBranch, err := t.executor.CurrentBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get all branches
	branches, err := t.executor.ListBranches(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	result := fmt.Sprintf("Current branch: %s\n\nAll branches:\n%s", currentBranch, branches)
	return result, nil
}
