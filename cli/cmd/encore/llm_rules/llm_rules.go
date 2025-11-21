package llm_rules

import (
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/root"
)

var llmRulesCmd = &cobra.Command{
	Use:   "llm_rules",
	Short: "Commands to create llm rules for apps",
}

func init() {
	root.Cmd.AddCommand(llmRulesCmd)
}
