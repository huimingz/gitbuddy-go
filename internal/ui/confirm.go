package ui

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/fatih/color"
)

// Confirm asks the user for a yes/no confirmation
// Default is no (returns false on empty input)
func Confirm(message string, input io.Reader, output io.Writer) (bool, error) {
	return ConfirmWithDefault(message, false, input, output)
}

// ConfirmWithDefault asks the user for a yes/no confirmation with a specified default
func ConfirmWithDefault(message string, defaultYes bool, input io.Reader, output io.Writer) (bool, error) {
	scanner := bufio.NewScanner(input)

	var prompt string
	if defaultYes {
		prompt = fmt.Sprintf("%s [Y/n]: ", message)
	} else {
		prompt = fmt.Sprintf("%s [y/N]: ", message)
	}

	for {
		_, err := fmt.Fprint(output, prompt)
		if err != nil {
			return false, err
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, err
			}
			return false, io.EOF
		}

		response := strings.TrimSpace(strings.ToLower(scanner.Text()))

		switch response {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			_, err := fmt.Fprintln(output, "Please enter 'y' or 'n'")
			if err != nil {
				return false, err
			}
			// Continue the loop to ask again
		}
	}
}

// ShowCommitMessage displays a formatted commit message
func ShowCommitMessage(message string, output io.Writer) error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)

	_, err := bold.Fprintln(output, "\n📝 Generated Commit Message:")
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "─────────────────────────────")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(output, message)
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "─────────────────────────────")
	return err
}

// PRDescriptionDisplayer is an interface for PR responses that can be displayed
type PRDescriptionDisplayer interface {
	GetTitle() string
	GetDescription() string
}

// ShowPRDescription displays a formatted PR description
func ShowPRDescription(pr PRDescriptionDisplayer, output io.Writer) error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	// Title section
	_, err := bold.Fprintln(output, "\n📋 Generated PR:")
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "═══════════════════════════════════════════════════════════════════════════════")
	if err != nil {
		return err
	}

	// PR Title
	_, err = green.Fprintf(output, "Title: ")
	if err != nil {
		return err
	}
	_, err = bold.Fprintln(output, pr.GetTitle())
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "───────────────────────────────────────────────────────────────────────────────")
	if err != nil {
		return err
	}

	// PR Description
	_, err = fmt.Fprintln(output, pr.GetDescription())
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "═══════════════════════════════════════════════════════════════════════════════")
	return err
}

// ReportDisplayer is an interface for report responses that can be displayed
type ReportDisplayer interface {
	GetTitle() string
	GetContent() string
}

// ShowReport displays a formatted development report
func ShowReport(report ReportDisplayer, output io.Writer) error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)

	// Header
	_, err := bold.Fprintln(output, "\n📊 Generated Development Report:")
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "════════════════════════════════════════════════════════════════════════════════")
	if err != nil {
		return err
	}

	// Report content
	_, err = fmt.Fprintln(output, report.GetContent())
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "════════════════════════════════════════════════════════════════════════════════")
	return err
}

// ReviewIssue represents a single code review issue for display
type ReviewIssue struct {
	Severity    string
	Category    string
	File        string
	Line        int
	Title       string
	Description string
	Suggestion  string
}

// ReviewResultDisplayer is an interface for review responses that can be displayed
type ReviewResultDisplayer interface {
	GetSummary() string
	GetIssueCount() int
	GetIssueAt(index int) ReviewIssue
}

// ShowReviewResult displays a formatted code review result
func ShowReviewResult(review interface{}, output io.Writer) error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	blue := color.New(color.FgBlue)
	green := color.New(color.FgGreen)
	dim := color.New(color.FgHiBlack)

	var issues []ReviewIssue
	var summary string

	// Get summary through interface
	if r, ok := review.(interface{ GetSummary() string }); ok {
		summary = r.GetSummary()
	}

	// Use reflection to get issues from agent.ReviewResponse
	// This works because agent.ReviewIssue has the same fields as ui.ReviewIssue
	rv := reflect.ValueOf(review)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Struct {
		issuesField := rv.FieldByName("Issues")
		if issuesField.IsValid() && issuesField.Kind() == reflect.Slice {
			for i := 0; i < issuesField.Len(); i++ {
				item := issuesField.Index(i)
				if item.Kind() == reflect.Struct {
					issue := ReviewIssue{
						Severity:    getStringField(item, "Severity"),
						Category:    getStringField(item, "Category"),
						File:        getStringField(item, "File"),
						Line:        getIntField(item, "Line"),
						Title:       getStringField(item, "Title"),
						Description: getStringField(item, "Description"),
						Suggestion:  getStringField(item, "Suggestion"),
					}
					issues = append(issues, issue)
				}
			}
		}
	}

	// Header
	_, err := bold.Fprintln(output, "\n🔍 Code Review Results:")
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "════════════════════════════════════════════════════════════════════════════════")
	if err != nil {
		return err
	}

	// Count issues by severity
	errorCount := 0
	warningCount := 0
	infoCount := 0

	for _, issue := range issues {
		switch issue.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}

	// Summary stats
	if len(issues) == 0 {
		_, err = green.Fprintln(output, "✅ No issues found!")
	} else {
		_, err = fmt.Fprintf(output, "Found %d issue(s): ", len(issues))
		if errorCount > 0 {
			_, _ = red.Fprintf(output, "%d error(s) ", errorCount)
		}
		if warningCount > 0 {
			_, _ = yellow.Fprintf(output, "%d warning(s) ", warningCount)
		}
		if infoCount > 0 {
			_, _ = blue.Fprintf(output, "%d suggestion(s)", infoCount)
		}
		_, err = fmt.Fprintln(output)
	}
	if err != nil {
		return err
	}

	_, err = cyan.Fprintln(output, "────────────────────────────────────────────────────────────────────────────────")
	if err != nil {
		return err
	}

	// Display issues grouped by severity
	severityOrder := []string{"error", "warning", "info"}
	severityEmoji := map[string]string{
		"error":   "🔴",
		"warning": "🟡",
		"info":    "🔵",
	}
	severityColor := map[string]*color.Color{
		"error":   red,
		"warning": yellow,
		"info":    blue,
	}

	for _, severity := range severityOrder {
		for i, issue := range issues {
			if issue.Severity != severity {
				continue
			}

			emoji := severityEmoji[issue.Severity]
			clr := severityColor[issue.Severity]

			// Issue header
			_, err = clr.Fprintf(output, "\n%s [%s] ", emoji, strings.ToUpper(issue.Severity))
			if err != nil {
				return err
			}
			_, err = bold.Fprintln(output, issue.Title)
			if err != nil {
				return err
			}

			// Location
			if issue.File != "" {
				location := issue.File
				if issue.Line > 0 {
					location = fmt.Sprintf("%s:%d", issue.File, issue.Line)
				}
				_, err = dim.Fprintf(output, "   📍 %s\n", location)
				if err != nil {
					return err
				}
			}

			// Category
			_, err = dim.Fprintf(output, "   🏷️  %s\n", issue.Category)
			if err != nil {
				return err
			}

			// Description
			_, err = fmt.Fprintf(output, "   %s\n", issue.Description)
			if err != nil {
				return err
			}

			// Suggestion
			if issue.Suggestion != "" {
				_, err = green.Fprintf(output, "   💡 %s\n", issue.Suggestion)
				if err != nil {
					return err
				}
			}

			// Add separator between issues (but not after the last one)
			if i < len(issues)-1 {
				_, err = dim.Fprintln(output, "   ─ ─ ─ ─ ─ ─ ─ ─ ─ ─")
				if err != nil {
					return err
				}
			}
		}
	}

	// Summary section
	if summary != "" {
		_, err = cyan.Fprintln(output, "\n────────────────────────────────────────────────────────────────────────────────")
		if err != nil {
			return err
		}

		_, err = bold.Fprintln(output, "📋 Summary:")
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(output, summary)
		if err != nil {
			return err
		}
	}

	_, err = cyan.Fprintln(output, "════════════════════════════════════════════════════════════════════════════════")
	return err
}

// getStringField gets a string field from a reflect.Value struct
func getStringField(v reflect.Value, name string) string {
	field := v.FieldByName(name)
	if field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}

// getIntField gets an int field from a reflect.Value struct
func getIntField(v reflect.Value, name string) int {
	field := v.FieldByName(name)
	if field.IsValid() && field.Kind() == reflect.Int {
		return int(field.Int())
	}
	return 0
}

// SelectOption presents a list of options to the user and returns the selected index
// Returns the 0-based index of the selected option
func SelectOption(message string, options []string, defaultIndex int, input io.Reader, output io.Writer) (int, error) {
	if len(options) == 0 {
		return -1, fmt.Errorf("no options provided")
	}

	if defaultIndex < 0 || defaultIndex >= len(options) {
		defaultIndex = 0
	}

	scanner := bufio.NewScanner(input)
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgHiBlack)

	for {
		// Print the message
		_, err := bold.Fprintln(output, message)
		if err != nil {
			return -1, err
		}

		// Print the options
		for i, option := range options {
			if i == defaultIndex {
				_, err = cyan.Fprintf(output, "  %d) %s [default]\n", i+1, option)
			} else {
				_, err = fmt.Fprintf(output, "  %d) %s\n", i+1, option)
			}
			if err != nil {
				return -1, err
			}
		}

		// Print the prompt
		_, err = dim.Fprintf(output, "Enter your choice (1-%d) [%d]: ", len(options), defaultIndex+1)
		if err != nil {
			return -1, err
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return -1, err
			}
			return -1, io.EOF
		}

		response := strings.TrimSpace(scanner.Text())

		// Handle empty input (use default)
		if response == "" {
			return defaultIndex, nil
		}

		// Parse the input
		var choice int
		_, err = fmt.Sscanf(response, "%d", &choice)
		if err != nil || choice < 1 || choice > len(options) {
			_, err = color.New(color.FgRed).Fprintf(output, "Invalid choice. Please enter a number between 1 and %d\n\n", len(options))
			if err != nil {
				return -1, err
			}
			continue
		}

		return choice - 1, nil
	}
}
