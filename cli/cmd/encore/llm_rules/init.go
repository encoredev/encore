package llm_rules

import (
	"os"
	"path/filepath"
	"strings"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/internal/userconfig"
	"encr.dev/pkg/appfile"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

var (
	llmRulesToolFlag = cmdutil.Oneof{
		Value:     "",
		Allowed:   LLMRulesFlagValues(),
		Flag:      "llm-rules",
		FlagShort: "r",
		Desc:      "Initialize the app with llm rules for a specific tool",
		TypeDesc:  "string",
	}
)

func init() {
	llmRules := &cobra.Command{
		Use:   "init",
		Short: "Initialize llm rules for this project",
		Args:  cobra.ExactArgs(0),

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {

			var tool Tool
			if llmRulesToolFlag.Value == "" {
				cfg, err := userconfig.Global().Get()
				if err != nil {
					cmdutil.Fatalf("Couldn't read user config: %s", err)
				}
				tool = Tool(cfg.LLMRules)
			} else {
				tool = Tool(llmRulesToolFlag.Value)
			}

			if err := initLLMRules(tool); err != nil {
				cmdutil.Fatal(err)
			}
		},
	}

	llmRulesCmd.AddCommand(llmRules)
	llmRulesToolFlag.AddFlag(llmRules)
}

func initLLMRules(tool Tool) error {
	if tool == "" {
		var llmRulesModel ToolSelectModel
		{
			ls := list.NewDefaultItemStyles()
			ls.SelectedTitle = ls.SelectedTitle.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
			ls.SelectedDesc = ls.SelectedDesc.Foreground(lipgloss.Color(cmdutil.CodeBlue)).BorderForeground(lipgloss.Color(cmdutil.CodeBlue))
			del := list.NewDefaultDelegate()
			del.Styles = ls
			del.ShowDescription = false
			del.SetSpacing(0)

			items := make([]list.Item, 0, len(AllLLMRules))
			for _, rule := range AllLLMRules {
				items = append(items, ToolItem{rule})
			}

			ll := list.New(items, del, 0, 0)
			ll.SetShowTitle(false)
			ll.SetShowHelp(false)
			ll.SetShowPagination(true)
			ll.SetShowFilter(false)
			ll.SetFilteringEnabled(false)
			ll.SetShowStatusBar(false)
			ll.DisableQuitKeybindings() // quit handled by toolSelectModel

			llmRulesModel = ToolSelectModel{
				List:       ll,
				Predefined: LLMRulesToolNone,
			}
			llmRulesModel.SetSize(0, 20)

		}
		t := toolSelectorModel{
			toolModel: llmRulesModel,
		}
		p := tea.NewProgram(t)

		result, err := p.Run()
		if err != nil {
			cmdutil.Fatal(err)
		}

		res := result.(toolSelectorModel)
		if res.aborted {
			os.Exit(1)
		}

		tool = res.toolModel.Selected()
	}

	// Determine the app root.
	root, _, err := cmdutil.MaybeAppRoot()
	if errors.Is(err, cmdutil.ErrNoEncoreApp) {
		root, err = os.Getwd()
	}
	if err != nil {
		cmdutil.Fatal(err)
	}

	// parse encore.app
	filePath := filepath.Join(root, "encore.app")
	encoreApp, err := appfile.ParseFile(filePath)
	if err != nil {
		cmdutil.Fatalf("couldn't parse encore.app: %s", err)
	}

	var lang cmdutil.Language
	switch encoreApp.Lang {
	case appfile.LangGo:
		lang = cmdutil.LanguageGo
	case appfile.LangTS:
		lang = cmdutil.LanguageTS
	}

	if err := SetupLLMRules(tool, lang, root, encoreApp.ID); err != nil {
		cmdutil.Fatal(err)
	}

	PrintLLMRulesInfo(tool)

	return nil
}

type toolSelectorModel struct {
	toolModel ToolSelectModel
	aborted   bool
}

func (t toolSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		c    tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.SetSize(msg.Width, msg.Height)
		return t, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			t.aborted = true
			return t, tea.Quit
		}

		t.toolModel, c = t.toolModel.Update(msg)
		cmds = append(cmds, c)
		return t, tea.Batch(cmds...)

	case ToolSelectDone:
		cmds = append(cmds, tea.Quit)
	}

	t.toolModel, c = t.toolModel.Update(msg)
	cmds = append(cmds, c)
	return t, tea.Batch(cmds...)
}

func (t toolSelectorModel) Init() tea.Cmd {
	return nil
}

func (t toolSelectorModel) View() string {
	var b strings.Builder
	b.WriteString(t.toolModel.View())
	return cmdutil.DocStyle.Render(b.String())
}

func (t *toolSelectorModel) SetSize(width, height int) {
	t.toolModel.SetSize(width, height)
}
