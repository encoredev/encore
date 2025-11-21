package cmdutil

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	CodeBlue       = "#6D89FF"
	CodePurple     = "#A36C8C"
	CodeGreen      = "#B3D77E"
	ValidationFail = "#CB1010"
)

var (
	InputStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: CodeBlue, Light: CodeBlue})
	DescStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: CodeGreen, Light: CodePurple})
	DocStyle     = lipgloss.NewStyle().Padding(0, 2, 0, 2)
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ValidationFail))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00C200"))
)

type SelectedID[T any] interface {
	SelectedID() T
}

type Selectable interface {
	comparable
	SelectPrompt() string
}

type SimpleSelectDone[T any] struct {
	Selected T
}

type SimpleSelectModel[T Selectable, S SelectedID[T]] struct {
	Predefined T
	List       list.Model
}

func (m SimpleSelectModel[T, S]) Selected() T {
	var empty T
	if m.Predefined != empty {
		return m.Predefined
	}
	sel := m.List.SelectedItem()
	if sel == nil {
		return empty
	}
	return sel.(S).SelectedID()
}

func (m SimpleSelectModel[T, I]) Update(msg tea.Msg) (SimpleSelectModel[T, I], tea.Cmd) {
	var c tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// Have we selected an item?
			if idx := m.List.Index(); idx >= 0 {
				return m, func() tea.Msg {
					return SimpleSelectDone[T]{
						Selected: m.Selected(),
					}
				}
			}
		}
	}

	m.List, c = m.List.Update(msg)
	return m, c
}

func (m *SimpleSelectModel[T, I]) SetSize(width, height int) {
	m.List.SetWidth(width)
	m.List.SetHeight(max(height-1, 0))
}

func (m SimpleSelectModel[T, I]) View() string {
	var b strings.Builder

	// Get the prompt from the type T
	var zero T
	prompt := zero.SelectPrompt()

	b.WriteString(InputStyle.Render(prompt))
	b.WriteString(DescStyle.Render(" [Use arrows to move]"))
	b.WriteString("\n")
	b.WriteString(m.List.View())

	return b.String()
}
