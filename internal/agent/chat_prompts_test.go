package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetChatSystemPrompt tests system prompt generation
func TestGetChatSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		language string
		want     string
	}{
		{
			name:     "English prompt",
			language: "en",
			want:     "GitBuddy",
		},
		{
			name:     "Chinese prompt",
			language: "zh",
			want:     "GitBuddy",
		},
		{
			name:     "Chinese simplified",
			language: "zh-cn",
			want:     "GitBuddy",
		},
		{
			name:     "Unknown language defaults to English",
			language: "unknown",
			want:     "GitBuddy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := GetChatSystemPrompt(tt.language)
			require.NotEmpty(t, prompt)
			assert.Contains(t, prompt, tt.want)
		})
	}
}

// TestGetChatWelcomeMessage tests welcome message generation
func TestGetChatWelcomeMessage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		want     string
	}{
		{
			name:     "English welcome",
			language: "en",
			want:     "Welcome to GitBuddy Chat",
		},
		{
			name:     "Chinese welcome",
			language: "zh",
			want:     "GitBuddy Chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetChatWelcomeMessage(tt.language)
			require.NotEmpty(t, msg)
			assert.Contains(t, msg, tt.want)
		})
	}
}

// TestGetChatExitMessage tests exit message generation
func TestGetChatExitMessage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		wantZh   bool
	}{
		{
			name:     "English exit message",
			language: "en",
			wantZh:   false,
		},
		{
			name:     "Chinese exit message",
			language: "zh",
			wantZh:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetChatExitMessage(tt.language)
			require.NotEmpty(t, msg)

			if tt.wantZh {
				assert.True(t, strings.Contains(msg, "谢谢") || strings.Contains(msg, "goodbye"))
			} else {
				assert.True(t, strings.Contains(msg, "Thank") || strings.Contains(msg, "Goodbye"))
			}
		})
	}
}

// TestGetChatErrorMessage tests error message generation
func TestGetChatErrorMessage(t *testing.T) {
	tests := []struct {
		name        string
		language    string
		messageKey  string
		args        []interface{}
		shouldExist bool
	}{
		{
			name:        "agent_timeout error in English",
			language:    "en",
			messageKey:  "agent_timeout",
			args:        []interface{}{30},
			shouldExist: true,
		},
		{
			name:        "agent_timeout error in Chinese",
			language:    "zh",
			messageKey:  "agent_timeout",
			args:        []interface{}{30},
			shouldExist: true,
		},
		{
			name:        "tool_error in English",
			language:    "en",
			messageKey:  "tool_error",
			args:        []interface{}{"test error"},
			shouldExist: true,
		},
		{
			name:        "invalid message key",
			language:    "en",
			messageKey:  "unknown_key",
			args:        []interface{}{},
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetChatErrorMessage(tt.language, tt.messageKey, tt.args...)
			require.NotEmpty(t, msg)

			if !tt.shouldExist {
				// Unknown keys should return a default message
				assert.Contains(t, strings.ToLower(msg), "error")
			}
		})
	}
}
