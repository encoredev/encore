package secrets

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
)

var forceFlag bool

var deleteSecretCmd = &cobra.Command{
	Use:                   "delete <id>",
	Short:                 "Deletes a secret value",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if --yes / --force flag was passed to skip confirmation
		if !forceFlag {
			fmt.Printf("Are you sure you want to delete secret %q? [y/N]: ", args[0])
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}
		doDelete(args[0], true)
		return nil
	},
}

func doDelete(groupID string, delete bool) {
	if !strings.HasPrefix(groupID, "secgrp") {
		cmdutil.Fatal("the id must begin with 'secgrp_'. Valid ids can be found with 'encore secret list <key>'.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := platform.UpdateSecretGroup(ctx, platform.UpdateSecretGroupParams{
		ID:     groupID,
		Delete: &delete,
	})
	if err != nil {
		cmdutil.Fatal(err)
	}
	fmt.Printf("Successfully deleted secret group %s.\n", groupID)
}

func init() {
	deleteSecretCmd.Flags().BoolVarP(&forceFlag, "yes", "y", false, "Skip confirmation prompt")
	secretCmd.AddCommand(deleteSecretCmd)
}
