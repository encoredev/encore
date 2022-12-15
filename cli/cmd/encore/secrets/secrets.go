package secrets

import (
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/root"
)

var secretCmd = &cobra.Command{
	Use:     "secret",
	Short:   "Secret management commands",
	Aliases: []string{"secrets"},
}

func init() {
	root.Cmd.AddCommand(secretCmd)
}
