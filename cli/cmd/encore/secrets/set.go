package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/exp/slices"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/cli/internal/platform/gql"
	daemonpb "encr.dev/proto/encore/daemon"
)

var setSecretCmd = &cobra.Command{
	Use:   "set --dev|prod <key>",
	Short: "Sets a secret value",
	Example: `
Entering a secret directly in terminal:

	$ encore secret set --dev MySecret
	Enter secret value: ...
	Successfully created development secret MySecret.

Piping a secret from a file:

	$ encore secret set --dev MySecret < my-secret.txt
	Successfully created development secret MySecret.

Note that this strips trailing newlines from the secret value.`,
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		setSecret(args[0])
	},
}

var secretEnvs secretEnvSelector

type secretEnvSelector struct {
	devFlag  bool
	prodFlag bool
	envTypes []string
	envNames []string
}

func init() {
	secretCmd.AddCommand(setSecretCmd)
	setSecretCmd.Flags().BoolVarP(&secretEnvs.devFlag, "dev", "d", false, "To set the secret for development use")
	setSecretCmd.Flags().BoolVarP(&secretEnvs.prodFlag, "prod", "p", false, "To set the secret for production use")
	setSecretCmd.Flags().StringSliceVarP(&secretEnvs.envTypes, "type", "t", nil, "To set the secret for specific environment types")
	setSecretCmd.Flags().StringSliceVarP(&secretEnvs.envNames, "env", "e", nil, "To set the secret for specific environment names")
}

func setSecret(key string) {
	plaintextValue := readSecretValue()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	appRoot, _ := cmdutil.AppRoot()
	appSlug := cmdutil.AppSlug()
	sel := secretEnvs.ParseSelector(ctx, appSlug)

	app, err := platform.GetApp(ctx, appSlug)
	if err != nil {
		cmdutil.Fatalf("unable to lookup app %s: %v", appSlug, err)
	}

	// Does a matching secret group already exist?
	secrets, err := platform.ListSecretGroups(ctx, app.Slug, []string{key})
	if err != nil {
		cmdutil.Fatalf("unable to list secrets: %v", err)
	}

	if matching := findMatchingSecretGroup(secrets, key, sel); matching != nil {
		// We found a matching secret group. Update it.
		err := platform.CreateSecretVersion(ctx, platform.CreateSecretVersionParams{
			GroupID:        matching.ID,
			PlaintextValue: plaintextValue,
			Etag:           matching.Etag,
		})
		if err != nil {
			cmdutil.Fatalf("unable to update secret: %v", err)
		}
		fmt.Printf("Successfully updated secret value for %s.\n", key)
		return
	}

	// Otherwise create a new secret group.
	err = platform.CreateSecretGroup(ctx, platform.CreateSecretGroupParams{
		AppID:          app.ID,
		Key:            key,
		PlaintextValue: plaintextValue,
		Selector:       sel,
		Description:    "", // not yet supported from CLI
	})
	if err != nil {
		if ce, ok := getConflictError(err); ok {
			var errMsg strings.Builder
			fmt.Fprintln(&errMsg, "the environment selection conflicts with other secret values:")
			for _, c := range ce.Conflicts {
				fmt.Fprintf(&errMsg, "\t%s %s\n", c.GroupID, strings.Join(c.Conflicts, ", "))
			}
			cmdutil.Fatal(errMsg.String())
		}
		cmdutil.Fatalf("unable to create secret: %v", err)
	}

	daemon := cmdutil.ConnectDaemon(ctx)
	if _, err := daemon.SecretsRefresh(ctx, &daemonpb.SecretsRefreshRequest{AppRoot: appRoot}); err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to refresh secret secret, skipping:", err)
	}

	fmt.Printf("Successfully created secret value for %s.\n", key)
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
				sel = append(sel, &gql.SecretSelectorEnvType{Kind: val})
			}
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

// readSecretValue reads the secret value from the user.
// If it's a terminal it becomes an interactive prompt,
// otherwise it reads from stdin.
func readSecretValue() string {
	var value string
	fd := syscall.Stdin
	if terminal.IsTerminal(int(fd)) {
		fmt.Fprint(os.Stderr, "Enter secret value: ")
		data, err := terminal.ReadPassword(int(fd))
		if err != nil {
			cmdutil.Fatal(err)
		}
		value = string(data)
		fmt.Fprintln(os.Stderr)
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			cmdutil.Fatal(err)
		}
		value = string(bytes.TrimRight(data, "\r\n"))
	}
	return value
}

// findMatchingSecretGroup find whether a matching secret group already exists
// for the given secret key and selector.
func findMatchingSecretGroup(secrets []*gql.Secret, key string, selector []gql.SecretSelector) *gql.SecretGroup {
	// canonicalize returns the secret selectors in canonical form
	canonicalize := func(sels []gql.SecretSelector) []string {
		var strs []string
		for _, s := range sels {
			strs = append(strs, s.String())
		}
		sort.Strings(strs)
		return strs
	}

	want := canonicalize(selector)
	for _, s := range secrets {
		if s.Key == key {
			for _, g := range s.Groups {
				got := canonicalize(g.Selector)
				if slices.Equal(got, want) {
					return g
				}
			}
		}
	}
	return nil
}

func getConflictError(err error) (*gql.ConflictError, bool) {
	var gqlErr gql.ErrorList
	if !errors.As(err, &gqlErr) {
		return nil, false
	}
	for _, e := range gqlErr {
		if conflict := e.Extensions["conflict"]; len(conflict) > 0 {
			var cerr gql.ConflictError
			if err := json.Unmarshal(conflict, &cerr); err == nil {
				return &cerr, true
			}
		}
	}
	return nil, false
}
