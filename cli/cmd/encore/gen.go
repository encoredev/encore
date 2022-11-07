package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/cobra"

	"encr.dev/internal/clientgen"
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
  go: A Go client using net/http"
`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if output == "" && lang == "" {
				fatal("specify at least one of --output or --lang.")
			}
			appID := args[0]

			if lang == "" {
				var ok bool
				l, ok := clientgen.Detect(output)
				// Temporarily disable JavaScript in the CLI
				if !ok || l == clientgen.LangJavascript {
					fatal("could not detect language from output.\n\nNote: you can specify the language explicitly with --lang.")
				}
				lang = string(l)
			} else {
				// Validate the user input for the language
				l, err := clientgen.GetLang(lang)
				if err != nil {
					fatal(fmt.Sprintf("%s: supported langauges are `typescript` and `go`", err))
				}
				lang = string(l)
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

		ValidArgsFunction: autoCompleteAppSlug,
	}

	genWrappersCmd := &cobra.Command{
		Use:   "wrappers",
		Short: "Generates user-facing wrapper code",
		Long: `Manually regenerates user-facing wrapper code.

This is typically not something you ever need to call during regular development,
as Encore automatically regenerates the wrappers whenever the code-base changes.

Its core use case is for CI/CD workflows where you want to run custom linters,
which may require the user-facing wrapper code to be manually generated.`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			appRoot, _ := determineAppRoot()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			daemon := setupDaemon(ctx)
			_, err := daemon.GenWrappers(ctx, &daemonpb.GenWrappersRequest{
				AppRoot: appRoot,
			})
			if err != nil {
				fatal(err)
			} else {
				fmt.Println("successfully generated encore wrappers.")
			}
		},
	}

	genCmd.AddCommand(genClientCmd)
	genCmd.AddCommand(genWrappersCmd)

	genClientCmd.Flags().StringVarP(&lang, "lang", "l", "", "The language to generate code for (\"typescript\" and \"go\" are supported)")
	_ = genClientCmd.RegisterFlagCompletionFunc("lang", autoCompleteFromStaticList(
		"typescript\tA TypeScript client using the in-browser Fetch API",
		"go\tA Go client using net/http",
	))

	genClientCmd.Flags().StringVarP(&output, "output", "o", "", "The filename to write the generated client code to")
	_ = genClientCmd.MarkFlagFilename("output", "go", "ts", "tsx")

	genClientCmd.Flags().StringVarP(&envName, "env", "e", "", "The environment to fetch the API for (defaults to the primary environment)")
	_ = genClientCmd.RegisterFlagCompletionFunc("env", autoCompleteEnvSlug)
}
