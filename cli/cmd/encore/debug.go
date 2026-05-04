package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	daemonpb "encr.dev/proto/encore/daemon"
)

var (
	debugBuildCodegenDebug bool
	debugBuildParseTests   bool
)

func init() {
	debugCmd := &cobra.Command{
		Use:    "debug",
		Short:  "debug is a collection of debug commands",
		Hidden: true,
	}

	buildCmd := &cobra.Command{
		Use:                   "build",
		Short:                 "Checks your application for compile-time errors using Encore's compiler.",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			appRoot, relPath := determineAppRoot()
			runDebugBuild(appRoot, relPath)
		},
	}
	buildCmd.Flags().BoolVar(&debugBuildCodegenDebug, "codegen-debug", false, "Dump generated code (for debugging Encore's code generation)")
	buildCmd.Flags().BoolVar(&debugBuildParseTests, "tests", false, "Parse tests as well")

	format := cmdutil.Oneof{
		Value:     "proto",
		Allowed:   []string{"proto", "json"},
		Flag:      "format",
		FlagShort: "f",
		Desc:      "Output format",
	}

	toFormat := func() daemonpb.DumpMetaRequest_Format {
		switch format.Value {
		case "proto":
			return daemonpb.DumpMetaRequest_FORMAT_PROTO
		case "json":
			return daemonpb.DumpMetaRequest_FORMAT_JSON
		default:
			return daemonpb.DumpMetaRequest_FORMAT_UNSPECIFIED
		}
	}

	var p dumpMetaParams
	dumpMeta := &cobra.Command{
		Use:   "meta",
		Short: "Outputs the parsed metadata",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			p.AppRoot, p.WorkingDir = determineAppRoot()
			p.Environ = os.Environ()
			p.Format = toFormat()
			dumpMeta(p)
		},
	}

	format.AddFlag(dumpMeta)
	dumpMeta.Flags().BoolVar(&p.ParseTests, "tests", false, "Parse tests as well")
	rootCmd.AddCommand(debugCmd)
	debugCmd.AddCommand(buildCmd)
	debugCmd.AddCommand(dumpMeta)
}

func runDebugBuild(appRoot, relPath string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	daemon := setupDaemon(ctx)
	stream, err := daemon.Check(ctx, &daemonpb.CheckRequest{
		AppRoot:      appRoot,
		WorkingDir:   relPath,
		CodegenDebug: debugBuildCodegenDebug,
		ParseTests:   debugBuildParseTests,
		Environ:      os.Environ(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal: ", err)
		os.Exit(1)
	}
	os.Exit(cmdutil.StreamCommandOutput(stream, nil))
}

type dumpMetaParams struct {
	AppRoot    string
	WorkingDir string
	ParseTests bool
	Format     daemonpb.DumpMetaRequest_Format
	Environ    []string
}

func dumpMeta(p dumpMetaParams) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	daemon := setupDaemon(ctx)
	resp, err := daemon.DumpMeta(ctx, &daemonpb.DumpMetaRequest{
		AppRoot:    p.AppRoot,
		WorkingDir: p.WorkingDir,
		ParseTests: p.ParseTests,
		Environ:    p.Environ,
		Format:     p.Format,
	})
	if err != nil {
		fatal(err)
	}
	_, _ = os.Stdout.Write(resp.Meta)
}
