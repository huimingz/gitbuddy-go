package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestSelectOption(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		options      []string
		defaultIndex int
		input        string
		want         int
		wantErr      bool
	}{
		{
			name:         "select first option",
			message:      "Choose an option:",
			options:      []string{"Option 1", "Option 2", "Option 3"},
			defaultIndex: 0,
			input:        "1\n",
			want:         0,
			wantErr:      false,
		},
		{
			name:         "select second option",
			message:      "Choose an option:",
			options:      []string{"Option 1", "Option 2", "Option 3"},
			defaultIndex: 0,
			input:        "2\n",
			want:         1,
			wantErr:      false,
		},
		{
			name:         "use default (empty input)",
			message:      "Choose an option:",
			options:      []string{"Option 1", "Option 2", "Option 3"},
			defaultIndex: 1,
			input:        "\n",
			want:         1,
			wantErr:      false,
		},
		{
			name:         "invalid then valid input",
			message:      "Choose an option:",
			options:      []string{"Option 1", "Option 2"},
			defaultIndex: 0,
			input:        "5\n2\n",
			want:         1,
			wantErr:      false,
		},
		{
			name:         "invalid text then valid input",
			message:      "Choose an option:",
			options:      []string{"Option 1", "Option 2"},
			defaultIndex: 0,
			input:        "abc\n1\n",
			want:         0,
			wantErr:      false,
		},
		{
			name:         "no options",
			message:      "Choose an option:",
			options:      []string{},
			defaultIndex: 0,
			input:        "1\n",
			want:         -1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			got, err := SelectOption(tt.message, tt.options, tt.defaultIndex, input, output)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("SelectOption() = %d, want %d", got, tt.want)
			}

			// Check that output contains the message
			outputStr := output.String()
			if !strings.Contains(outputStr, tt.message) {
				t.Errorf("output should contain message '%s'", tt.message)
			}

			// Check that output contains the options
			for _, option := range tt.options {
				if !strings.Contains(outputStr, option) {
					t.Errorf("output should contain option '%s'", option)
				}
			}
		})
	}
}

func TestSelectOption_DefaultIndexOutOfRange(t *testing.T) {
	input := strings.NewReader("\n")
	output := &bytes.Buffer{}

	options := []string{"Option 1", "Option 2"}

	// Test with negative default index
	got, err := SelectOption("Choose:", options, -1, input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected default to be adjusted to 0, got %d", got)
	}

	// Reset for next test
	input = strings.NewReader("\n")
	output = &bytes.Buffer{}

	// Test with out-of-range default index
	got, err = SelectOption("Choose:", options, 10, input, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected default to be adjusted to 0, got %d", got)
	}
}
