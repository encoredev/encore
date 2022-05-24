package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/spf13/cobra"

	daemonpb "encr.dev/proto/encore/daemon"
)

func init() {
	ejectCmd := &cobra.Command{
		Use:   "eject",
		Short: "eject provides ways to eject your application to migrate away from using Encore",
	}

	var push bool
	dockerEjectCmd := &cobra.Command{
		Use:   "docker IMAGE_TAG",
		Short: "docker builds a portable docker image of your Encore application",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			appRoot, _ := determineAppRoot()
			imgTag := args[0]
			dockerEject(appRoot, imgTag, push)
		},
	}

	dockerEjectCmd.Flags().BoolVarP(&push, "push", "p", false, "push image to remote repository")

	rootCmd.AddCommand(ejectCmd)
	ejectCmd.AddCommand(dockerEjectCmd)
}

func dockerEject(appRoot string, imageTag string, push bool) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	daemon := setupDaemon(ctx)
	params := &daemonpb.DockerExportParams{}
	if push {
		params.PushDestinationTag = imageTag
	} else {
		params.LocalDaemonTag = imageTag
	}

	goos, goarch := "linux", runtime.GOARCH
	cgoEnabled := false
	if s := os.Getenv("CGO_ENABLED"); s != "" {
		cgoEnabled = s == "1"
	}

	stream, err := daemon.Export(ctx, &daemonpb.ExportRequest{
		AppRoot:    appRoot,
		CgoEnabled: cgoEnabled,
		Goos:       goos,
		Goarch:     goarch,
		Format: &daemonpb.ExportRequest_Docker{
			Docker: params,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal: ", err)
		os.Exit(1)
	}
	if code := streamCommandOutput(stream, true); code != 0 {
		os.Exit(code)
	}
	fmt.Print(`
Successfully ejected Encore application.
To run the container, specify the environment variables ENCORE_RUNTIME_CONFIG and ENCORE_APP_SECRETS
as documented here: https://encore.dev/docs/how-to/migrate-away.

`)

}
