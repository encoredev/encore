package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"encr.dev/cli/cmd/encore/cmdutil"
)

const (
	hotPink  = lipgloss.Color("#FF06B7")
	darkGray = lipgloss.Color("#767676")
)

var (
	inputStyle = lipgloss.NewStyle().Foreground(hotPink)
	docStyle   = lipgloss.NewStyle().Margin(1, 2)
)

type item struct {
	title    string
	desc     string
	template string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	numInputs int // 1 or 2 depending on what is shown
	focused   int // 0 or 1

	showName bool
	name     textinput.Model

	showList     bool
	list         list.Model
	lastSelected int

	aborted bool
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.focused == (m.numInputs - 1) {
				return m, tea.Quit
			}
			m.nextInput()

		case tea.KeyCtrlC, tea.KeyEsc:
			m.aborted = true
			return m, tea.Quit
		case tea.KeyShiftTab, tea.KeyCtrlP:
			m.prevInput()
		case tea.KeyTab, tea.KeyCtrlN:
			m.nextInput()
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	}

	var cmd tea.Cmd
	if m.focused == 0 && m.showName {
		m.name, cmd = m.name.Update(msg)
	} else {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	var b strings.Builder
	if m.showName {
		b.WriteString(inputStyle.Width(30).Render("App Name"))
		b.WriteByte('\n')
		b.WriteString(m.name.View())
		if m.showList {
			b.WriteString("\n\n")
		}
	}
	if m.showList {
		b.WriteString(inputStyle.Width(30).Render("Template"))
		b.WriteByte('\n')
		b.WriteString(m.list.View())
	}
	b.WriteString("\n")
	return docStyle.Render(b.String())
}

// nextInput focuses the next input field
func (m *model) nextInput() {
	m.focused = (m.focused + 1) % m.numInputs
	m.updateFocus()
}

// prevInput focuses the previous input field
func (m *model) prevInput() {
	m.focused--
	// Wrap around
	if m.focused < 0 {
		m.focused = m.numInputs - 1
	}
	m.updateFocus()
}

func (m *model) updateFocus() {
	if !m.showName || !m.showList {
		// Nothing to do
		return
	}

	nameFocused := m.focused == 0 && m.showName
	if nameFocused {
		m.name.Focus()
		// Store the last focused index from the list and deselect it.
		m.lastSelected = m.list.Index()
		m.list.Select(-1)
	} else {
		m.name.Blur()
		if m.lastSelected == -1 {
			m.lastSelected = 0
		}
		m.list.Select(m.lastSelected)
	}
}

func selectTemplate(inputName, inputTemplate string) (appName, template string) {
	// If we have both name and template already, return them.
	if inputName != "" && inputTemplate != "" {
		return inputName, inputTemplate
	}

	items := []list.Item{
		item{
			title:    "Uptime Monitor",
			desc:     "Microservices, SQL Databases, Pub/Sub, Cron Jobs",
			template: "https://github.com/encoredev/example-app-uptime",
		},
		item{
			title: "URL Shortener",
			desc:  "REST API, SQL Database",
		},
		item{title: "Slack bot", desc: "Slack integration"},
		item{title: "Empty app", desc: "Start from scratch (experienced users only)"},
	}

	name := textinput.New()
	name.Focus()
	name.CharLimit = 20
	name.Width = 30
	name.Validate = incrementalValidateNameInput

	ll := list.New(items, list.NewDefaultDelegate(), 0, 14)
	ll.SetShowTitle(false)
	ll.SetShowHelp(false)
	ll.SetShowPagination(false)
	ll.SetShowFilter(false)
	ll.SetFilteringEnabled(false)
	ll.SetShowStatusBar(false)

	m := model{
		name:     name,
		list:     ll,
		showName: inputName == "",
		showList: inputTemplate == "",
	}
	if m.showName {
		m.numInputs++
	}
	if m.showList {
		m.numInputs++
	}

	// If we have a name, start the list without any selection.
	if m.showName {
		m.list.Select(-1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	result, err := p.Run()
	if err != nil {
		cmdutil.Fatal(err)
	}

	// Validate the result.
	res := result.(model)
	if res.aborted {
		os.Exit(1)
	}

	appName, template = inputName, inputTemplate

	if appName == "" {
		appName = res.name.Value()
	}

	if template == "" {
		sel, ok := res.list.SelectedItem().(item)
		if !ok {
			cmdutil.Fatal("no template selected")
		}
		template = sel.template
	}

	return appName, template
}

// incrementalValidateName is like validateName but only
// checks for valid/invalid characters. It can't check for
// whether the last character is a dash, since if we treat that
// as an error the user won't be able to enter dashes at all.
func incrementalValidateNameInput(name string) error {
	ln := len(name)
	if ln == 0 {
		return fmt.Errorf("name must not be empty")
	} else if ln > 50 {
		return fmt.Errorf("name too long (max 50 chars)")
	}

	for i, s := range name {
		// Outside of [a-z], [0-9] and != '-'?
		if !((s >= 'a' && s <= 'z') || (s >= '0' && s <= '9') || s == '-') {
			return fmt.Errorf("name must only contain lowercase letters, digits, or dashes")
		} else if s == '-' {
			if i == 0 {
				return fmt.Errorf("name cannot start with a dash")
			} else if name[i-1] == '-' {
				return fmt.Errorf("name cannot contain repeated dashes")
			}
		}
	}
	return nil
}
