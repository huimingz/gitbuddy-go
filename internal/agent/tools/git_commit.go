package tools

import (
	"context"
	"fmt"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitCommitParams represents parameters for the git commit tool
type GitCommitParams struct {
	Message string `json:"message" jsonschema:"description=The commit message to use for git commit"`
}

// GitCommitTool is a tool that executes git commit
type GitCommitTool struct {
	executor git.Executor
}

// NewGitCommitTool creates a new GitCommitTool
func NewGitCommitTool(executor git.Executor) *GitCommitTool {
	return &GitCommitTool{
		executor: executor,
	}
}

// Name returns the tool name
func (t *GitCommitTool) Name() string {
	return "git_commit"
}

// Description returns the tool description
func (t *GitCommitTool) Description() string {
	return `Execute git commit with the provided message.
Use this after generating a commit message to actually commit the staged changes.`
}

// Execute runs git commit with the provided message
func (t *GitCommitTool) Execute(ctx context.Context, params *GitCommitParams) (string, error) {
	if params == nil || params.Message == "" {
		return "", fmt.Errorf("commit message is required")
	}

	err := t.executor.Commit(ctx, params.Message)
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	return "Commit successful", nil
}
