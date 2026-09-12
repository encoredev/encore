package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tailscale/hujson"
	"golang.org/x/term"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/llm_rules"
	"encr.dev/pkg/option"
)

type templateItem struct {
	ItemTitle string           `json:"title"`
	Desc      string           `json:"desc"`
	Template  string           `json:"template"`
	Lang      cmdutil.Language `json:"lang"`
}

func (i templateItem) Title() string       { return i.ItemTitle }
func (i templateItem) Description() string { return i.Desc }
func (i templateItem) FilterValue() string { return i.ItemTitle }

type CreateStep int

const (
	CreateStepLang CreateStep = iota
	CreateStepTemplate
	CreateStepAppName
	CreateStepLLMRules
)

type createFormModel struct {
	steps []CreateStep

	lang      langSelectModel
	templates templateListModel
	appName   appNameModel
	llmRules  llm_rules.ToolSelectModel

	initExistingApp bool

	width   int
	height  int
	aborted bool
}

func (m createFormModel) currentStep() option.Option[CreateStep] {
	if len(m.steps) == 0 {
		return option.None[CreateStep]()
	}
	return option.Some(m.steps[0])
}

func (m createFormModel) hasStep(s CreateStep) bool {
	return slices.Contains(m.steps, s)
}

func (m *createFormModel) removeStep(s CreateStep) {
	m.steps = slices.DeleteFunc(m.steps, func(step CreateStep) bool {
		return step == s
	})
}

func (m createFormModel) Init() tea.Cmd {
	return tea.Batch(
		m.appName.Init(),
		m.templates.Init(),
	)
}

const checkmark = "✔"

type appNameDone struct{}

type appNameModel struct {
	predefined string
	text       textinput.Model
	dirExists  bool
}

func (m appNameModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
	)
}

func (m appNameModel) Selected() string {
	if m.predefined != "" {
		return m.predefined
	}
	return m.text.Value()
}

func (m appNameModel) Update(msg tea.Msg) (appNameModel, tea.Cmd) {
	var cmds []tea.Cmd
	var c tea.Cmd
	m.text, c = m.text.Update(msg)
	cmds = append(cmds, c)

	if val := m.text.Value(); val != "" {
		_, err := os.Stat(val)
		m.dirExists = err == nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.text.Value() != "" && !m.dirExists {
				cmds = append(cmds, func() tea.Msg {
					return appNameDone{}
				})
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m appNameModel) View() string {
	var b strings.Builder
	if m.text.Focused() {
		b.WriteString(cmdutil.InputStyle.Render("App Name"))
		b.WriteString(cmdutil.DescStyle.Render(" [Use only lowercase letters, digits, and dashes]"))
		b.WriteByte('\n')
		b.WriteString(m.text.View())
		if m.dirExists {
			b.WriteString(cmdutil.ErrorStyle.Render(" error: dir already exists"))
		}
	} else {
		fmt.Fprintf(&b, "%s App Name: %s", checkmark, m.text.Value())
	}
	b.WriteByte('\n')
	return b.String()
}

type templateListModel struct {
	predefined string
	filter     cmdutil.Language

	all     []templateItem
	list    list.Model
	loading spinner.Model
}

func (m templateListModel) Init() tea.Cmd {
	return tea.Batch(
		loadTemplates,
		m.loading.Tick,
	)
}

func (m *templateListModel) SetSize(width, height int) {
	m.list.SetWidth(width)
	m.list.SetHeight(max(height-1, 0))
}

type templateSelectDone struct{}

func (m templateListModel) Update(msg tea.Msg) (templateListModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// Have we selected a template?
			if idx := m.list.Index(); idx >= 0 {
				return m, func() tea.Msg { return templateSelectDone{} }
			}
		}

	case spinner.TickMsg:
		m.loading, _ = m.loading.Update(msg)

	case loadedTemplates:
		m.all = msg
		m.refreshFilter()
		newList, c := m.list.Update(msg)
		m.list = newList
		cmds = append(cmds, c)
	}

	newList, c := m.list.Update(msg)
	m.list = newList
	cmds = append(cmds, c)

	return m, tea.Batch(cmds...)
}

func (m *templateListModel) UpdateFilter(lang cmdutil.Language) {
	m.filter = lang
	m.refreshFilter()
}

func (m *templateListModel) refreshFilter() {
	var listItems []list.Item
	for _, it := range m.all {
		if it.Lang == m.filter {
			listItems = append(listItems, it)
		}
	}
	m.list.SetItems(listItems)
}

func (m templateListModel) View() string {
	var b strings.Builder
	b.WriteString(cmdutil.InputStyle.Render("Template"))
	b.WriteString(cmdutil.DescStyle.Render(" [Use arrows to move]"))
	b.WriteByte('\n')
	b.WriteString(m.list.View())

	return b.String()
}

func (m templateListModel) Selected() string {
	if m.predefined != "" {
		return m.predefined
	}
	idx := m.list.Index()
	if idx < 0 {
		return ""
	}
	return m.list.Items()[idx].FilterValue()
}

func (m createFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		c    tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.aborted = true
			return m, tea.Quit
		case "q":
			// Only quit if no text input is focused
			if step, ok := m.currentStep().Get(); ok && step == CreateStepAppName {
				if m.appName.text.Focused() {
					break
				}
			}
			m.aborted = true
			return m, tea.Quit
		}

		if step, ok := m.currentStep().Get(); ok {
			switch step {
			case CreateStepLang:
				m.lang, c = m.lang.Update(msg)
				cmds = append(cmds, c)
			case CreateStepTemplate:
				m.templates, c = m.templates.Update(msg)
				cmds = append(cmds, c)
			case CreateStepAppName:
				m.appName, c = m.appName.Update(msg)
				cmds = append(cmds, c)
			case CreateStepLLMRules:
				m.llmRules, c = m.llmRules.Update(msg)
				cmds = append(cmds, c)
			}
		}
		return m, tea.Batch(cmds...)

	case langSelectDone:
		m.removeStep(CreateStepLang)
		m.templates.UpdateFilter(msg.Selected)
		m.SetSize(m.width, m.height)

	case llm_rules.ToolSelectDone:
		m.removeStep(CreateStepLLMRules)
		m.SetSize(m.width, m.height)

	case templateSelectDone:
		m.removeStep(CreateStepTemplate)
		if m.appName.predefined != "" {
			m.removeStep(CreateStepAppName)
		}
		m.SetSize(m.width, m.height)

	case appNameDone:
		m.removeStep(CreateStepAppName)
		m.SetSize(m.width, m.height)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	}

	// No more steps, quit
	if !m.currentStep().Present() {
		cmds = append(cmds, tea.Quit)
	}

	// Update all submodels for other messages.
	m.lang, c = m.lang.Update(msg)
	cmds = append(cmds, c)
	m.templates, c = m.templates.Update(msg)
	cmds = append(cmds, c)
	m.llmRules, c = m.llmRules.Update(msg)
	cmds = append(cmds, c)
	m.appName, c = m.appName.Update(msg)
	cmds = append(cmds, c)

	return m, tea.Batch(cmds...)
}

func (m *createFormModel) SetSize(width, height int) {
	doneHeight := lipgloss.Height(m.doneView())
	availHeight := height - doneHeight

	// CreateStepLang
	m.lang.SetSize(width, availHeight)

	// CreateStepTemplate
	m.templates.SetSize(width, availHeight)

	// CreateStepLLMRules
	m.llmRules.SetSize(width, availHeight)
}

func (m createFormModel) doneView() string {
	var b strings.Builder

	renderDone := func(title, value string) {
		b.WriteString(cmdutil.SuccessStyle.Render(fmt.Sprintf("%s %s: ", checkmark, title)))
		b.WriteString(value)
		b.WriteByte('\n')
	}

	renderLangDone := func() {
		renderDone("Language", m.lang.Selected().Display())
	}

	renderNameDone := func() {
		renderDone("App Name", m.appName.Selected())
	}

	renderTemplateDone := func() {
		renderDone("Template", m.templates.Selected())
	}

	renderLLMRulesDone := func() {
		renderDone("LLM Rules", m.llmRules.Selected().Display())
	}

	if m.appName.predefined != "" {
		renderNameDone()
	}
	if m.templates.predefined == "" && !m.hasStep(CreateStepLang) {
		renderLangDone()
	}
	if !m.initExistingApp {
		if m.templates.predefined != "" || !m.hasStep(CreateStepTemplate) {
			renderTemplateDone()
		}
		if m.llmRules.Predefined != "" || !m.hasStep(CreateStepLLMRules) {
			if m.llmRules.Selected() != llm_rules.LLMRulesToolNone {
				renderLLMRulesDone()
			}
		}
	}
	if m.appName.predefined == "" && !m.hasStep(CreateStepAppName) {
		renderNameDone()
	}

	return b.String()
}

func (m createFormModel) View() string {
	var b strings.Builder

	doneView := m.doneView()

	b.WriteString(doneView)
	if doneView != "" {
		b.WriteByte('\n')
	}

	if step, ok := m.currentStep().Get(); ok {
		if step == CreateStepLang {
			b.WriteString(m.lang.View())
		}

		if step == CreateStepTemplate {
			b.WriteString(m.templates.View())
		}

		if step == CreateStepAppName {
			b.WriteString(m.appName.View())
		}

		if step == CreateStepLLMRules {
			b.WriteString(m.llmRules.View())
		}
	}

	return cmdutil.DocStyle.Render(b.String())
}

func (m templateListModel) SelectedItem() (templateItem, bool) {
	if m.predefined != "" {
		return templateItem{}, false
	}
	idx := m.list.Index()
	items := m.list.Items()
	if idx >= 0 && len(items) > idx {
		return items[idx].(templateItem), true
	}
	return templateItem{}, false
}

func createAppForm(inputName, inputTemplate string, inputLang cmdutil.Language, inputLLMRules llm_rules.Tool, initExistingApp bool) (appName, template string, selectedLang cmdutil.Language, selectedRules llm_rules.Tool) {
	// If all is set, just return
	if inputName != "" && inputTemplate != "" && inputLLMRules != "" {
		return inputName, inputTemplate, inputLang, inputLLMRules
	}

	// If shell is non-interactive, don't prompt
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		if inputName == "" {
			cmdutil.Fatal("specify an app name")
		}
		return inputName, inputTemplate, inputLang, inputLLMRules
	}

	var langModel langSelectModel
	{
		ls := list.NewDefaultItemStyles()
		ls.SelectedTitle = ls.SelectedTitle.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
		ls.SelectedDesc = ls.SelectedDesc.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
		del := list.NewDefaultDelegate()
		del.Styles = ls
		del.ShowDescription = false
		del.SetSpacing(0)

		items := []list.Item{
			langItem{
				lang: cmdutil.LanguageGo,
				desc: "Build performant and scalable backends with Go",
			},
			langItem{
				lang: cmdutil.LanguageTS,
				desc: "Build backend and full-stack applications with TypeScript",
			},
		}

		ll := list.New(items, del, 0, 0)
		ll.SetShowTitle(false)
		ll.SetShowHelp(false)
		ll.SetShowPagination(true)
		ll.SetShowFilter(false)
		ll.SetFilteringEnabled(false)
		ll.SetShowStatusBar(false)
		ll.DisableQuitKeybindings() // quit handled by createFormModel
		langModel = langSelectModel{
			List:       ll,
			Predefined: inputLang,
		}
		langModel.SetSize(0, 20)
	}

	var templateModel templateListModel
	{
		ls := list.NewDefaultItemStyles()
		ls.SelectedTitle = ls.SelectedTitle.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
		ls.SelectedDesc = ls.SelectedDesc.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
		del := list.NewDefaultDelegate()
		del.Styles = ls

		ll := list.New(nil, del, 0, 20)
		ll.SetShowTitle(false)
		ll.SetShowHelp(false)
		ll.SetShowPagination(true)
		ll.SetShowFilter(false)
		ll.SetFilteringEnabled(false)
		ll.SetShowStatusBar(false)
		ll.DisableQuitKeybindings() // quit handled by createFormModel

		sp := spinner.New()
		sp.Spinner = spinner.Dot
		sp.Style = cmdutil.InputStyle.Copy().Inline(true)
		templateModel = templateListModel{
			predefined: inputTemplate,
			list:       ll,
			loading:    sp,
		}
	}
	var llmRulesModel llm_rules.ToolSelectModel
	{
		ls := list.NewDefaultItemStyles()
		ls.SelectedTitle = ls.SelectedTitle.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
		ls.SelectedDesc = ls.SelectedDesc.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
		del := list.NewDefaultDelegate()
		del.Styles = ls
		del.ShowDescription = false
		del.SetSpacing(0)

		items := make([]list.Item, 0, len(llm_rules.AllLLMRules)+1)
		items = append(items, llm_rules.NewLLMRulesItem(llm_rules.LLMRulesToolNone))
		for _, rule := range llm_rules.AllLLMRules {
			items = append(items, llm_rules.NewLLMRulesItem(rule))
		}

		ll := list.New(items, del, 0, 0)
		ll.SetShowTitle(false)
		ll.SetShowHelp(false)
		ll.SetShowPagination(true)
		ll.SetShowFilter(false)
		ll.SetFilteringEnabled(false)
		ll.SetShowStatusBar(false)
		ll.DisableQuitKeybindings() // quit handled by createFormModel

		llmRulesModel = llm_rules.ToolSelectModel{
			List:       ll,
			Predefined: inputLLMRules,
		}
		llmRulesModel.SetSize(0, 20)

	}

	var nameModel appNameModel
	{
		text := textinput.New()
		text.Focus()
		text.CharLimit = 50
		text.Width = 60
		text.Validate = incrementalValidateNameInput

		nameModel = appNameModel{predefined: inputName, text: text}
	}

	// Setup what steps and in what order they should be presented
	var steps []CreateStep
	if initExistingApp {
		if langModel.Predefined == "" {
			steps = append(steps, CreateStepLang)
		}
	} else {
		if templateModel.predefined == "" {
			if langModel.Predefined == "" {
				steps = append(steps, CreateStepLang)
			} else {
				templateModel.UpdateFilter(inputLang)
			}
			steps = append(steps, CreateStepTemplate)
		}
		if llmRulesModel.Predefined == "" {
			steps = append(steps, CreateStepLLMRules)
		}
	}
	if nameModel.predefined == "" {
		steps = append(steps, CreateStepAppName)
	}

	m := createFormModel{
		steps:           steps,
		lang:            langModel,
		templates:       templateModel,
		llmRules:        llmRulesModel,
		appName:         nameModel,
		initExistingApp: initExistingApp,
	}

	// If we have a name, start the list without any selection.
	if m.appName.predefined != "" {
		m.templates.list.Select(-1)
	}

	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		cmdutil.Fatal(err)
	}

	// Validate the result.
	res := result.(createFormModel)
	if res.aborted {
		os.Exit(1)
	}

	appName, template = inputName, inputTemplate

	if appName == "" {
		appName = res.appName.text.Value()
	}

	if template == "" && !initExistingApp {
		sel, ok := res.templates.SelectedItem()
		if !ok {
			cmdutil.Fatal("no template selected")
		}
		template = sel.Template
	}

	return appName, template, res.lang.Selected(), res.llmRules.Selected()
}

type langItem struct {
	lang cmdutil.Language
	desc string
}

func (i langItem) FilterValue() string          { return i.lang.Display() }
func (i langItem) Title() string                { return i.FilterValue() }
func (i langItem) Description() string          { return "" }
func (i langItem) SelectedID() cmdutil.Language { return i.lang }

type langSelectModel = cmdutil.SimpleSelectModel[cmdutil.Language, langItem]
type langSelectDone = cmdutil.SimpleSelectDone[cmdutil.Language]

type loadedTemplates []templateItem

var defaultTutorials = []templateItem{
	{
		ItemTitle: "Intro to Encore.ts",
		Desc:      "An interactive tutorial",
		Template:  "ts/introduction",
		Lang:      "ts",
	},
}

var defaultTemplates = []templateItem{
	{
		ItemTitle: "Hello World",
		Desc:      "A simple REST API",
		Template:  "hello-world",
		Lang:      "go",
	},
	{
		ItemTitle: "Hello World",
		Desc:      "A simple REST API",
		Template:  "ts/hello-world",
		Lang:      "ts",
	},
	{
		ItemTitle: "Uptime Monitor",
		Desc:      "Microservices, SQL Databases, Pub/Sub, Cron Jobs",
		Template:  "uptime",
		Lang:      "go",
	},
	{
		ItemTitle: "Uptime Monitor",
		Desc:      "Microservices, SQL Databases, Pub/Sub, Cron Jobs",
		Template:  "ts/uptime",
		Lang:      "ts",
	},
	{
		ItemTitle: "GraphQL",
		Desc:      "GraphQL API, Microservices, SQL Database",
		Template:  "graphql",
		Lang:      "go",
	},
	{
		ItemTitle: "URL Shortener",
		Desc:      "REST API, SQL Database",
		Template:  "url-shortener",
		Lang:      "go",
	},
	{
		ItemTitle: "URL Shortener",
		Desc:      "REST API, SQL Database",
		Template:  "ts/url-shortener",
		Lang:      "ts",
	},
	{
		ItemTitle: "SaaS Starter",
		Desc:      "Complete app with Clerk auth, Stripe billing, etc. (advanced)",
		Template:  "ts/saas-starter",
		Lang:      "ts",
	},
	{
		ItemTitle: "Empty app",
		Desc:      "Start from scratch (experienced users only)",
		Template:  "",
		Lang:      "go",
	},
	{
		ItemTitle: "Empty app",
		Desc:      "Start from scratch (experienced users only)",
		Template:  "ts/empty",
		Lang:      "ts",
	},
}

func fetchTemplates(url string, defaults []templateItem) []templateItem {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if req, err := http.NewRequestWithContext(ctx, "GET", url, nil); err == nil {
		if resp, err := http.DefaultClient.Do(req); err == nil {
			if data, err := io.ReadAll(resp.Body); err == nil {
				data, err = hujson.Standardize(data)
				if err == nil {
					var items []templateItem
					if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
						return items
					}
				}
			}
		}
	}
	return defaults
}

func loadTemplates() tea.Msg {
	var wg sync.WaitGroup
	var templates, tutorials []templateItem
	wg.Add(1)
	go func() {
		defer wg.Done()
		templates = fetchTemplates("https://raw.githubusercontent.com/encoredev/examples/main/cli-templates.json", defaultTemplates)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		tutorials = fetchTemplates("https://raw.githubusercontent.com/encoredev/examples/main/cli-tutorials.json", defaultTutorials)
	}()
	wg.Wait()
	return loadedTemplates(append(tutorials, templates...))
}

// incrementalValidateNameInput is like validateName but only
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
