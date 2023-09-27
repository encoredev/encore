package k8s

import (
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/root"
)

var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Short:   "Kubernetes management commands",
	Aliases: []string{"k8s"},
}

func init() {
	root.Cmd.AddCommand(kubernetesCmd)
}
