package ui

import (
	"bufio"
	"fmt"
	"io"
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
