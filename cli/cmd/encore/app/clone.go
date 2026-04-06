package app

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
)

var cloneAppCmd = &cobra.Command{
	Use:   "clone [app-id] [directory]",
	Short: "Clone an Encore app to your computer",
	Args:  cobra.MinimumNArgs(1),

	DisableFlagsInUseLine: true,
	Run: func(c *cobra.Command, args []string) {
		cmdArgs := append([]string{"clone", "--origin", defaultGitRemoteName, defaultGitRemoteURL + args[0]}, args[1:]...)
		cmd := exec.Command("git", cmdArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			os.Exit(1)
		}
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return cmdutil.AutoCompleteAppSlug(cmd, args, toComplete)
		case 1:
			return nil, cobra.ShellCompDirectiveFilterDirs
		default:
			return nil, cobra.ShellCompDirectiveDefault
		}
	},
}

func init() {
	appCmd.AddCommand(cloneAppCmd)
}
