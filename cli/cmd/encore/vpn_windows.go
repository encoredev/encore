package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"encr.dev/cli/internal/winsvc"
)

func init() {
	installCmd := &cobra.Command{
		Hidden: true,
		Use:    "svc-install",
		Short:  "Installs the windows service for the WireGuard tunnel",
		Args:   cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := winsvc.Install(args[0]); err != nil {
				fatal(err)
			}
		},
	}
	vpnCmd.AddCommand(installCmd)

	uninstallCmd := &cobra.Command{
		Hidden: true,
		Use:    "svc-uninstall",
		Short:  "Uninstalls the windows service",
		Args:   cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := winsvc.Uninstall(args[0]); err != nil {
				fatal(err)
			}
		},
	}
	vpnCmd.AddCommand(uninstallCmd)

	statusCmd := &cobra.Command{
		Hidden: true,
		Use:    "svc-status",
		Short:  "Uninstalls the windows service",
		Args:   cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			installed, err := winsvc.Status(args[0])
			if err != nil {
				fatal(err)
			}
			if installed {
				fmt.Fprintln(os.Stdout, "installed")
			} else {
				fmt.Fprintln(os.Stdout, "not installed")
			}
		},
	}
	vpnCmd.AddCommand(statusCmd)

	runCmd := &cobra.Command{
		Hidden: true,
		Use:    "svc-run",
		Short:  "Runs the windows service",
		Args:   cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := winsvc.Run(args[0]); err != nil {
				fatal(err)
			}
		},
	}
	vpnCmd.AddCommand(runCmd)
}
