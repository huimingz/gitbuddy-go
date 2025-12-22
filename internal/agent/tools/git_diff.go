package tools

import (
	"context"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitDiffCachedTool is a tool for getting staged diff
type GitDiffCachedTool struct {
	executor git.Executor
}

// NewGitDiffCachedTool creates a new GitDiffCachedTool
func NewGitDiffCachedTool(executor git.Executor) *GitDiffCachedTool {
	return &GitDiffCachedTool{executor: executor}
}

// Name returns the tool name
func (t *GitDiffCachedTool) Name() string {
	return "git_diff_cached"
}

// Description returns the tool description
func (t *GitDiffCachedTool) Description() string {
	return `Get the diff of staged changes (git diff --cached).
This shows the changes that have been added to the staging area and are ready to be committed.
Use this tool to understand what changes will be included in the next commit.`
}

// Execute runs the tool and returns the diff
func (t *GitDiffCachedTool) Execute(ctx context.Context, params interface{}) (string, error) {
	diff, err := t.executor.DiffCached(ctx)
	if err != nil {
		return "", err
	}

	if diff == "" {
		return "No staged changes found. Please stage some changes using 'git add' first.", nil
	}

	return diff, nil
}
