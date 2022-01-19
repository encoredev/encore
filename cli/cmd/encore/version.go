package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"

	"encr.dev/cli/internal/update"
	"encr.dev/cli/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Reports the current version of the encore application",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			ver string
			err error
		)
		if version.Version != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ver, err = update.Check(ctx)
		}

		fmt.Fprintln(os.Stdout, "encore version", version.Version)
		if err != nil {
			fatalf("could not check for update: %v", err)
		} else if semver.Compare(ver, version.Version) > 0 {
			fmt.Println(aurora.Sprintf(aurora.Yellow("Update available: %s -> %s\nUpdate with: encore version update"), version.Version, ver))
		}
	},
}

var versionUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Checks for an update of encore and, if one is available, runs the appropriate command to update it.",

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if version.Version == "" {
			fatal("cannot update development build, first install Encore from https://encore.dev/docs/intro/install")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ver, err := update.Check(ctx)
		if err != nil {
			fatalf("could not check for update: %v", err)
		}
		if semver.Compare(ver, version.Version) > 0 {
			doUpgrade(ver)
		} else {
			fmt.Println("Encore already up to date.")
		}
	},
}

// doUpgrade upgrades Encore.
// Adapted from flyctl: https://github.com/superfly/flyctl
func doUpgrade(ver string) {
	fmt.Printf("Upgrading Encore to %v...\n", ver)

	script := ""
	switch runtime.GOOS {
	case "windows":
		script = "iwr https://encore.dev/install.ps1 -useb | iex"
	case "darwin":
		// Was it installed via brew?
		if _, err := exec.LookPath("brew"); err == nil {
			script = "brew upgrade encore"
		} else {
			script = "curl -L \"https://encore.dev/install.sh\" | sh"
		}
	default:
		script = "curl -L \"https://encore.dev/install.sh\" | sh"
	}

	arg := "-c"
	shell, ok := os.LookupEnv("SHELL")
	if !ok {
		if runtime.GOOS == "windows" {
			shell = "powershell.exe"
			arg = "-Command"
		} else {
			shell = "/bin/bash"
		}
	}
	fmt.Println("Running update [" + script + "]")
	cmd := exec.Command(shell, arg, script)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func init() {
	versionCmd.AddCommand(versionUpdateCmd)
	rootCmd.AddCommand(versionCmd)
}
