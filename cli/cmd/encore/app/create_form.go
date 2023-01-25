package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tailscale/hujson"

	"encr.dev/cli/cmd/encore/cmdutil"
)

const (
	codeBlue   = "#6D89FF"
	codePurple = "#A36C8C"
	codeGreen  = "#B3D77E"
)

var (
	inputStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: codeBlue, Light: codeBlue})
	descStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: codeGreen, Light: codePurple})
	docStyle   = lipgloss.NewStyle().Margin(1, 2, 0, 2)
)

type item struct {
	ItemTitle string `json:"title"`
	Desc      string `json:"desc"`
	Template  string `json:"template"`
}

func (i item) Title() string       { return i.ItemTitle }
func (i item) Description() string { return i.Desc }
func (i item) FilterValue() string { return i.ItemTitle }

type model struct {
	numInputs int // 1 or 2 depending on what is shown
	focused   int // 0 or 1

	showName bool
	name     textinput.Model

	showList     bool
	list         list.Model
	lastSelected int

	loadingTemplates spinner.Model

	aborted bool
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		loadTemplates,
		m.loadingTemplates.Tick,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var c tea.Cmd
	var cmds []tea.Cmd

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
		m.list.SetHeight(msg.Height - 6)
		return m, nil

	case spinner.TickMsg:
		m.loadingTemplates, c = m.loadingTemplates.Update(msg)
		cmds = append(cmds, c)

	case loadedTemplates:
		var listItems []list.Item
		for _, it := range msg {
			listItems = append(listItems, it)
		}
		m.list.SetItems(listItems)
		m.list, c = m.list.Update(msg)
		cmds = append(cmds, c)
	}

	if m.focused == 0 && m.showName {
		m.name, c = m.name.Update(msg)
		cmds = append(cmds, c)
	} else {
		m.list, c = m.list.Update(msg)
		cmds = append(cmds, c)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var b strings.Builder
	if m.showName {
		b.WriteString(inputStyle.Render("App Name"))
		b.WriteString(descStyle.Render(" [Use only lowercase letters, digits, and dashes]"))
		b.WriteByte('\n')
		b.WriteString(m.name.View())
		if m.showList {
			b.WriteString("\n\n")
		}
	}
	if m.showList {
		if m.templatesLoading() {
			b.WriteString(inputStyle.Render(m.loadingTemplates.View() + " Loading templates..."))
		} else {
			b.WriteString(inputStyle.Render("Template"))
			b.WriteString(descStyle.Render(" [Use arrows to move]"))
			b.WriteByte('\n')
			b.WriteString(m.list.View())
		}
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

		// Are we loading templates still? If so revert to the name input if we have it.
		if m.showName && m.templatesLoading() {
			m.focused = 0
			m.updateFocus()
		}
	}
}

func (m model) templatesLoading() bool {
	return m.showList && len(m.list.Items()) == 0
}

func (m model) SelectedItem() (item, bool) {
	if !m.showList {
		return item{}, false
	}
	idx := m.list.Index()
	if idx < 0 {
		idx = m.lastSelected
	}
	if idx >= 0 {
		return m.list.Items()[idx].(item), true
	}
	return item{}, false
}

type loadedTemplates []item

func selectTemplate(inputName, inputTemplate string) (appName, template string) {
	// If we have both name and template already, return them.
	if inputName != "" && inputTemplate != "" {
		return inputName, inputTemplate
	}

	name := textinput.New()
	name.Focus()
	name.CharLimit = 20
	name.Width = 30
	name.Validate = incrementalValidateNameInput

	ls := list.NewDefaultItemStyles()
	ls.SelectedTitle = ls.SelectedTitle.Foreground(lipgloss.Color(codeBlue)).BorderForeground(lipgloss.Color(codeBlue))
	ls.SelectedDesc = ls.SelectedDesc.Foreground(lipgloss.Color(codeBlue)).BorderForeground(lipgloss.Color(codeBlue))
	del := list.NewDefaultDelegate()
	del.Styles = ls

	ll := list.New(nil, del, 0, 14)
	ll.SetShowTitle(false)
	ll.SetShowHelp(false)
	ll.SetShowPagination(true)
	ll.SetShowFilter(false)
	ll.SetFilteringEnabled(false)
	ll.SetShowStatusBar(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = inputStyle.Copy().Inline(true)

	m := model{
		name:             name,
		list:             ll,
		showName:         inputName == "",
		showList:         inputTemplate == "",
		loadingTemplates: sp,
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
		sel, ok := res.SelectedItem()
		if !ok {
			cmdutil.Fatal("no template selected")
		}
		template = sel.Template
	}

	return appName, template
}

func loadTemplates() tea.Msg {
	// Get the list of templates from GitHub
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := "https://raw.githubusercontent.com/encoredev/examples/main/cli-templates.json"
	if req, err := http.NewRequestWithContext(ctx, "GET", url, nil); err == nil {
		if resp, err := http.DefaultClient.Do(req); err == nil {
			if data, err := io.ReadAll(resp.Body); err == nil {
				if data, err = hujson.Standardize(data); err == nil {
					var items []item
					if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
						return loadedTemplates(items)
					}
				}
			}
		}
	}

	// Return a precompiled list of default items in case we can't read them from GitHub.
	return loadedTemplates([]item{
		{
			ItemTitle: "Hello World",
			Desc:      "A simple REST API",
			Template:  "hello-world",
		},
		{
			ItemTitle: "Uptime Monitor",
			Desc:      "Microservices, SQL Databases, Pub/Sub, Cron Jobs",
			Template:  "https://github.com/encoredev/example-app-uptime",
		},
		{
			ItemTitle: "GraphQL",
			Desc:      "GraphQL API, Microservices, SQL Database",
			Template:  "graphql",
		},
		{
			ItemTitle: "URL Shortener",
			Desc:      "REST API, SQL Database",
			Template:  "url-shortener",
		},
		{
			ItemTitle: "Empty app",
			Desc:      "Start from scratch (experienced users only)",
			Template:  "",
		},
	})
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
