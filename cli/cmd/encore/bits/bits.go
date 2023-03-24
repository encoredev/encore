package bits

import (
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/root"
)

var bitsCmd = &cobra.Command{
	Use:   "bits",
	Short: "Commands to manage encore bits, reusable functionality for Encore applications",
}

func init() {
	root.Cmd.AddCommand(bitsCmd)
}
