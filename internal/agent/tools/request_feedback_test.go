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
		wantResponse  string
		wantOptionIdx int
		wantIsChoice  bool
		wantErr       bool
		errMsg        string
	}{
		{
			name: "select first option",
			params: &RequestFeedbackParams{
				Title:   "调查路径选择",
				Content: "发现了多个问题",
				Prompt:  "请选择要调查的路径",
				Options: []string{"Path A", "Path B", "Path C"},
			},
			userInput:     "1\n",
			wantResponse:  "Path A",
			wantOptionIdx: 0,
			wantIsChoice:  true,
			wantErr:       false,
		},
		{
			name: "select second option",
			params: &RequestFeedbackParams{
				Title:   "调查路径选择",
				Content: "需要选择调查方向",
				Prompt:  "请选择",
				Options: []string{"Path A", "Path B"},
			},
			userInput:     "2\n",
			wantResponse:  "Path B",
			wantOptionIdx: 1,
			wantIsChoice:  true,
			wantErr:       false,
		},
		{
			name: "use default (empty input)",
			params: &RequestFeedbackParams{
				Title:   "方向选择",
				Content: "需要选择方向",
				Prompt:  "请选择方向",
				Options: []string{"Option 1", "Option 2"},
			},
			userInput:     "\n",
			wantResponse:  "Option 1",
			wantOptionIdx: 0,
			wantIsChoice:  true,
			wantErr:       false,
		},
		{
			name: "open-ended question with text input",
			params: &RequestFeedbackParams{
				Title:   "日志文件路径",
				Content: "需要查看日志文件",
				Prompt:  "请输入日志文件路径",
			},
			userInput:    "/var/log/app.log\n",
			wantResponse: "/var/log/app.log",
			wantIsChoice: false,
			wantErr:      false,
		},
		{
			name: "open-ended question with empty input",
			params: &RequestFeedbackParams{
				Title:   "错误描述",
				Content: "需要了解错误详情",
				Prompt:  "请描述错误",
			},
			userInput:    "\n",
			wantResponse: "",
			wantIsChoice: false,
			wantErr:      false,
		},
		{
			name:      "nil params",
			params:    nil,
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "params is required",
		},
		{
			name: "empty title",
			params: &RequestFeedbackParams{
				Title:   "",
				Content: "Some content",
				Prompt:  "Please answer",
				Options: []string{"Option 1", "Option 2"},
			},
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "title is required",
		},
		{
			name: "empty content",
			params: &RequestFeedbackParams{
				Title:   "Title",
				Content: "",
				Prompt:  "Please answer",
				Options: []string{"Option 1", "Option 2"},
			},
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "content is required",
		},
		{
			name: "empty prompt",
			params: &RequestFeedbackParams{
				Title:   "Title",
				Content: "Content",
				Prompt:  "",
				Options: []string{"Option 1", "Option 2"},
			},
			userInput: "1\n",
			wantErr:   true,
			errMsg:    "prompt is required",
		},
		{
			name: "insufficient options for choice question",
			params: &RequestFeedbackParams{
				Title:   "Choose",
				Content: "Need to choose",
				Prompt:  "Select one",
				Options: []string{"Only one"},
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

			// Check user response
			userResponse, ok := response["user_response"].(string)
			if !ok {
				t.Fatal("response should contain 'user_response' as string")
			}

			if userResponse != tt.wantResponse {
				t.Errorf("expected user_response '%s', got '%s'", tt.wantResponse, userResponse)
			}

			// Check is_choice flag
			isChoice, ok := response["is_choice"].(bool)
			if !ok {
				t.Fatal("response should contain 'is_choice' as bool")
			}

			if isChoice != tt.wantIsChoice {
				t.Errorf("expected is_choice %v, got %v", tt.wantIsChoice, isChoice)
			}

			// Check selected index for choice questions
			if tt.wantIsChoice {
				selectedIndex, ok := response["selected_index"].(float64)
				if !ok {
					t.Fatal("response should contain 'selected_index' as number for choice questions")
				}

				if int(selectedIndex) != tt.wantOptionIdx {
					t.Errorf("expected selected_index %d, got %d", tt.wantOptionIdx, int(selectedIndex))
				}
			}

			// Check output contains title
			outputStr := output.String()
			if !strings.Contains(outputStr, tt.params.Title) {
				t.Error("output should contain the title")
			}

			// Check output contains content
			if !strings.Contains(outputStr, tt.params.Content) {
				t.Error("output should contain the content")
			}

			// Check output contains prompt
			if !strings.Contains(outputStr, tt.params.Prompt) {
				t.Error("output should contain the prompt")
			}

			// Check output contains options for choice questions
			if len(tt.params.Options) > 0 {
				for _, option := range tt.params.Options {
					if !strings.Contains(outputStr, option) {
						t.Errorf("output should contain option '%s'", option)
					}
				}
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
		Title:   "选择",
		Content: "需要做出选择",
		Prompt:  "请选择一个选项",
		Options: []string{"Option 1", "Option 2"},
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
