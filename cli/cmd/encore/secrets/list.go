package secrets

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/cli/internal/platform/gql"
)

var listSecretCmd = &cobra.Command{
	Use:                   "list [keys...]",
	Short:                 "Lists secrets, optionally for a specific key",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		appSlug := cmdutil.AppSlug()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var keys []string
		if len(args) > 0 {
			keys = args
		}
		secrets, err := platform.ListSecretGroups(ctx, appSlug, keys...)
		if err != nil {
			cmdutil.Fatal(err)
		}

		if keys == nil {
			labels := append(envTypeLabels, "Specific Envs")

			printSecretsOverview(allEnvTypes, labels, secrets, func(w *tabwriter.Writer, d secretEnvDesc) {
				for i, env := range d.specific {
					if i > 0 {
						_, _ = fmt.Fprintf(w, ",")
					}
					_, _ = fmt.Fprintf(w, "%s", env.Name)
				}
			})
		} else {
			// Specific secrets
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			_, _ = fmt.Fprint(w, "ID\tSecret Key\tEnvironment(s)\t\n")

			slices.SortFunc(secrets, func(a, b *gql.Secret) int {
				return cmp.Compare(a.Key, b.Key)
			})
			for _, s := range secrets {
				// Sort the archived groups to the end
				slices.SortFunc(s.Groups, func(a, b *gql.SecretGroup) int {
					aa, ab := a.ArchivedAt != nil, b.ArchivedAt != nil
					if aa != ab {
						if aa {
							return 1
						} else {
							return -1
						}
					} else if aa {
						return a.ArchivedAt.Compare(*b.ArchivedAt)
					} else {
						return cmp.Compare(a.ID, b.ID)
					}
				})

				for _, g := range s.Groups {
					var sel []string
					for _, s := range g.Selector {
						switch s := s.(type) {
						case *gql.SecretSelectorSpecificEnv:
							// If we have a specific environment, render the name
							// instead of the id (which is the default when using s.String()).
							sel = append(sel, "env:"+s.Env.Name)
						default:
							sel = append(sel, s.String())
						}
					}

					s := fmt.Sprintf("%s\t%s\t%s\t", g.ID, s.Key, strings.Join(sel, ", "))
					if g.ArchivedAt != nil {
						s += "(archived)\t"
						_, _ = color.New(color.Concealed).Fprintln(w, s)
					} else {
						_, _ = fmt.Fprintln(w, s)
					}
				}
			}
			_ = w.Flush()
		}
	},
}

func init() {
	secretCmd.AddCommand(listSecretCmd)
}
