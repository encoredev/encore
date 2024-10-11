package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/pkg/appfile"
	daemonpb "encr.dev/proto/encore/daemon"
)

func init() {
	ejectCmd := &cobra.Command{
		Use:   "eject",
		Short: "eject provides ways to eject your application to migrate away from using Encore",
	}

	p := ejectParams{
		CgoEnabled: os.Getenv("CGO_ENABLED") == "1",
		Goos:       or(os.Getenv("GOOS"), "linux"),
		Goarch:     or(os.Getenv("GOARCH"), "amd64"),
	}
	dockerEjectCmd := &cobra.Command{
		Use:   "docker IMAGE_TAG",
		Short: "docker builds a portable docker image of your Encore application",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			p.AppRoot, _ = determineAppRoot()
			file, err := appfile.ParseFile(filepath.Join(p.AppRoot, appfile.Name))
			if err == nil {
				if !cmd.Flag("base").Changed && file.Lang == appfile.LangTS {
					p.BaseImg = "node:slim"
				}
				if !cmd.Flag("cgo").Changed {
					p.CgoEnabled = file.Build.CgoEnabled
				}
			}
			p.ImageTag = args[0]
			dockerEject(p)
		},
	}

	dockerEjectCmd.Flags().BoolVarP(&p.Push, "push", "p", false, "push image to remote repository")
	dockerEjectCmd.Flags().StringVar(&p.BaseImg, "base", "scratch", "base image to build from")
	dockerEjectCmd.Flags().StringVar(&p.Goos, "os", p.Goos, "target operating system. One of (linux, darwin, windows)")
	dockerEjectCmd.Flags().StringVar(&p.Goarch, "arch", p.Goarch, "target architecture. One of (amd64, arm64)")
	dockerEjectCmd.Flags().BoolVar(&p.CgoEnabled, "cgo", false, "enable cgo")
	dockerEjectCmd.Flags().BoolVar(&p.SkipInfraConf, "skip-config", false, "do not read or generate a infra configuration file")
	dockerEjectCmd.Flags().StringVar(&p.InfraConfPath, "config", "", "infra configuration file path")
	p.Services = dockerEjectCmd.Flags().StringSlice("services", nil, "services to include in the image")
	p.Gateways = dockerEjectCmd.Flags().StringSlice("gateways", nil, "gateways to include in the image")
	rootCmd.AddCommand(ejectCmd)
	ejectCmd.AddCommand(dockerEjectCmd)
}

type ejectParams struct {
	AppRoot       string
	ImageTag      string
	Push          bool
	BaseImg       string
	Goos          string
	Goarch        string
	CgoEnabled    bool
	SkipInfraConf bool
	InfraConfPath string
	Services      *[]string
	Gateways      *[]string
}

func dockerEject(p ejectParams) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-interrupt
		cancel()
	}()

	daemon := setupDaemon(ctx)
	params := &daemonpb.DockerExportParams{
		BaseImageTag: p.BaseImg,
	}
	if p.Push {
		params.PushDestinationTag = p.ImageTag
	} else {
		params.LocalDaemonTag = p.ImageTag
	}

	var services, gateways []string
	if p.Services != nil {
		services = *p.Services
	}
	if p.Gateways != nil {
		gateways = *p.Gateways
	}
	var err error
	cfgPath := ""
	if p.InfraConfPath != "" {
		cfgPath, err = filepath.Abs(p.InfraConfPath)
		if err != nil {
			cmdutil.Fatalf("failed to resolve absolute path for %s: %v", p.InfraConfPath, err)
		}
	}
	stream, err := daemon.Export(ctx, &daemonpb.ExportRequest{
		AppRoot:    p.AppRoot,
		CgoEnabled: p.CgoEnabled,
		Goos:       p.Goos,
		Goarch:     p.Goarch,
		Environ:    os.Environ(),
		Format: &daemonpb.ExportRequest_Docker{
			Docker: params,
		},
		InfraConfPath: cfgPath,
		Services:      services,
		Gateways:      gateways,
		SkipInfraConf: p.SkipInfraConf,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal: ", err)
		os.Exit(1)
	}
	if code := streamCommandOutput(stream, convertJSONLogs()); code != 0 {
		os.Exit(code)
	}
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
