package tools

import (
	"context"
	"fmt"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitLogRangeParams represents the parameters for the git_log_range tool
type GitLogRangeParams struct {
	// Base is the base branch/ref to compare from
	Base string `json:"base" jsonschema:"description=Base branch or ref to compare from (e.g., main)"`
	// Head is the head branch/ref to compare to (defaults to HEAD)
	Head string `json:"head,omitempty" jsonschema:"description=Head branch or ref to compare to (defaults to HEAD)"`
}

// GitLogRangeTool is a tool for getting commit log between two refs
type GitLogRangeTool struct {
	executor git.Executor
}

// NewGitLogRangeTool creates a new GitLogRangeTool
func NewGitLogRangeTool(executor git.Executor) *GitLogRangeTool {
	return &GitLogRangeTool{executor: executor}
}

// Name returns the tool name
func (t *GitLogRangeTool) Name() string {
	return "git_log_range"
}

// Description returns the tool description
func (t *GitLogRangeTool) Description() string {
	return `Get the commit log between two branches or refs (git log base..head).
This shows all commits that are in head but not in base.
Useful for understanding what commits would be included in a pull request.
Parameters:
- base: The base branch or ref to compare from (required, e.g., "main")
- head: The head branch or ref to compare to (optional, defaults to HEAD)`
}

// Execute runs the tool and returns the log
func (t *GitLogRangeTool) Execute(ctx context.Context, params interface{}) (string, error) {
	p, ok := params.(*GitLogRangeParams)
	if !ok || p == nil {
		return "", fmt.Errorf("invalid parameters: expected GitLogRangeParams")
	}

	if p.Base == "" {
		return "", fmt.Errorf("base branch/ref is required")
	}

	head := p.Head
	if head == "" {
		head = "HEAD"
	}

	log, err := t.executor.LogRange(ctx, p.Base, head)
	if err != nil {
		return "", err
	}

	if log == "" {
		return fmt.Sprintf("No commits found between %s and %s", p.Base, head), nil
	}

	return log, nil
}
