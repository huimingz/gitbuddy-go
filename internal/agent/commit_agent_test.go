package agent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/huimingz/gitbuddy-go/internal/agent/tools"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/huimingz/gitbuddy-go/internal/git"
)

// MockLLMProvider is a mock implementation of llm.Provider for testing
type MockLLMProvider struct {
	cfg config.ModelConfig
}

func (m *MockLLMProvider) Name() string {
	return "mock"
}

func (m *MockLLMProvider) GetConfig() config.ModelConfig {
	return m.cfg
}

func (m *MockLLMProvider) CreateChatModel(ctx context.Context) (model.ChatModel, error) {
	return nil, nil
}

func TestCommitInfo_Title(t *testing.T) {
	tests := []struct {
		name     string
		info     CommitInfo
		expected string
	}{
		{
			name: "simple commit",
			info: CommitInfo{
				Type:        "feat",
				Description: "add new feature",
			},
			expected: "feat: add new feature",
		},
		{
			name: "with scope",
			info: CommitInfo{
				Type:        "fix",
				Scope:       "api",
				Description: "handle nil response",
			},
			expected: "fix(api): handle nil response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.info.Title())
		})
	}
}

func TestCommitInfo_Message(t *testing.T) {
	tests := []struct {
		name     string
		info     CommitInfo
		expected string
	}{
		{
			name: "title only",
			info: CommitInfo{
				Type:        "feat",
				Description: "add feature",
			},
			expected: "feat: add feature",
		},
		{
			name: "with body",
			info: CommitInfo{
				Type:        "feat",
				Description: "add feature",
				Body:        "This is the body",
			},
			expected: "feat: add feature\n\nThis is the body",
		},
		{
			name: "with footer",
			info: CommitInfo{
				Type:        "feat",
				Description: "add feature",
				Footer:      "Closes #123",
			},
			expected: "feat: add feature\n\nCloses #123",
		},
		{
			name: "full commit",
			info: CommitInfo{
				Type:        "feat",
				Scope:       "auth",
				Description: "add OAuth",
				Body:        "Add Google OAuth support",
				Footer:      "BREAKING CHANGE: API changed",
			},
			expected: "feat(auth): add OAuth\n\nAdd Google OAuth support\n\nBREAKING CHANGE: API changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.info.Message())
		})
	}
}

func TestCommitInfo_Validate(t *testing.T) {
	tests := []struct {
		name    string
		info    CommitInfo
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid commit",
			info: CommitInfo{
				Type:        "feat",
				Description: "add feature",
			},
			wantErr: false,
		},
		{
			name: "missing type",
			info: CommitInfo{
				Description: "add feature",
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "invalid type",
			info: CommitInfo{
				Type:        "invalid",
				Description: "add feature",
			},
			wantErr: true,
			errMsg:  "invalid",
		},
		{
			name: "missing description",
			info: CommitInfo{
				Type: "feat",
			},
			wantErr: true,
			errMsg:  "description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommitInfoFromToolParams(t *testing.T) {
	params := &tools.SubmitCommitParams{
		Type:        "feat",
		Scope:       "auth",
		Description: "add login",
		Body:        "Implement JWT",
		Footer:      "Closes #123",
	}

	info := CommitInfoFromToolParams(params)

	assert.Equal(t, "feat", info.Type)
	assert.Equal(t, "auth", info.Scope)
	assert.Equal(t, "add login", info.Description)
	assert.Equal(t, "Implement JWT", info.Body)
	assert.Equal(t, "Closes #123", info.Footer)
}

func TestNewCommitAgent(t *testing.T) {
	mockProvider := &MockLLMProvider{cfg: config.ModelConfig{Provider: "mock", Model: "test"}}
	mockExecutor := &MockGitExecutor{}

	opts := CommitAgentOptions{
		Language:    "en",
		LLMProvider: mockProvider,
		GitExecutor: mockExecutor,
	}

	agent, err := NewCommitAgent(opts)
	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestCommitAgentOptions_Validate(t *testing.T) {
	mockProvider := &MockLLMProvider{cfg: config.ModelConfig{Provider: "mock", Model: "test"}}
	mockExecutor := &MockGitExecutor{}

	t.Run("valid options", func(t *testing.T) {
		opts := CommitAgentOptions{
			Language:    "en",
			LLMProvider: mockProvider,
			GitExecutor: mockExecutor,
		}
		err := opts.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty language defaults to en", func(t *testing.T) {
		opts := CommitAgentOptions{
			LLMProvider: mockProvider,
			GitExecutor: mockExecutor,
		}
		err := opts.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "en", opts.Language)
	})

	t.Run("missing LLM provider", func(t *testing.T) {
		opts := CommitAgentOptions{
			GitExecutor: mockExecutor,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LLM provider is required")
	})

	t.Run("missing Git executor", func(t *testing.T) {
		opts := CommitAgentOptions{
			LLMProvider: mockProvider,
		}
		err := opts.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Git executor is required")
	})
}

func TestBuildSystemPrompt(t *testing.T) {
	t.Run("without context", func(t *testing.T) {
		prompt := BuildSystemPrompt("en", "")
		assert.Contains(t, prompt, "Git commit message generator")
		assert.Contains(t, prompt, "en")
		assert.NotContains(t, prompt, "Additional Context")
	})

	t.Run("with context", func(t *testing.T) {
		prompt := BuildSystemPrompt("zh", "这是一个修复bug的提交")
		assert.Contains(t, prompt, "zh")
		assert.Contains(t, prompt, "Additional Context")
		assert.Contains(t, prompt, "这是一个修复bug的提交")
	})
}

// MockGitExecutor is a mock implementation of git.Executor for testing
type MockGitExecutor struct {
	DiffCachedResult   string
	DiffCachedErr      error
	StatusResult       string
	StatusErr          error
	LogResult          string
	LogErr             error
	CommitErr          error
	CurrentBranchValue string
	CurrentUserValue   string
}

func (m *MockGitExecutor) DiffCached(ctx context.Context) (string, error) {
	return m.DiffCachedResult, m.DiffCachedErr
}

func (m *MockGitExecutor) DiffBranches(ctx context.Context, base, head string) (string, error) {
	return "", nil
}

func (m *MockGitExecutor) Status(ctx context.Context) (string, error) {
	return m.StatusResult, m.StatusErr
}

func (m *MockGitExecutor) Log(ctx context.Context, opts git.LogOptions) (string, error) {
	return m.LogResult, m.LogErr
}

func (m *MockGitExecutor) LogRange(ctx context.Context, base, head string) (string, error) {
	return m.LogResult, m.LogErr
}

func (m *MockGitExecutor) Show(ctx context.Context, ref string) (string, error) {
	return "", nil
}

func (m *MockGitExecutor) ListBranches(ctx context.Context) (string, error) {
	return "", nil
}

func (m *MockGitExecutor) Commit(ctx context.Context, message string) error {
	return m.CommitErr
}

func (m *MockGitExecutor) CurrentBranch(ctx context.Context) (string, error) {
	return m.CurrentBranchValue, nil
}

func (m *MockGitExecutor) CurrentUser(ctx context.Context) (string, error) {
	return m.CurrentUserValue, nil
}
