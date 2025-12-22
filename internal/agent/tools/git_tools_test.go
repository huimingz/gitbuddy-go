package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/huimingz/gitbuddy-go/internal/git"
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

func TestNewGitDiffCachedTool(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)

	tool := NewGitDiffCachedTool(executor)
	assert.NotNil(t, tool)
	assert.Equal(t, "git_diff_cached", tool.Name())
	assert.NotEmpty(t, tool.Description())
}

func TestGitDiffCachedTool_Execute(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)
	tool := NewGitDiffCachedTool(executor)
	ctx := context.Background()

	t.Run("empty staging area", func(t *testing.T) {
		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "No staged changes")
	})

	t.Run("with staged changes", func(t *testing.T) {
		createAndStageFile(t, repoDir, "test.go", "package main\n\nfunc main() {}\n")

		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "test.go")
		assert.Contains(t, result, "package main")
	})
}

func TestNewGitStatusTool(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)

	tool := NewGitStatusTool(executor)
	assert.NotNil(t, tool)
	assert.Equal(t, "git_status", tool.Name())
	assert.NotEmpty(t, tool.Description())
}

func TestGitStatusTool_Execute(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)
	tool := NewGitStatusTool(executor)
	ctx := context.Background()

	t.Run("clean repo", func(t *testing.T) {
		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("with staged file", func(t *testing.T) {
		createAndStageFile(t, repoDir, "status-test.txt", "content")

		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "status-test.txt")
	})
}

func TestNewGitLogTool(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)

	tool := NewGitLogTool(executor)
	assert.NotNil(t, tool)
	assert.Equal(t, "git_log", tool.Name())
	assert.NotEmpty(t, tool.Description())
}

func TestGitLogTool_Execute(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)
	tool := NewGitLogTool(executor)
	ctx := context.Background()

	// Create some commits
	createAndStageFile(t, repoDir, "first.txt", "first")
	commitFile(t, repoDir, "feat: first feature")

	createAndStageFile(t, repoDir, "second.txt", "second")
	commitFile(t, repoDir, "fix: bug fix")

	t.Run("default count", func(t *testing.T) {
		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "first feature")
		assert.Contains(t, result, "bug fix")
	})

	t.Run("with count parameter", func(t *testing.T) {
		params := &GitLogParams{Count: 1}
		result, err := tool.Execute(ctx, params)
		require.NoError(t, err)
		assert.Contains(t, result, "bug fix")
		assert.NotContains(t, result, "first feature")
	})
}

func TestNewSubmitCommitTool(t *testing.T) {
	tool := NewSubmitCommitTool(nil) // callback can be nil for this test
	assert.NotNil(t, tool)
	assert.Equal(t, "submit_commit", tool.Name())
	assert.NotEmpty(t, tool.Description())
}

func TestSubmitCommitTool_Execute(t *testing.T) {
	var capturedInfo *SubmitCommitParams
	callback := func(info *SubmitCommitParams) error {
		capturedInfo = info
		return nil
	}

	tool := NewSubmitCommitTool(callback)
	ctx := context.Background()

	t.Run("valid commit info", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type:        "feat",
			Scope:       "auth",
			Description: "add user login",
			Body:        "Implement JWT authentication",
			Footer:      "Closes #123",
		}

		result, err := tool.Execute(ctx, params)
		require.NoError(t, err)
		assert.Contains(t, result, "submitted")

		// Verify callback was called with correct params
		require.NotNil(t, capturedInfo)
		assert.Equal(t, "feat", capturedInfo.Type)
		assert.Equal(t, "auth", capturedInfo.Scope)
		assert.Equal(t, "add user login", capturedInfo.Description)
	})

	t.Run("missing required type", func(t *testing.T) {
		params := &SubmitCommitParams{
			Description: "some change",
		}

		_, err := tool.Execute(ctx, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type")
	})

	t.Run("missing required description", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type: "feat",
		}

		_, err := tool.Execute(ctx, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "description")
	})

	t.Run("invalid type", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type:        "invalid",
			Description: "some change",
		}

		_, err := tool.Execute(ctx, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})
}

func TestSubmitCommitParams_FormatMessage(t *testing.T) {
	t.Run("simple commit", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type:        "feat",
			Description: "add new feature",
		}

		msg := params.FormatMessage()
		assert.Equal(t, "feat: add new feature", msg)
	})

	t.Run("with scope", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type:        "fix",
			Scope:       "api",
			Description: "handle nil response",
		}

		msg := params.FormatMessage()
		assert.Equal(t, "fix(api): handle nil response", msg)
	})

	t.Run("with body", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type:        "feat",
			Description: "add login",
			Body:        "Implement JWT authentication\nAdd password hashing",
		}

		msg := params.FormatMessage()
		expected := "feat: add login\n\nImplement JWT authentication\nAdd password hashing"
		assert.Equal(t, expected, msg)
	})

	t.Run("with footer", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type:        "feat",
			Description: "add feature",
			Footer:      "BREAKING CHANGE: API changed",
		}

		msg := params.FormatMessage()
		expected := "feat: add feature\n\nBREAKING CHANGE: API changed"
		assert.Equal(t, expected, msg)
	})

	t.Run("full commit", func(t *testing.T) {
		params := &SubmitCommitParams{
			Type:        "feat",
			Scope:       "auth",
			Description: "add OAuth support",
			Body:        "- Add Google OAuth\n- Add GitHub OAuth",
			Footer:      "Closes #456",
		}

		msg := params.FormatMessage()
		expected := "feat(auth): add OAuth support\n\n- Add Google OAuth\n- Add GitHub OAuth\n\nCloses #456"
		assert.Equal(t, expected, msg)
	})
}

func TestSubmitCommitParams_Validate(t *testing.T) {
	validTypes := []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "chore", "build", "ci", "revert"}

	for _, typ := range validTypes {
		t.Run("valid type: "+typ, func(t *testing.T) {
			params := &SubmitCommitParams{
				Type:        typ,
				Description: "test",
			}
			err := params.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestNewGitCommitTool(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)

	tool := NewGitCommitTool(executor)
	assert.NotNil(t, tool)
	assert.Equal(t, "git_commit", tool.Name())
	assert.NotEmpty(t, tool.Description())
}

func TestGitCommitTool_Execute(t *testing.T) {
	t.Run("successful commit", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitCommitTool(executor)
		ctx := context.Background()

		// Create and stage a file
		createAndStageFile(t, repoDir, "commit-test.txt", "test content")

		params := &GitCommitParams{
			Message: "feat: add test file",
		}

		result, err := tool.Execute(ctx, params)
		require.NoError(t, err)
		assert.Contains(t, result, "successful")
	})

	t.Run("empty message", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitCommitTool(executor)
		ctx := context.Background()

		params := &GitCommitParams{
			Message: "",
		}

		_, err := tool.Execute(ctx, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message is required")
	})

	t.Run("commit with no staged changes", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitCommitTool(executor)
		ctx := context.Background()

		params := &GitCommitParams{
			Message: "feat: empty commit",
		}

		_, err := tool.Execute(ctx, params)
		assert.Error(t, err) // Git will fail if nothing to commit
	})

	t.Run("multiline commit message", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitCommitTool(executor)
		ctx := context.Background()

		// Create and stage a file
		createAndStageFile(t, repoDir, "multiline-test.txt", "content")

		params := &GitCommitParams{
			Message: "feat(auth): add login\n\nImplement JWT authentication\n\nCloses #123",
		}

		result, err := tool.Execute(ctx, params)
		require.NoError(t, err)
		assert.Contains(t, result, "successful")

		// Verify the commit was created with the full message
		cmd := exec.Command("git", "log", "-1", "--format=%B")
		cmd.Dir = repoDir
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.Contains(t, string(output), "feat(auth): add login")
		assert.Contains(t, string(output), "Implement JWT authentication")
		assert.Contains(t, string(output), "Closes #123")
	})
}

func TestNewGitShowTool(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)

	tool := NewGitShowTool(executor)
	assert.NotNil(t, tool)
	assert.Equal(t, "git_show", tool.Name())
	assert.NotEmpty(t, tool.Description())
}

func TestGitShowTool_Execute(t *testing.T) {
	t.Run("show HEAD commit", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitShowTool(executor)
		ctx := context.Background()

		// Create a commit first
		createAndStageFile(t, repoDir, "show-test.txt", "content")
		commitFile(t, repoDir, "feat: test commit for show")

		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "feat: test commit for show")
		assert.Contains(t, result, "show-test.txt")
	})

	t.Run("show specific commit", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitShowTool(executor)
		ctx := context.Background()

		// Create commits
		createAndStageFile(t, repoDir, "first.txt", "first")
		commitFile(t, repoDir, "feat: first commit")

		createAndStageFile(t, repoDir, "second.txt", "second")
		commitFile(t, repoDir, "feat: second commit")

		params := &GitShowParams{Ref: "HEAD~1"}
		result, err := tool.Execute(ctx, params)
		require.NoError(t, err)
		assert.Contains(t, result, "feat: first commit")
	})
}

func TestNewGitBranchTool(t *testing.T) {
	repoDir := setupTestRepo(t)
	executor := git.NewExecutor(repoDir)

	tool := NewGitBranchTool(executor)
	assert.NotNil(t, tool)
	assert.Equal(t, "git_branch", tool.Name())
	assert.NotEmpty(t, tool.Description())
}

func TestGitBranchTool_Execute(t *testing.T) {
	t.Run("get branch info", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitBranchTool(executor)
		ctx := context.Background()

		// Create a commit first (needed for branches to work)
		createAndStageFile(t, repoDir, "branch-test.txt", "content")
		commitFile(t, repoDir, "feat: initial commit")

		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "Current branch:")
		assert.Contains(t, result, "All branches:")
	})

	t.Run("shows multiple branches", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		executor := git.NewExecutor(repoDir)
		tool := NewGitBranchTool(executor)
		ctx := context.Background()

		// Create initial commit
		createAndStageFile(t, repoDir, "init.txt", "content")
		commitFile(t, repoDir, "feat: initial")

		// Create a new branch
		cmd := exec.Command("git", "branch", "feature-test")
		cmd.Dir = repoDir
		require.NoError(t, cmd.Run())

		result, err := tool.Execute(ctx, nil)
		require.NoError(t, err)
		assert.Contains(t, result, "feature-test")
	})
}
