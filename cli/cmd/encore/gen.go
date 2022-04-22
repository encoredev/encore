package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	daemonpb "encr.dev/proto/encore/daemon"
)

func init() {
	genCmd := &cobra.Command{
		Use:   "gen",
		Short: "Code generation commands",
	}
	rootCmd.AddCommand(genCmd)

	var (
		output  string
		lang    string
		envName string
	)

	genClientCmd := &cobra.Command{
		Use:   "client <app-id> [--env=prod]",
		Short: "Generates an API client for your app",
		Long: `Generates an API client for your app.

By default generates the API based on your primary production environment.
Use '--env=local' to generate it based on your local development version of the app.

Supported language codes are:
  typescript: A TypeScript-client using the in-browser Fetch API
  go: A Go-Lang client"
`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if output == "" && lang == "" {
				fatal("specify at least one of --output or --lang.")
			}
			appID := args[0]

			if lang == "" {
				var ok bool
				lang, ok = detectLang(output)
				if !ok {
					fatal("could not detect language from output.\n\nNote: you can specify the language explicitly with --lang.")
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			daemon := setupDaemon(ctx)
			resp, err := daemon.GenClient(ctx, &daemonpb.GenClientRequest{
				AppId:   appID,
				EnvName: envName,
				Lang:    lang,
			})
			if err != nil {
				fatal(err)
			}

			if output == "" {
				os.Stdout.Write(resp.Code)
			} else {
				if err := ioutil.WriteFile(output, resp.Code, 0755); err != nil {
					fatal(err)
				}
			}
		},
	}

	genCmd.AddCommand(genClientCmd)
	genClientCmd.Flags().StringVarP(&output, "output", "o", "", "The filename to write the generated client code to")
	genClientCmd.Flags().StringVarP(&lang, "lang", "l", "", "The language to generate code for (\"typescript\" or \"go\" are supported)")
	genClientCmd.Flags().StringVarP(&envName, "env", "e", "", "The environment to fetch the API for (defaults to the primary environment)")
}

func detectLang(path string) (string, bool) {
	suffix := strings.ToLower(filepath.Ext(path))
	switch suffix {
	case ".ts":
		return "typescript", true
	case ".go":
		return "go", true
	default:
		return "", false
	}
}
