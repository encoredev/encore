package secrets

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/internal/version"
)

var archiveSecretCmd = &cobra.Command{
	Deprecated:            "Use the command 'encore secret delete <id>' to delete the secret group.\n",
	Use:                   "archive <id>",
	Short:                 "Archives a secret value",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doArchiveOrUnarchive(args[0], true)
	},
}

var unarchiveSecretCmd = &cobra.Command{
	Deprecated:            "use the command 'encore secret delete <id>' to delete the secret group.\n",
	Use:                   "unarchive <id>",
	Short:                 "Unarchives a secret value",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doArchiveOrUnarchive(args[0], false)
	},
}

func doArchiveOrUnarchive(groupID string, archive bool) {
	if !strings.HasPrefix(groupID, "secgrp") {
		cmdutil.Fatal("the id must begin with 'secgrp_'. Valid ids can be found with 'encore secret list <key>'.")
	}
	if version.Compare("1.2.521") < 0 {
		// older version
		fmt.Printf("Please update your encore CLI to a newer version")
	} else {
		// newer version
		// do nothing since we are providing the deprecated string from the cobra command
	}
}

func init() {
	secretCmd.AddCommand(archiveSecretCmd)
	secretCmd.AddCommand(unarchiveSecretCmd)
}
