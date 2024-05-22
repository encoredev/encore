package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/logrusorgru/aurora/v3"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/cli/internal/telemetry"
	"encr.dev/pkg/fns"
	daemonpb "encr.dev/proto/encore/daemon"
)

var TelemetryDisabledByEnvVar = os.Getenv("DISABLE_ENCORE_TELEMETRY") == "1"
var TelemetryDebugByEnvVar = os.Getenv("ENCORE_TELEMETRY_DEBUG") == "1"

func printTelemetryStatus() {
	status := aurora.Green("Enabled").String()
	if !telemetry.IsEnabled() {
		status = aurora.Red("Disabled").String()
	}
	fmt.Println(aurora.Sprintf("%s\n", aurora.Bold("Encore Telemetry")))
	items := [][2]string{
		{"Status", status},
	}
	if root.Verbosity > 0 {
		items = append(items, [2]string{"Install ID", telemetry.GetAnonID()})
	}
	if telemetry.IsDebug() {
		items = append(items, [2]string{"Debug", aurora.Green("Enabled").String()})
	}
	maxKeyLen := fns.Max(items, func(entry [2]string) int { return len(entry[0]) })
	for _, item := range items {
		spacing := strings.Repeat(" ", maxKeyLen-len(item[0]))
		fmt.Printf("%s: %s%s\n", item[0], spacing, item[1])
	}
	fmt.Println(aurora.Sprintf("\nLearn more: %s", aurora.Underline("https://encore.dev/docs/telemetry")))
}

func updateTelemetry(ctx context.Context) {
	err := func() error {
		//
		daemon := cmdutil.ConnectDaemon(ctx, cmdutil.SkipStart)
		if daemon != nil {
			// Update the telemetry config on the daemon if it is running
			_, err := daemon.Telemetry(ctx, &daemonpb.TelemetryConfig{
				AnonId:  telemetry.GetAnonID(),
				Enabled: telemetry.IsEnabled(),
				Debug:   telemetry.IsDebug(),
			})
			return err
		} else {
			// Update the telemetry config locally if the daemon is not running
			return telemetry.SaveConfig()
		}
	}()
	if err != nil {
		log.Debug().Err(err).Msgf("could not update telemetry: %s", err)
	}
}

var telemetryCommand = &cobra.Command{
	Use:   "telemetry",
	Short: "Reports the current telemetry status",

	Run: func(cmd *cobra.Command, args []string) {
		printTelemetryStatus()
	},
}

var telemetryEnableCommand = &cobra.Command{
	Use:   "enable",
	Short: "Enables telemetry reporting",
	Run: func(cmd *cobra.Command, args []string) {
		if telemetry.SetEnabled(true) {
			updateTelemetry(cmd.Context())
		}
		printTelemetryStatus()
	},
}

var telemetryDisableCommand = &cobra.Command{
	Use:   "disable",
	Short: "Disables telemetry reporting",
	Run: func(cmd *cobra.Command, args []string) {
		if telemetry.SetEnabled(false) {
			updateTelemetry(cmd.Context())
		}
		printTelemetryStatus()
	},
}

func init() {
	telemetryCommand.AddCommand(telemetryEnableCommand, telemetryDisableCommand)
	rootCmd.AddCommand(telemetryCommand)
	root.AddPreRun(func(cmd *cobra.Command, args []string) {
		update := false
		if TelemetryDisabledByEnvVar {
			update = telemetry.SetEnabled(false)
		}
		if cmd.Use == "daemon" {
			return
		}
		update = update || telemetry.SetDebug(TelemetryDebugByEnvVar)
		if update {
			go updateTelemetry(cmd.Context())
		}
		if telemetry.ShouldShowWarning() && cmd.Use != "version" {
			fmt.Println()
			fmt.Println(aurora.Sprintf("%s: This CLI tool collects usage data to help us improve Encore.", aurora.Bold("Note")))
			fmt.Println(aurora.Sprintf("      You can disable this by running '%s'.\n", aurora.Yellow("encore telemetry disable")))
			telemetry.SetShownWarning()
		}
	})

}
