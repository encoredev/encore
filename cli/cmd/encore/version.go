package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"

	"encr.dev/cli/internal/update"
	"encr.dev/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Reports the current version of the encore application",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			ver *update.LatestVersion
			err error
		)
		if version.Version != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ver, err = update.Check(ctx)
		}

		// NOTE: This output format is relied on by the Encore IntelliJ plugin.
		// Don't change this without considering its impact on that plugin.
		fmt.Fprintln(os.Stdout, "encore version", version.Version)

		if err != nil {
			fatalf("could not check for update: %v", err)
		} else if ver.IsNewer(version.Version) {
			if ver.ForceUpgrade {
				fmt.Println(aurora.Red("An urgent security update for Encore is available."))
				if ver.SecurityNotes != "" {
					fmt.Println(aurora.Sprintf(aurora.Yellow("%s"), ver.SecurityNotes))
				}

				versionUpdateCmd.Run(cmd, args)
			} else {
				if ver.SecurityUpdate {
					fmt.Println(aurora.Sprintf(aurora.Red("A security update is update available: %s -> %s\nUpdate with: encore version update"), version.Version, ver.Version()))

					if ver.SecurityNotes != "" {
						fmt.Println(aurora.Sprintf(aurora.Yellow("%s"), ver.SecurityNotes))
					}
				} else {
					fmt.Println(aurora.Sprintf(aurora.Yellow("Update available: %s -> %s\nUpdate with: encore version update"), version.Version, ver.Version()))
				}
			}
		}
	},
}

var versionUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Checks for an update of encore and, if one is available, runs the appropriate command to update it.",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if version.Version == "" || strings.HasPrefix(version.Version, "devel") {
			fatal("cannot update development build, first install Encore from https://encore.dev/docs/install")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ver, err := update.Check(ctx)
		if err != nil {
			fatalf("could not check for update: %v", err)
		}
		if ver.IsNewer(version.Version) {
			fmt.Printf("Upgrading Encore to %v...\n", ver.Version())

			if err := ver.DoUpgrade(os.Stdout, os.Stderr); err != nil {
				fatalf("could not update: %v", err)
				os.Exit(1)
			}
			os.Exit(0)
		} else {
			fmt.Println("Encore already up to date.")
		}
	},
}

func init() {
	versionCmd.AddCommand(versionUpdateCmd)
	rootCmd.AddCommand(versionCmd)
}
