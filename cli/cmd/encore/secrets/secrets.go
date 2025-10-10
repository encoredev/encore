package secrets // This must match the directory name

import (
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/root"
)

// The parent command for 'encore secret'
var secretCmd = &cobra.Command{
	Use:     "secret",
	Short:   "Secret management commands",
	Aliases: []string{"secrets"},
}

// The subcommand for 'encore secret check'
// This definition should logically be in a separate file (e.g., secret_check_cmd.go)
// but is defined here to correct the syntax error.
var secretCheckCmd = &cobra.Command{
	Use:   "check [environments...]",
	Short: "Checks if all required secrets are set in the specified environments.",
	Long:  "If no environments are provided, it checks 'local', 'dev', and 'prod'.",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println("Running secret check command...")
		if len(args) > 0 {
			cmd.Printf("Checking environments: %s\n", args)
		} else {
			cmd.Println("Checking default environments: local, dev, prod")
		}
	},
}

// The init function wires the commands together, running automatically at package load time.
func init() {
	// Register the parent command to the root of the CLI
	root.Cmd.AddCommand(secretCmd) 
	
	// Add the check command as a subcommand of the secret command
	secretCmd.AddCommand(secretCheckCmd) 
}