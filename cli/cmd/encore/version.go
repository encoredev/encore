package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the version of the encore binary.
// It is set using `go build -ldflags "-X main.Version=v1.2.3"`.
var Version string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Reports the current version of the encore application",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stdout, "encore version", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
