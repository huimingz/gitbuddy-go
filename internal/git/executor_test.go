package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	return tmpDir
}

// createAndStageFile creates a file and stages it
func createAndStageFile(t *testing.T, repoDir, filename, content string) {
	t.Helper()

	filePath := filepath.Join(repoDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	cmd := exec.Command("git", "add", filename)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
}

// commitFile commits staged changes
func commitFile(t *testing.T, repoDir, message string) {
	t.Helper()

	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
}

func TestNewExecutor(t *testing.T) {
	executor := NewExecutor("/tmp/test")
	assert.NotNil(t, executor)
}

func TestExecutor_DiffCached(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	t.Run("empty staging area", func(t *testing.T) {
		diff, err := executor.DiffCached(ctx)
		require.NoError(t, err)
		assert.Empty(t, diff)
	})

	t.Run("with staged changes", func(t *testing.T) {
		createAndStageFile(t, repoDir, "test.txt", "hello world")

		diff, err := executor.DiffCached(ctx)
		require.NoError(t, err)
		assert.Contains(t, diff, "test.txt")
		assert.Contains(t, diff, "hello world")
	})
}

func TestExecutor_DiffCached_MultipleFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	createAndStageFile(t, repoDir, "file1.go", "package main")
	createAndStageFile(t, repoDir, "file2.go", "package test")

	diff, err := executor.DiffCached(ctx)
	require.NoError(t, err)
	assert.Contains(t, diff, "file1.go")
	assert.Contains(t, diff, "file2.go")
}

func TestExecutor_Status(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	t.Run("clean repo", func(t *testing.T) {
		status, err := executor.Status(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, status)
	})

	t.Run("with staged file", func(t *testing.T) {
		createAndStageFile(t, repoDir, "new.txt", "content")

		status, err := executor.Status(ctx)
		require.NoError(t, err)
		assert.Contains(t, status, "new.txt")
	})
}

func TestExecutor_Log(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	t.Run("empty repo", func(t *testing.T) {
		log, err := executor.Log(ctx, LogOptions{Count: 5})
		// Empty repo might return error or empty string
		if err == nil {
			assert.Empty(t, log)
		}
	})

	t.Run("with commits", func(t *testing.T) {
		createAndStageFile(t, repoDir, "first.txt", "first")
		commitFile(t, repoDir, "feat: first commit")

		createAndStageFile(t, repoDir, "second.txt", "second")
		commitFile(t, repoDir, "fix: second commit")

		log, err := executor.Log(ctx, LogOptions{Count: 5})
		require.NoError(t, err)
		assert.Contains(t, log, "first commit")
		assert.Contains(t, log, "second commit")
	})

	t.Run("with count limit", func(t *testing.T) {
		log, err := executor.Log(ctx, LogOptions{Count: 1})
		require.NoError(t, err)
		assert.Contains(t, log, "second commit")
		assert.NotContains(t, log, "first commit")
	})
}

func TestExecutor_Commit(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	t.Run("commit staged changes", func(t *testing.T) {
		createAndStageFile(t, repoDir, "commit-test.txt", "test content")

		err := executor.Commit(ctx, "test: commit message")
		require.NoError(t, err)

		// Verify commit was created
		log, err := executor.Log(ctx, LogOptions{Count: 1})
		require.NoError(t, err)
		assert.Contains(t, log, "commit message")
	})

	t.Run("commit with body", func(t *testing.T) {
		createAndStageFile(t, repoDir, "commit-body.txt", "body test")

		message := "feat: add feature\n\nThis is the body of the commit.\nIt explains what and why."
		err := executor.Commit(ctx, message)
		require.NoError(t, err)

		log, err := executor.Log(ctx, LogOptions{Count: 1})
		require.NoError(t, err)
		assert.Contains(t, log, "add feature")
	})

	t.Run("commit with empty staging area fails", func(t *testing.T) {
		err := executor.Commit(ctx, "empty commit")
		assert.Error(t, err)
	})
}

func TestExecutor_CurrentBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	// Need at least one commit to have a branch
	createAndStageFile(t, repoDir, "init.txt", "init")
	commitFile(t, repoDir, "initial commit")

	branch, err := executor.CurrentBranch(ctx)
	require.NoError(t, err)
	// Default branch could be "main" or "master"
	assert.True(t, branch == "main" || branch == "master", "branch should be main or master, got: %s", branch)
}

func TestExecutor_CurrentUser(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	user, err := executor.CurrentUser(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Test User", user)
}

func TestExecutor_DiffBranches(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := NewExecutor(repoDir)
	ctx := context.Background()

	// Create initial commit on main
	createAndStageFile(t, repoDir, "main.txt", "main content")
	commitFile(t, repoDir, "initial commit")

	// Create a feature branch
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// Add commits on feature branch
	createAndStageFile(t, repoDir, "feature.txt", "feature content")
	commitFile(t, repoDir, "feat: add feature")

	// Get current branch name (main or master)
	mainBranch, _ := executor.CurrentBranch(ctx)
	cmd = exec.Command("git", "checkout", "-")
	cmd.Dir = repoDir
	cmd.Run()
	mainBranch, _ = executor.CurrentBranch(ctx)
	cmd = exec.Command("git", "checkout", "feature")
	cmd.Dir = repoDir
	cmd.Run()

	diff, err := executor.DiffBranches(ctx, mainBranch, "HEAD")
	require.NoError(t, err)
	assert.Contains(t, diff, "feature.txt")
}

func TestExecutor_NotAGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewExecutor(tmpDir)
	ctx := context.Background()

	_, err := executor.Status(ctx)
	assert.Error(t, err)
}
