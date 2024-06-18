package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
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
		output           string
		lang             string
		envName          string
		genServiceNames  []string
		excludedServices []string
		endpointTags     []string
	)

	genClientCmd := &cobra.Command{
		Use:   "client [<app-id>] [--env=<name>] [--services=foo,bar] [--excluded-services=baz,qux] [--tags=cache,mobile]",
		Short: "Generates an API client for your app",
		Long: `Generates an API client for your app.

By default generates the API based on your primary production environment.
Use '--env=local' to generate it based on your local development version of the app.

Supported language codes are:
  typescript: A TypeScript client using the Fetch API
  javascript: A JavaScript client using the Fetch API
  go: A Go client using net/http"
  openapi: An OpenAPI specification (EXPERIMENTAL)

By default all services with a non-private API endpoint are included.
To further narrow down the services to generate, use the '--services' flag.
`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if output == "" && lang == "" {
				fatal("specify at least one of --output or --lang.")
			}

			// Determine the app id, either from the argument or from the current directory.
			var appID string
			if len(args) == 0 {
				appID = cmdutil.AppSlug()
			} else {
				appID = args[0]
			}

			if lang == "" {
				var ok bool
				l, ok := clientgen.Detect(output)
				if !ok {
					fatal("could not detect language from output.\n\nNote: you can specify the language explicitly with --lang.")
				}
				lang = string(l)
			} else {
				// Validate the user input for the language
				l, err := clientgen.GetLang(lang)
				if err != nil {
					fatal(fmt.Sprintf("%s: supported langauges are `typescript`, `javascript`, and `go`", err))
				}
				lang = string(l)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			daemon := setupDaemon(ctx)

			if genServiceNames == nil {
				genServiceNames = []string{"*"}
			}
			resp, err := daemon.GenClient(ctx, &daemonpb.GenClientRequest{
				AppId:            appID,
				EnvName:          envName,
				Lang:             lang,
				Services:         genServiceNames,
				ExcludedServices: excludedServices,
				EndpointTags:     endpointTags,
			})
			if err != nil {
				fatal(err)
			}

			if output == "" {
				_, _ = os.Stdout.Write(resp.Code)
			} else {
				if err := os.WriteFile(output, resp.Code, 0755); err != nil {
					fatal(err)
				}
			}
		},

		ValidArgsFunction: cmdutil.AutoCompleteAppSlug,
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
			ctx := context.Background()
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

	genClientCmd.Flags().StringVarP(&lang, "lang", "l", "", "The language to generate code for (\"typescript\", \"javascript\", \"go\", and \"openapi\" are supported)")
	_ = genClientCmd.RegisterFlagCompletionFunc("lang", cmdutil.AutoCompleteFromStaticList(
		"typescript\tA TypeScript client using the in-browser Fetch API",
		"javascript\tA JavaScript client using the in-browser Fetch API",
		"go\tA Go client using net/http",
		"openapi\tAn OpenAPI specification",
	))

	genClientCmd.Flags().StringVarP(&output, "output", "o", "", "The filename to write the generated client code to")
	_ = genClientCmd.MarkFlagFilename("output", "go", "ts", "tsx", "js", "jsx")

	genClientCmd.Flags().StringVarP(&envName, "env", "e", "", "The environment to fetch the API for (defaults to the primary environment)")
	_ = genClientCmd.RegisterFlagCompletionFunc("env", cmdutil.AutoCompleteEnvSlug)

	genClientCmd.Flags().StringSliceVarP(&genServiceNames, "services", "s", nil, "The names of the services to include in the output")
	genClientCmd.Flags().StringSliceVarP(&excludedServices, "excluded-services", "x", nil, "The names of the services to exclude in the output")
	genClientCmd.Flags().StringSliceVarP(&endpointTags, "tags", "t", nil, "The names of endpoint tags to include in the output")
}
