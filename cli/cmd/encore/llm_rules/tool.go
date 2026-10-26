package llm_rules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/tailscale/hujson"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/internal/version"
)

const mdcTemplate string = `---
description: Encore %s rules
globs:
alwaysApply: true
---
%s
`

type Tool string

// NOTE: changes to these values should also be reflected in userconfig
const (
	LLMRulesToolNone       Tool = ""
	LLMRulesToolCursor     Tool = "cursor"
	LLMRulesToolClaudeCode Tool = "claudecode"
	LLMRulesToolVSCode     Tool = "vscode"
	LLMRulesToolAgentsMD   Tool = "agentsmd"
	LLMRulesToolZed        Tool = "zed"
)

// all available options exept for None
var AllLLMRules = []Tool{
	LLMRulesToolCursor,
	LLMRulesToolClaudeCode,
	LLMRulesToolVSCode,
	LLMRulesToolAgentsMD,
	LLMRulesToolZed,
}

func LLMRulesFlagValues() []string {
	result := make([]string, 0, len(AllLLMRules))
	for _, r := range AllLLMRules {
		result = append(result, string(r))
	}
	return result
}

func (e Tool) Display() string {
	switch e {
	case LLMRulesToolCursor:
		return "Cursor"
	case LLMRulesToolClaudeCode:
		return "Claude Code"
	case LLMRulesToolVSCode:
		return "VS Code"
	case LLMRulesToolAgentsMD:
		return "AGENTS.md"
	case LLMRulesToolZed:
		return "Zed"
	default:
		return "None"
	}
}

func (e Tool) SelectPrompt() string {
	return "Select a tool to generate LLM rules for"
}

type ToolItem struct {
	tool Tool
}

func NewLLMRulesItem(tool Tool) ToolItem {
	return ToolItem{tool: tool}
}

func (i ToolItem) FilterValue() string { return i.tool.Display() }
func (i ToolItem) Title() string       { return i.FilterValue() }
func (i ToolItem) Description() string { return "" }
func (i ToolItem) SelectedID() Tool    { return i.tool }

type ToolSelectModel = cmdutil.SimpleSelectModel[Tool, ToolItem]
type ToolSelectDone = cmdutil.SimpleSelectDone[Tool]

func SetupLLMRules(llmRules Tool, lang cmdutil.Language, appRootRelpath string, appSlug string) error {
	if llmRules == LLMRulesToolNone {
		return nil
	}

	llmInstructions, err := downloadLLMInstructions(lang)
	if err != nil {
		return err
	}

	switch llmRules {
	case LLMRulesToolCursor:
		cursorDir := filepath.Join(appRootRelpath, ".cursor")
		rulesDir := filepath.Join(cursorDir, "rules")
		err := os.MkdirAll(rulesDir, 0755)
		if err != nil {
			return err
		}

		if appSlug != "" {
			// https://cursor.com/docs/context/mcp#using-mcpjson
			mcpPath := filepath.Join(cursorDir, "mcp.json")
			err = updateJsonFile(mcpPath, "mcpServers", func(mcpServers map[string]any) {
				mcpServers["encore-local"] = map[string]any{
					"command": "encore",
					"args":    []string{"mcp", "run", "--app=" + appSlug},
				}
			})
			if err != nil {
				return err
			}
		}

		// https://cursor.com/docs/context/rules
		// always overwrite as we have a dedicated encore config file
		err = os.WriteFile(filepath.Join(rulesDir, "encore.mdc"), fmt.Appendf(nil, mdcTemplate, lang, string(llmInstructions)), 0644)
		if err != nil {
			return err
		}
	case LLMRulesToolClaudeCode:
		if appSlug != "" {
			// https://code.claude.com/docs/en/mcp#project-scope
			mcpPath := filepath.Join(appRootRelpath, ".mcp.json")
			err = updateJsonFile(mcpPath, "mcpServers", func(mcpServers map[string]any) {
				mcpServers["encore-local"] = map[string]any{
					"command": "encore",
					"args":    []string{"mcp", "run", "--app=" + appSlug},
				}
			})
			if err != nil {
				return err
			}
		}

		// https://code.claude.com/docs/en/settings#key-points-about-the-configuration-system
		claudeDir := filepath.Join(appRootRelpath, ".claude")
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			return err
		}
		err = writeNewFileOrSkip(filepath.Join(claudeDir, "CLAUDE.md"), []byte(llmInstructions))
		if err != nil {
			return err
		}

	case LLMRulesToolVSCode:
		githubDir := filepath.Join(appRootRelpath, ".github")
		if err := os.MkdirAll(githubDir, 0755); err != nil {
			return err
		}

		// https://docs.github.com/en/copilot/how-tos/configure-custom-instructions/add-repository-instructions#writing-your-own-copilot-instructionsmd-file
		err = writeNewFileOrSkip(filepath.Join(githubDir, "copilot-instructions.md"), []byte(llmInstructions))
		if err != nil {
			return err
		}

		if appSlug != "" {
			vscodePath := filepath.Join(appRootRelpath, ".vscode")
			if err := os.MkdirAll(vscodePath, 0755); err != nil {
				return err
			}

			// https://code.visualstudio.com/docs/copilot/customization/mcp-servers#_configuration-format
			mcpPath := filepath.Join(vscodePath, "mcp.json")
			err = updateJsonFile(mcpPath, "servers", func(servers map[string]any) {
				servers["encore-local"] = map[string]any{
					"command": "encore",
					"args":    []string{"mcp", "run", "--app=" + appSlug},
				}
			})
			if err != nil {
				return err
			}
		}

	case LLMRulesToolAgentsMD:
		// https://agents.md/
		err = writeNewFileOrSkip(filepath.Join(appRootRelpath, "AGENTS.md"), []byte(llmInstructions))
		if err != nil {
			return err
		}
	case LLMRulesToolZed:
		// https://zed.dev/docs/ai/rules#rules-files
		rulesPath := filepath.Join(appRootRelpath, ".rules")
		err = writeNewFileOrSkip(rulesPath, []byte(llmInstructions))
		if err != nil {
			return err
		}

		if appSlug != "" {
			zedDir := filepath.Join(appRootRelpath, ".zed")
			err := os.MkdirAll(zedDir, 0755)
			if err != nil {
				return err
			}

			// https://zed.dev/docs/ai/mcp#as-custom-servers
			settingsPath := filepath.Join(zedDir, "settings.json")
			err = updateJsonFile(settingsPath, "context_servers", func(contextServers map[string]any) {
				contextServers["encore-local"] = map[string]any{
					"command": "encore",
					"args":    []string{"mcp", "run", "--app=" + appSlug},
					"env":     map[string]any{},
					"source":  "custom",
				}
			})
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func PrintLLMRulesInfo(tool Tool) {
	if tool == LLMRulesToolNone {
		return
	}

	cyan := color.New(color.FgCyan)
	cyanf := cyan.SprintfFunc()

	switch tool {
	case LLMRulesToolCursor, LLMRulesToolClaudeCode, LLMRulesToolVSCode, LLMRulesToolZed:
		fmt.Printf("MCP:      %s\n", cyanf("Configured in %s", tool.Display()))
		fmt.Println()
	}

	fmt.Printf("Try these prompts in %s:\n", tool.Display())
	fmt.Println("→ \"add image uploads to my hello world app\"")
	fmt.Println("→ \"add a SQL database for storing user profiles\"")
	fmt.Println("→ \"add a pub/sub topic for sending notifications\"")
	fmt.Println()
}

func updateJsonFile(path, parent string, updateFn func(field map[string]any)) error {
	var conf map[string]any

	// Read existing config if it exists. Editor configs such as VS Code's mcp.json
	// and Zed's settings.json are JSONC, so standardize comments and trailing commas first.
	if existingData, err := os.ReadFile(path); err == nil {
		existingData, err = hujson.Standardize(existingData)
		if err != nil {
			return fmt.Errorf("failed to parse existing %s: %w", path, err)
		}
		if err := json.Unmarshal(existingData, &conf); err != nil {
			return fmt.Errorf("failed to parse existing %s: %w", path, err)
		}
	} else {
		conf = make(map[string]any)
	}

	// Get or create mcpServers
	mcpServers, ok := conf[parent].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
		conf[parent] = mcpServers
	}

	updateFn(mcpServers)

	// Write back the config
	data, err := json.MarshalIndent(conf, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", path, err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

// write to file if it doesnt exist, and emits a warning and skips writing if the file exist
func writeNewFileOrSkip(filePath string, data []byte) error {
	if _, err := os.Stat(filePath); err == nil {
		// File already exists, skip writing
		yellow := color.New(color.FgYellow)
		yellow.Printf("Warning: %s file already exists, skipping\n", filePath)
	} else {
		err = os.WriteFile(filePath, data, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadLLMInstructions(lang cmdutil.Language) (string, error) {
	var file string
	switch lang {
	case cmdutil.LanguageGo:
		file = "go_llm_instructions.txt"
	case cmdutil.LanguageTS:
		file = "ts_llm_instructions.txt"
	default:
		return "", fmt.Errorf("unsupported language")
	}
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Prefix = "Downloading LLM instructions..."
	s.Start()
	defer s.Stop()

	// Prefer the instructions matching this CLI release, so they only describe
	// commands and MCP tools this version has. Fall back to main for
	// non-release builds or if the tag is missing.
	refs := []string{"refs/heads/main"}
	if version.Channel == version.GA {
		refs = []string{"refs/tags/" + version.Version, "refs/heads/main"}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error
	for _, ref := range refs {
		url := "https://raw.githubusercontent.com/encoredev/encore/" + ref + "/" + file
		body, err := fetchLLMInstructions(client, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
	}
	s.FinalMSG = fmt.Sprintf("failed, skipping: %v\n", lastErr)
	return "", lastErr
}

func fetchLLMInstructions(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
