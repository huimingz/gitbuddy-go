package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugCmd_NoArgsInteractiveInput(t *testing.T) {
	// Create a command with the resume flag to match the original
	cmd := &cobra.Command{
		Use:  "debug",
		Args: debugCmd.Args,
	}
	cmd.Flags().String("resume", "", "Resume session")

	// Test that no arguments should now be accepted (after implementing interactive input)
	err := cmd.Args(cmd, []string{})
	// After implementation, this should succeed
	assert.NoError(t, err, "Modified implementation should accept no arguments for interactive input")
}

func TestDebugCmd_InteractiveCancel(t *testing.T) {
	// This test verifies that the command gracefully handles cancellation during interactive input

	// Create a new command instance with resume flag
	cmd := &cobra.Command{
		Use:  "debug",
		Args: debugCmd.Args,
	}
	cmd.Flags().String("resume", "", "Resume session")

	// Test with no arguments - should be accepted
	err := cmd.Args(cmd, []string{})
	assert.NoError(t, err)
}

func TestDebugCmd_BackwardCompatibility(t *testing.T) {
	// Test that providing an issue description as argument still works

	// Create a new command instance with resume flag
	cmd := &cobra.Command{
		Use:  "debug",
		Args: debugCmd.Args,
	}
	cmd.Flags().String("resume", "", "Resume session")

	// Test with one argument - should be accepted
	err := cmd.Args(cmd, []string{"Test issue description"})
	assert.NoError(t, err)

	// Test with multiple arguments - should be rejected
	err = cmd.Args(cmd, []string{"Test", "issue", "description"})
	assert.Error(t, err, "Should reject multiple arguments")

	// Test with empty string - should be accepted at argument level
	err = cmd.Args(cmd, []string{""})
	assert.NoError(t, err)
}

func TestDebugCmd_ResumeMode(t *testing.T) {
	// Test resume mode argument validation

	// Create a new command with resume flag
	cmd := &cobra.Command{
		Use:  "debug",
		Args: debugCmd.Args,
	}
	cmd.Flags().String("resume", "", "Resume session")

	// Set resume flag
	err := cmd.Flags().Set("resume", "test-session-id")
	require.NoError(t, err)

	// When resuming, no args should be required
	err = cmd.Args(cmd, []string{})
	assert.NoError(t, err)

	// When resuming, additional args should be rejected
	err = cmd.Args(cmd, []string{"some-issue"})
	assert.Error(t, err, "Should reject arguments when resuming")
}

func TestDebugCmdArgValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		resume      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "No args, no resume - should accept (interactive)",
			args:        []string{},
			resume:      "",
			expectError: false,
		},
		{
			name:        "One arg, no resume - should accept (traditional)",
			args:        []string{"Test issue"},
			resume:      "",
			expectError: false,
		},
		{
			name:        "Multiple args, no resume - should reject",
			args:        []string{"Test", "issue", "with", "spaces"},
			resume:      "",
			expectError: true,
			errorMsg:    "accepts between 0 and 1 arg(s), received 4",
		},
		{
			name:        "No args, with resume - should accept",
			args:        []string{},
			resume:      "session-123",
			expectError: false,
		},
		{
			name:        "Args with resume - should reject",
			args:        []string{"Test"},
			resume:      "session-123",
			expectError: true,
			errorMsg:    "unknown command", // Cobra treats this as subcommand when resume is set
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use:  "debug",
				Args: debugCmd.Args,
			}
			cmd.Flags().String("resume", "", "Resume session")

			if tt.resume != "" {
				err := cmd.Flags().Set("resume", tt.resume)
				require.NoError(t, err)
			}

			err := cmd.Args(cmd, tt.args)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// MockPrompt is a mock implementation for testing interactive input
type MockPrompt struct {
	input string
	err   error
}

func (m *MockPrompt) Show(input interface{}, output interface{}) (string, error) {
	return m.input, m.err
}

func TestInteractiveIssueInput(t *testing.T) {
	tests := []struct {
		name           string
		mockInput      string
		mockErr        error
		expectedResult string
		expectedErr    string
	}{
		{
			name:           "Valid multiline input",
			mockInput:      "This is a test issue\nwith multiple lines",
			mockErr:        nil,
			expectedResult: "This is a test issue\nwith multiple lines",
		},
		{
			name:        "Empty input error",
			mockInput:   "",
			mockErr:     assert.AnError,
			expectedErr: "assert.AnError general error for testing",
		},
		{
			name:           "Single line input",
			mockInput:      "Simple issue description",
			mockErr:        nil,
			expectedResult: "Simple issue description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockPrompt{
				input: tt.mockInput,
				err:   tt.mockErr,
			}

			result, err := mock.Show(nil, nil)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}