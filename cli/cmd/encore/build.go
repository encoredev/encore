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

var (
	targetOS = cmdutil.Oneof{
		Value:   "linux",
		Allowed: []string{"linux"},
		Flag:    "os",
		Desc:    "the target operating system",
	}
	targetArch = cmdutil.Oneof{
		Value:   "amd64",
		Allowed: []string{"amd64", "arm64"},
		Flag:    "arch",
		Desc:    "the target architecture",
	}
)

func init() {
	buildCmd := &cobra.Command{
		Use:     "build",
		Aliases: []string{"eject"},
		Short:   "build provides ways to build your application for deployment",
	}

	p := buildParams{
		CgoEnabled: os.Getenv("CGO_ENABLED") == "1",
	}
	dockerBuildCmd := &cobra.Command{
		Use:   "docker IMAGE_TAG",
		Short: "docker builds a portable docker image of your Encore application",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			p.Goarch = targetArch.Value
			p.Goos = targetOS.Value
			p.AppRoot, _ = determineAppRoot()
			p.WorkspaceRoot = determineWorkspaceRoot(p.AppRoot)
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
			dockerBuild(p)
		},
	}

	dockerBuildCmd.Flags().BoolVarP(&p.Push, "push", "p", false, "push image to remote repository")
	dockerBuildCmd.Flags().StringVar(&p.BaseImg, "base", "scratch", "base image to build from")
	dockerBuildCmd.Flags().BoolVar(&p.CgoEnabled, "cgo", false, "enable cgo")
	dockerBuildCmd.Flags().BoolVar(&p.SkipInfraConf, "skip-config", false, "do not read or generate a infra configuration file")
	dockerBuildCmd.Flags().StringVar(&p.InfraConfPath, "config", "", "infra configuration file path")
	p.Services = dockerBuildCmd.Flags().StringSlice("services", nil, "services to include in the image")
	p.Gateways = dockerBuildCmd.Flags().StringSlice("gateways", nil, "gateways to include in the image")
	targetOS.AddFlag(dockerBuildCmd)
	targetArch.AddFlag(dockerBuildCmd)
	rootCmd.AddCommand(buildCmd)
	buildCmd.AddCommand(dockerBuildCmd)
}

type buildParams struct {
	AppRoot       string
	WorkspaceRoot string
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

func dockerBuild(p buildParams) {
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
		AppRoot:       p.AppRoot,
		WorkspaceRoot: p.WorkspaceRoot,
		CgoEnabled:    p.CgoEnabled,
		Goos:          p.Goos,
		Goarch:        p.Goarch,
		Environ:       os.Environ(),
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
	if code := cmdutil.StreamCommandOutput(stream, cmdutil.ConvertJSONLogs()); code != 0 {
		os.Exit(code)
	}
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
