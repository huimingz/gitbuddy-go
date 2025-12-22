package tools

import (
	"context"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitShowParams represents the parameters for the git_show tool
type GitShowParams struct {
	// Ref is the commit reference to show (default: HEAD)
	Ref string `json:"ref,omitempty" jsonschema:"description=Commit reference to show (commit hash, branch name, tag, or HEAD). Default: HEAD"`
}

// GitShowTool is a tool for showing commit details
type GitShowTool struct {
	executor git.Executor
}

// NewGitShowTool creates a new GitShowTool
func NewGitShowTool(executor git.Executor) *GitShowTool {
	return &GitShowTool{executor: executor}
}

// Name returns the tool name
func (t *GitShowTool) Name() string {
	return "git_show"
}

// Description returns the tool description
func (t *GitShowTool) Description() string {
	return `Show detailed information about a specific commit (git show).
This displays the commit message, author, date, and a summary of changes.
Parameters:
- ref: Commit reference to show (commit hash, branch name, tag, or HEAD). Default: HEAD`
}

// Execute runs the tool and returns the commit details
func (t *GitShowTool) Execute(ctx context.Context, params *GitShowParams) (string, error) {
	ref := "HEAD"
	if params != nil && params.Ref != "" {
		ref = params.Ref
	}

	output, err := t.executor.Show(ctx, ref)
	if err != nil {
		return "", err
	}

	if output == "" {
		return "No commit found for reference: " + ref, nil
	}

	return output, nil
}
