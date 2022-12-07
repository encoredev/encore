package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	daemonpkg "encr.dev/cli/cmd/encore/daemon"
	"encr.dev/internal/env"
	daemonpb "encr.dev/proto/encore/daemon"
)

var daemonizeForeground bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Starts the encore daemon",
	Run: func(cc *cobra.Command, args []string) {
		if daemonizeForeground {
			daemonpkg.Main()
		} else {
			if err := cmdutil.StartDaemonInBackground(context.Background()); err != nil {
				fatal(err)
			}
			fmt.Fprintln(os.Stdout, "encore daemon is now running")
		}
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().BoolVarP(&daemonizeForeground, "foreground", "f", false, "Start the daemon in the foreground")
	daemonCmd.AddCommand(daemonEnvCmd)
}

func setupDaemon(ctx context.Context) daemonpb.DaemonClient {
	return cmdutil.ConnectDaemon(ctx)
}

var daemonEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Prints Encore environment information",
	Run: func(cc *cobra.Command, args []string) {
		envs := env.List()
		for _, e := range envs {
			fmt.Println(e)
		}
	},
}
