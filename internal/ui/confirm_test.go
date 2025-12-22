package ui

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirm_Yes(t *testing.T) {
	input := strings.NewReader("y\n")
	output := &bytes.Buffer{}

	result, err := Confirm("Do you want to continue?", input, output)
	require.NoError(t, err)
	assert.True(t, result)
	assert.Contains(t, output.String(), "Do you want to continue?")
}

func TestConfirm_No(t *testing.T) {
	input := strings.NewReader("n\n")
	output := &bytes.Buffer{}

	result, err := Confirm("Do you want to continue?", input, output)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestConfirm_YesUpperCase(t *testing.T) {
	input := strings.NewReader("Y\n")
	output := &bytes.Buffer{}

	result, err := Confirm("Proceed?", input, output)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestConfirm_YesFull(t *testing.T) {
	input := strings.NewReader("yes\n")
	output := &bytes.Buffer{}

	result, err := Confirm("Proceed?", input, output)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestConfirm_NoFull(t *testing.T) {
	input := strings.NewReader("no\n")
	output := &bytes.Buffer{}

	result, err := Confirm("Proceed?", input, output)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestConfirm_EmptyDefaultsToNo(t *testing.T) {
	input := strings.NewReader("\n")
	output := &bytes.Buffer{}

	result, err := Confirm("Proceed?", input, output)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestConfirm_InvalidThenYes(t *testing.T) {
	// User enters invalid input first, then valid input
	input := strings.NewReader("invalid\ny\n")
	output := &bytes.Buffer{}

	result, err := Confirm("Proceed?", input, output)
	require.NoError(t, err)
	assert.True(t, result)
	// Should show the prompt again after invalid input
	assert.Contains(t, output.String(), "y/N")
}

func TestConfirm_EOF(t *testing.T) {
	input := strings.NewReader("") // EOF immediately
	output := &bytes.Buffer{}

	result, err := Confirm("Proceed?", input, output)
	assert.Equal(t, io.EOF, err)
	assert.False(t, result)
}

func TestConfirmWithDefault_YesDefault(t *testing.T) {
	input := strings.NewReader("\n")
	output := &bytes.Buffer{}

	result, err := ConfirmWithDefault("Proceed?", true, input, output)
	require.NoError(t, err)
	assert.True(t, result)                     // Empty input uses default (true)
	assert.Contains(t, output.String(), "Y/n") // Shows Y is default
}

func TestConfirmWithDefault_NoDefault(t *testing.T) {
	input := strings.NewReader("\n")
	output := &bytes.Buffer{}

	result, err := ConfirmWithDefault("Proceed?", false, input, output)
	require.NoError(t, err)
	assert.False(t, result)                    // Empty input uses default (false)
	assert.Contains(t, output.String(), "y/N") // Shows N is default
}

func TestShowCommitMessage(t *testing.T) {
	output := &bytes.Buffer{}

	message := "feat(auth): add login\n\nImplement JWT authentication\n\nCloses #123"
	err := ShowCommitMessage(message, output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "feat(auth): add login")
	assert.Contains(t, outputStr, "JWT authentication")
}
