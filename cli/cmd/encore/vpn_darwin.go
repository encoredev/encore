package main

import (
	"log"

	"encr.dev/cli/internal/wgtunnel"
	"encr.dev/cli/internal/xos"
	"github.com/spf13/cobra"
)

func init() {
	runCmd := &cobra.Command{
		Use:    "__run",
		Short:  "Runs the WireGuard tunnel synchronously.",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			if admin, err := xos.IsAdminUser(); err == nil && !admin {
				log.Fatalf("fatal: must start VPN as root user (use 'sudo'?)")
			}
			if err := wgtunnel.Run(); err != nil {
				fatal(err)
			}
		},
	}

	vpnCmd.AddCommand(runCmd)
}
