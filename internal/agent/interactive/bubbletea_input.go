package interactive

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// BubbleteaInput provides an alternative input method using Bubbletea for better Unicode support
type BubbleteaInput struct {
	session *InteractiveSession
	ctx     context.Context
}

// inputModel represents the Bubbletea model for interactive input
type inputModel struct {
	textInput textinput.Model
	session   *InteractiveSession
	ctx       context.Context
	output    strings.Builder
	quitting  bool
	err       error
}

type inputMsg struct {
	input string
}

type errorMsg struct {
	err error
}

// NewBubbleteaInput creates a new BubbleteaInput instance
func NewBubbleteaInput(session *InteractiveSession, ctx context.Context) *BubbleteaInput {
	return &BubbleteaInput{
		session: session,
		ctx:     ctx,
	}
}

// Start begins the Bubbletea-based interactive session
func (b *BubbleteaInput) Start(output io.Writer) error {
	// Create the initial model
	ti := textinput.New()
	ti.Placeholder = "输入您的问题或命令..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 80

	// Configure text input for better Chinese character support
	ti.Prompt = "gitbuddy🤖> "

	m := inputModel{
		textInput: ti,
		session:   b.session,
		ctx:       b.ctx,
	}

	// Display welcome message
	b.session.displayWelcome(output)

	// Start the Bubbletea program
	p := tea.NewProgram(&m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Output any final content
	final := finalModel.(*inputModel)
	if final.output.Len() > 0 {
		_, err = output.Write([]byte(final.output.String()))
	}

	return final.err
}

// Init implements the Bubbletea Model interface
func (m *inputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements the Bubbletea Model interface
func (m *inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			m.output.WriteString("\nGoodbye! Thank you for using GitBuddy interactive mode.\n")
			return m, tea.Quit

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textInput.Value())

			// Check for exit commands
			if input == "exit" || input == "quit" || input == "q" {
				m.quitting = true
				m.output.WriteString("\nGoodbye! Thank you for using GitBuddy interactive mode.\n")
				return m, tea.Quit
			}

			// Process the command if not empty
			if input != "" {
				// Process command using the session
				var output strings.Builder
				err := m.session.ProcessCommand(m.ctx, input, &output)
				if err != nil {
					m.output.WriteString(fmt.Sprintf("Error: %v\n", err))
				} else {
					m.output.WriteString(output.String())
				}
				m.output.WriteString("\n")
			}

			// Clear the input
			m.textInput.SetValue("")

		case tea.KeyCtrlD:
			// Handle Ctrl+D as exit
			m.quitting = true
			m.output.WriteString("\nGoodbye! Thank you for using GitBuddy interactive mode.\n")
			return m, tea.Quit
		}

	case errorMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit

	// Check for context cancellation
	case tea.WindowSizeMsg:
		// Adjust input width based on terminal size
		if msg.Width > 10 {
			m.textInput.Width = msg.Width - 10
		}
	}

	// Update the text input
	m.textInput, cmd = m.textInput.Update(msg)

	// Check for context cancellation
	select {
	case <-m.ctx.Done():
		m.quitting = true
		return m, tea.Quit
	default:
	}

	return m, cmd
}

// View implements the Bubbletea Model interface
func (m *inputModel) View() string {
	if m.quitting {
		return ""
	}

	// Create the view with better styling
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2)

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		Render("GitBuddy Interactive Debug Session")

	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("输入问题或命令，按 Enter 提交，Ctrl+C 或 'exit' 退出")

	// Show recent output if any
	output := ""
	if m.output.Len() > 0 {
		output = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Render(m.output.String())
	}

	content := fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n",
		header,
		instructions,
		output,
		m.textInput.View())

	return style.Render(content)
}

// startWithBubbletea provides a Bubbletea-based alternative for better Unicode support
func (s *InteractiveSession) startWithBubbletea(ctx context.Context) error {
	bubbleInput := NewBubbleteaInput(s, ctx)
	return bubbleInput.Start(&strings.Builder{}) // For now, use a builder - could be enhanced
}