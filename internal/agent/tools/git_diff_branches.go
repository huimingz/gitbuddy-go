package tools

import (
	"context"
	"fmt"

	"github.com/huimingz/gitbuddy-go/internal/git"
)

// GitDiffBranchesParams represents the parameters for the git_diff_branches tool
type GitDiffBranchesParams struct {
	// Base is the base branch to compare from
	Base string `json:"base" jsonschema:"description=Base branch to compare from (e.g., main, develop)"`
	// Head is the head branch to compare to (defaults to current branch)
	Head string `json:"head,omitempty" jsonschema:"description=Head branch to compare to (defaults to HEAD)"`
}

// GitDiffBranchesTool is a tool for getting diff between two branches
type GitDiffBranchesTool struct {
	executor git.Executor
}

// NewGitDiffBranchesTool creates a new GitDiffBranchesTool
func NewGitDiffBranchesTool(executor git.Executor) *GitDiffBranchesTool {
	return &GitDiffBranchesTool{executor: executor}
}

// Name returns the tool name
func (t *GitDiffBranchesTool) Name() string {
	return "git_diff_branches"
}

// Description returns the tool description
func (t *GitDiffBranchesTool) Description() string {
	return `Get the diff between two branches (git diff base..head).
This shows all code changes between the base branch and head branch.
Useful for understanding what changes would be included in a pull request.
Parameters:
- base: The base branch to compare from (required, e.g., "main")
- head: The head branch to compare to (optional, defaults to HEAD)`
}

// Execute runs the tool and returns the diff
func (t *GitDiffBranchesTool) Execute(ctx context.Context, params interface{}) (string, error) {
	p, ok := params.(*GitDiffBranchesParams)
	if !ok || p == nil {
		return "", fmt.Errorf("invalid parameters: expected GitDiffBranchesParams")
	}

	if p.Base == "" {
		return "", fmt.Errorf("base branch is required")
	}

	head := p.Head
	if head == "" {
		head = "HEAD"
	}

	diff, err := t.executor.DiffBranches(ctx, p.Base, head)
	if err != nil {
		return "", err
	}

	if diff == "" {
		return fmt.Sprintf("No differences found between %s and %s", p.Base, head), nil
	}

	return diff, nil
}
