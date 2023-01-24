package app

import (
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/root"
)

// These can be overwritten using
// `go build -ldflags "-X main.defaultGitRemoteName=encore"`.
var (
	defaultGitRemoteName = "encore"
	defaultGitRemoteURL  = "encore://"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Commands to create and link Encore apps",
}

func init() {
	root.Cmd.AddCommand(appCmd)
}
