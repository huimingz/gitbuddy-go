package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRequestFeedbackTool_Name(t *testing.T) {
	tool := NewRequestFeedbackTool(nil, nil)
	if tool.Name() != "request_feedback" {
		t.Errorf("expected name 'request_feedback', got '%s'", tool.Name())
	}
}

func TestRequestFeedbackTool_Description(t *testing.T) {
	tool := NewRequestFeedbackTool(nil, nil)
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(desc, "feedback") && !strings.Contains(desc, "user") {
		t.Error("description should mention feedback and user interaction")
	}
}

func TestRequestFeedbackTool_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		params        *RequestFeedbackParams
		userInput     string
		wantOptionIdx int
		wantErr       bool
		errMsg        string
	}{
		{
			name: "select first option",
			params: &RequestFeedbackParams{
				Question: "Which path to investigate?",
				Options:  []string{"Path A", "Path B", "Path C"},
				Context:  "Found multiple issues",
			},
			userInput:     "1\n",
			wantOptionIdx: 0,
			wantErr:       false,
		},
		{
			name: "select second option",
			params: &RequestFeedbackParams{
				Question: "Which path to investigate?",
				Options:  []string{"Path A", "Path B"},
			},
			userInput:     "2\n",
			wantOptionIdx: 1,
			wantErr:       false,
		},
		{
			name: "use default (empty input)",
			params: &RequestFeedbackParams{
				Question: "Choose direction:",
				Options:  []string{"Option 1", "Option 2"},
			},
			userInput:     "\n",
			wantOptionIdx: 0,
			wantErr:       false,
		},
		{
			name:      "nil params",
			params:    nil,
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "params is required",
		},
		{
			name: "empty question",
			params: &RequestFeedbackParams{
				Question: "",
				Options:  []string{"Option 1", "Option 2"},
			},
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "question is required",
		},
		{
			name: "insufficient options",
			params: &RequestFeedbackParams{
				Question: "Choose:",
				Options:  []string{"Only one"},
			},
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "at least 2 options are required",
		},
		{
			name: "no options",
			params: &RequestFeedbackParams{
				Question: "Choose:",
				Options:  []string{},
			},
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "at least 2 options are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.userInput)
			output := &bytes.Buffer{}

			tool := NewRequestFeedbackTool(input, output)
			result, err := tool.Execute(ctx, tt.params)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Parse the JSON response
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				t.Fatalf("failed to parse response JSON: %v", err)
			}

			// Check selected option
			selectedOption, ok := response["selected_option"].(string)
			if !ok {
				t.Fatal("response should contain 'selected_option' as string")
			}

			expectedOption := tt.params.Options[tt.wantOptionIdx]
			if selectedOption != expectedOption {
				t.Errorf("expected selected_option '%s', got '%s'", expectedOption, selectedOption)
			}

			// Check selected index
			selectedIndex, ok := response["selected_index"].(float64)
			if !ok {
				t.Fatal("response should contain 'selected_index' as number")
			}

			if int(selectedIndex) != tt.wantOptionIdx {
				t.Errorf("expected selected_index %d, got %d", tt.wantOptionIdx, int(selectedIndex))
			}

			// Check output contains question
			outputStr := output.String()
			if !strings.Contains(outputStr, tt.params.Question) {
				t.Error("output should contain the question")
			}

			// Check output contains options
			for _, option := range tt.params.Options {
				if !strings.Contains(outputStr, option) {
					t.Errorf("output should contain option '%s'", option)
				}
			}

			// Check output contains context if provided
			if tt.params.Context != "" && !strings.Contains(outputStr, tt.params.Context) {
				t.Error("output should contain the context")
			}
		})
	}
}

func TestRequestFeedbackTool_Execute_InvalidInput(t *testing.T) {
	ctx := context.Background()

	// Test with invalid input followed by valid input
	input := strings.NewReader("invalid\n2\n")
	output := &bytes.Buffer{}

	tool := NewRequestFeedbackTool(input, output)
	params := &RequestFeedbackParams{
		Question: "Choose:",
		Options:  []string{"Option 1", "Option 2"},
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	// Should have selected option 2 (index 1)
	selectedIndex := int(response["selected_index"].(float64))
	if selectedIndex != 1 {
		t.Errorf("expected selected_index 1, got %d", selectedIndex)
	}

	// Output should contain error message about invalid input
	outputStr := output.String()
	if !strings.Contains(outputStr, "Invalid") {
		t.Error("output should contain error message about invalid input")
	}
}
