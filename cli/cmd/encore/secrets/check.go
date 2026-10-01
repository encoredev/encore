package secrets

import (
	"context"
	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"fmt"
	"github.com/spf13/cobra"
	"time"
)

var checkSecretsCmd = &cobra.Command{
	Use:                   "check [envTypes...]",
	Short:                 "Check missing secrets for specified environment types (all types by default)",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		appSlug := cmdutil.AppSlug()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		s := secretEnvSelector{
			envTypes: args,
		}

		envTypes := s.ParseEnvTypes()

		if len(envTypes) == 0 {
			envTypes = allEnvTypes
		}

		secrets, err := platform.ListSecretGroups(ctx, appSlug)
		if err != nil {
			cmdutil.Fatal(err)
		}

		printSecretsOverview(envTypes, envTypeLabels, secrets, nil)

		missing := make(map[string]bool)

		for _, s := range secrets {
			d := getSecretEnvDesc(s.Groups)
			if !d.hasAny {
				continue
			}

			for _, t := range envTypes {
				if t == "production" && !d.prod {
					missing[s.Key] = true
				}

				if t == "development" && !d.dev {
					missing[s.Key] = true
				}

				if t == "local" && !d.local {
					missing[s.Key] = true
				}

				if t == "preview" && !d.preview {
					missing[s.Key] = true
				}
			}
		}

		if len(missing) > 0 {
			fmt.Println()
			cmdutil.Fatalf("%d secret(s) don't have matching values in all required environment types", len(missing))
		}
	},
}

func init() {
	secretCmd.AddCommand(checkSecretsCmd)
}
