package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/cli/internal/platform/gql"
	daemonpb "encr.dev/proto/encore/daemon"
)

var setSecretCmd = &cobra.Command{
	Use:   "set --type <types> <secret-name>",
	Short: "Sets a secret value",
	Long: `
Sets a secret value for one or more environment types.

The valid environment types are 'prod', 'dev', 'pr' and 'local'.
`,

	Example: `
Entering a secret directly in terminal:

	$ encore secret set --type dev,local MySecret
	Enter secret value: ...
	Successfully created secret value for MySecret.

Piping a secret from a file:

	$ encore secret set --type dev,local,pr MySecret < my-secret.txt
	Successfully created secret value for MySecret.

Note that this strips trailing newlines from the secret value.`,
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		setSecret(args[0])
	},
}

func init() {
	secretCmd.AddCommand(setSecretCmd)
	setSecretCmd.Flags().BoolVarP(&secretEnvs.devFlag, "dev", "d", false, "To set the secret for development use")
	setSecretCmd.Flags().BoolVarP(&secretEnvs.prodFlag, "prod", "p", false, "To set the secret for production use")
	setSecretCmd.Flags().StringSliceVarP(&secretEnvs.envTypes, "type", "t", nil, "environment type(s) to set for (comma-separated list)")
	setSecretCmd.Flags().StringSliceVarP(&secretEnvs.envNames, "env", "e", nil, "environment name(s) to set for (comma-separated list)")
	_ = setSecretCmd.Flags().MarkHidden("dev")
	_ = setSecretCmd.Flags().MarkHidden("prod")
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
	secrets, err := platform.ListSecretGroups(ctx, app.Slug, []string{key}...)
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
