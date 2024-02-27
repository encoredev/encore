package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	daemonpb "encr.dev/proto/encore/daemon"
)

func init() {
	debugCmd := &cobra.Command{
		Use:    "debug",
		Short:  "debug is a collection of debug commands",
		Hidden: true,
	}

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
	debugCmd.AddCommand(dumpMeta)
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
