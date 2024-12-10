package secrets

import (
	"bytes"
	"context"
	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/cli/internal/platform/gql"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"text/tabwriter"

	"encr.dev/cli/cmd/encore/root"
)

var allEnvTypes = []string{
	"production",
	"development",
	"local",
	"preview",
}

var secretCmd = &cobra.Command{
	Use:     "secret",
	Short:   "Secret management commands",
	Aliases: []string{"secrets"},
}

func init() {
	root.Cmd.AddCommand(secretCmd)
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

var envTypeLabels = []string{
	"Production",
	"Development",
	"Local",
	"Preview",
}

func printSecretsOverview(envTypes, envTypeLabels []string, secrets []*gql.Secret, af func(w *tabwriter.Writer, d secretEnvDesc)) {
	// Print secrets overview
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', tabwriter.StripEscape)

	_, _ = fmt.Fprint(w, "Secret Key\t")

	for _, t := range envTypeLabels {
		if !hasEnvType(t, envTypes) {
			continue
		}

		_, _ = fmt.Fprintf(w, "%s\t", t)
	}

	_, _ = fmt.Fprint(w, "\n")

	const (
		checkYes = "\u2713"
		checkNo  = "\u2717"
	)

	render := func(b bool) string {
		if b {
			return checkYes
		} else {
			return checkNo
		}
	}

	for _, s := range secrets {
		d := getSecretEnvDesc(s.Groups)
		if !d.hasAny {
			continue
		}

		_, _ = fmt.Fprintf(w, "%s\t", s.Key)

		if hasEnvType("production", envTypes) {
			_, _ = fmt.Fprintf(w, "%v\t", render(d.prod))
		}

		if hasEnvType("development", envTypes) {
			_, _ = fmt.Fprintf(w, "%v\t", render(d.dev))
		}

		if hasEnvType("local", envTypes) {
			_, _ = fmt.Fprintf(w, "%v\t", render(d.local))
		}

		if hasEnvType("preview", envTypes) {
			_, _ = fmt.Fprintf(w, "%v\t", render(d.preview))
		}

		if af != nil {
			af(w, d)
		}

		_, _ = fmt.Fprint(w, "\t\n")
	}

	_ = w.Flush()

	// Add color to the checkmarks now that the table is correctly laid out.
	// We can't do it before since the tabwriter will get the alignment wrong
	// if we include a bunch of ANSI escape codes that it doesn't understand.
	r := strings.NewReplacer(checkYes, color.GreenString(checkYes), checkNo, color.RedString(checkNo))
	_, _ = r.WriteString(os.Stdout, buf.String())
}

func hasEnvType(t string, envTypes []string) bool {
	for _, et := range envTypes {
		if et == strings.ToLower(t) {
			return true
		}
	}

	return false
}

var secretEnvs secretEnvSelector

type secretEnvSelector struct {
	devFlag  bool
	prodFlag bool
	envTypes []string
	envNames []string
}

func (s secretEnvSelector) ParseSelector(ctx context.Context, appSlug string) []gql.SecretSelector {
	if s.devFlag && s.prodFlag {
		cmdutil.Fatal("cannot specify both --dev and --prod")
	} else if s.devFlag && (len(s.envTypes) > 0 || len(s.envNames) > 0) {
		cmdutil.Fatal("cannot combine --dev with --type/--env")
	} else if s.prodFlag && (len(s.envTypes) > 0 || len(s.envNames) > 0) {
		cmdutil.Fatal("cannot combine --prod with --type/--env")
	}

	// Look up the environments
	envMap := make(map[string]string) // name -> id
	envs, err := platform.ListEnvs(ctx, appSlug)
	if err != nil {
		cmdutil.Fatalf("unable to list environments: %v", err)
	}
	for _, env := range envs {
		envMap[env.Slug] = env.ID
	}

	var sel []gql.SecretSelector
	if s.devFlag {
		sel = append(sel,
			&gql.SecretSelectorEnvType{Kind: "development"},
			&gql.SecretSelectorEnvType{Kind: "preview"},
			&gql.SecretSelectorEnvType{Kind: "local"},
		)
	} else if s.prodFlag {
		sel = append(sel, &gql.SecretSelectorEnvType{Kind: "production"})
	} else {
		for _, t := range s.ParseEnvTypes() {
			sel = append(sel, &gql.SecretSelectorEnvType{Kind: t})
		}

		for _, n := range s.envNames {
			envID, ok := envMap[n]
			if !ok {
				cmdutil.Fatalf("environment %q not found", n)
			}
			sel = append(sel, &gql.SecretSelectorSpecificEnv{Env: &gql.Env{ID: envID}})
		}
	}

	if len(sel) == 0 {
		cmdutil.Fatal("must specify at least one environment with --type/--env (or --dev/--prod)")
	}
	return sel
}

func (s secretEnvSelector) ParseEnvTypes() []string {
	var envTypes []string

	// Parse env types and env names
	seenTypes := make(map[string]bool)
	validTypes := map[string]string{
		// Actual names
		"development": "development",
		"production":  "production",
		"preview":     "preview",
		"local":       "local",

		// Aliases
		"dev":       "development",
		"prod":      "production",
		"pr":        "preview",
		"ephemeral": "preview",
	}

	for _, t := range s.envTypes {
		val, ok := validTypes[t]
		if !ok {
			cmdutil.Fatalf("invalid environment type %q", t)
		}
		if !seenTypes[val] {
			seenTypes[val] = true
			envTypes = append(envTypes, val)
		}
	}

	return envTypes
}
