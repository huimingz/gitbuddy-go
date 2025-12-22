package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// LogOptions represents options for git log command
type LogOptions struct {
	Author string
	Since  string
	Until  string
	Format string
	Count  int
}

// Executor defines the interface for git command execution
type Executor interface {
	// DiffCached returns the diff of staged changes
	DiffCached(ctx context.Context) (string, error)

	// DiffBranches returns the diff between two branches
	DiffBranches(ctx context.Context, base, head string) (string, error)

	// Status returns the current git status
	Status(ctx context.Context) (string, error)

	// Log returns the commit log
	Log(ctx context.Context, opts LogOptions) (string, error)

	// Show returns detailed information about a commit
	Show(ctx context.Context, ref string) (string, error)

	// ListBranches returns all branches
	ListBranches(ctx context.Context) (string, error)

	// Commit executes a git commit with the given message
	Commit(ctx context.Context, message string) error

	// CurrentBranch returns the current branch name
	CurrentBranch(ctx context.Context) (string, error)

	// CurrentUser returns the current git user name
	CurrentUser(ctx context.Context) (string, error)
}

// DefaultExecutor is the default implementation of Executor
type DefaultExecutor struct {
	workDir string
}

// NewExecutor creates a new DefaultExecutor
func NewExecutor(workDir string) *DefaultExecutor {
	return &DefaultExecutor{workDir: workDir}
}

// runGit runs a git command and returns the output
func (e *DefaultExecutor) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = e.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// DiffCached returns the diff of staged changes
func (e *DefaultExecutor) DiffCached(ctx context.Context) (string, error) {
	return e.runGit(ctx, "diff", "--cached")
}

// DiffBranches returns the diff between two branches
func (e *DefaultExecutor) DiffBranches(ctx context.Context, base, head string) (string, error) {
	return e.runGit(ctx, "diff", fmt.Sprintf("%s..%s", base, head))
}

// Status returns the current git status
func (e *DefaultExecutor) Status(ctx context.Context) (string, error) {
	return e.runGit(ctx, "status")
}

// Log returns the commit log
func (e *DefaultExecutor) Log(ctx context.Context, opts LogOptions) (string, error) {
	args := []string{"log"}

	// Add count limit
	if opts.Count > 0 {
		args = append(args, "-n", strconv.Itoa(opts.Count))
	}

	// Add author filter
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}

	// Add date range
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}

	// Add format
	if opts.Format != "" {
		args = append(args, "--format="+opts.Format)
	}

	output, err := e.runGit(ctx, args...)
	if err != nil {
		// Empty repo returns error, return empty string instead
		if strings.Contains(err.Error(), "does not have any commits") {
			return "", nil
		}
		return "", err
	}
	return output, nil
}

// Commit executes a git commit with the given message
func (e *DefaultExecutor) Commit(ctx context.Context, message string) error {
	_, err := e.runGit(ctx, "commit", "-m", message)
	return err
}

// Show returns detailed information about a commit
func (e *DefaultExecutor) Show(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	return e.runGit(ctx, "show", ref, "--stat")
}

// ListBranches returns all branches
func (e *DefaultExecutor) ListBranches(ctx context.Context) (string, error) {
	return e.runGit(ctx, "branch", "-a", "-v")
}

// CurrentBranch returns the current branch name
func (e *DefaultExecutor) CurrentBranch(ctx context.Context) (string, error) {
	return e.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

// CurrentUser returns the current git user name
func (e *DefaultExecutor) CurrentUser(ctx context.Context) (string, error) {
	return e.runGit(ctx, "config", "user.name")
}
