package secrets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/cli/internal/platform"
)

var archiveSecretCmd = &cobra.Command{
	Use:                   "archive <id>",
	Short:                 "Archives a secret value",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doArchiveOrUnarchive(args[0], true)
	},
}

var unarchiveSecretCmd = &cobra.Command{
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := platform.UpdateSecretGroup(ctx, platform.UpdateSecretGroupParams{
		ID:       groupID,
		Archived: &archive,
	})
	if err != nil {
		cmdutil.Fatal(err)
	}
	if archive {
		fmt.Printf("Successfully archived secret group %s.\n", groupID)
	} else {
		fmt.Printf("Successfully unarchived secret group %s.\n", groupID)
	}
}

func init() {
	root.Cmd.AddCommand(secretCmd)
	secretCmd.AddCommand(archiveSecretCmd)
	secretCmd.AddCommand(unarchiveSecretCmd)
}
