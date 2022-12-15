package secrets

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

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
		secrets, err := platform.ListSecretGroups(ctx, appSlug, keys)
		if err != nil {
			cmdutil.Fatal(err)
		}

		if keys == nil {
			// Print secrets overview
			var buf bytes.Buffer
			w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', tabwriter.StripEscape)

			fmt.Fprint(w, "Secret Key\tProduction\tDevelopment\tLocal\tPreview\tSpecific Envs\t\n")
			const (
				checkYes = "\u2713"
				checkNo  = "\u2717"
			)
			for _, s := range secrets {
				render := func(b bool) string {
					if b {
						return checkYes
					} else {
						return checkNo
					}
				}
				d := getSecretEnvDesc(s.Groups)
				if !d.hasAny {
					continue
				}
				fmt.Fprintf(w, "%s\t%v\t%v\t%v\t%v\t", s.Key,
					render(d.prod), render(d.dev), render(d.local), render(d.preview))
				// Render specific envs, if any
				for i, env := range d.specific {
					if i > 0 {
						fmt.Fprintf(w, ",")
					}
					fmt.Fprintf(w, "%s", env.Name)
				}

				fmt.Fprint(w, "\t\n")
			}
			w.Flush()

			// Add color to the checkmarks now that the table is correctly laid out.
			// We can't do it before since the tabwriter will get the alignment wrong
			// if we include a bunch of ANSI escape codes that it doesn't understand.
			r := strings.NewReplacer(checkYes, color.GreenString(checkYes), checkNo, color.RedString(checkNo))
			r.WriteString(os.Stdout, buf.String())
		} else {
			// Specific secrets
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprint(w, "ID\tSecret Key\tEnvironment(s)\t\n")

			slices.SortFunc(secrets, func(a, b *gql.Secret) bool {
				return a.Key < b.Key
			})
			for _, s := range secrets {
				// Sort the archived groups to the end
				slices.SortFunc(s.Groups, func(a, b *gql.SecretGroup) bool {
					aa, ab := a.ArchivedAt != nil, b.ArchivedAt != nil
					if aa != ab {
						return !aa
					} else if aa {
						return a.ArchivedAt.After(*b.ArchivedAt)
					} else {
						return a.ID < b.ID
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
						color.New(color.Concealed).Fprintln(w, s)
					} else {
						fmt.Fprintln(w, s)
					}
				}
			}
			w.Flush()
		}
	},
}

func init() {
	secretCmd.AddCommand(listSecretCmd)
}

type secretEnvDesc struct {
	hasAny                    bool // if there are any non-archived groups at all
	prod, dev, local, preview bool
	specific                  []*gql.Env
}

func getSecretEnvDesc(groups []*gql.SecretGroup) secretEnvDesc {
	var desc secretEnvDesc
	for _, g := range groups {
		if g.ArchivedAt != nil {
			continue
		}
		desc.hasAny = true
		for _, sel := range g.Selector {
			switch sel := sel.(type) {
			case *gql.SecretSelectorEnvType:
				switch sel.Kind {
				case "production":
					desc.prod = true
				case "development":
					desc.dev = true
				case "local":
					desc.local = true
				case "preview":
					desc.preview = true
				}
			case *gql.SecretSelectorSpecificEnv:
				desc.specific = append(desc.specific, sel.Env)
			}
		}
	}
	return desc
}
