package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	daemonpb "encr.dev/proto/encore/daemon"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Secret management commands",
}

var (
	secretDevFlag  bool
	secretProdFlag bool
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
		if !secretDevFlag && !secretProdFlag {
			fatal("must specify either --dev or --prod.")
		} else if secretDevFlag && secretProdFlag {
			fatal("cannot specify both --dev and --prod.")
		}

		appRoot, _ := determineAppRoot()

		key := args[0]
		var value string
		fd := syscall.Stdin
		if terminal.IsTerminal(int(fd)) {
			fmt.Fprint(os.Stderr, "Enter secret value: ")
			data, err := terminal.ReadPassword(int(fd))
			if err != nil {
				fatal(err)
			}
			value = string(data)
			fmt.Fprintln(os.Stderr)
		} else {
			data, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				fatal(err)
			}
			value = string(bytes.TrimRight(data, "\r\n"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		daemon := setupDaemon(ctx)
		typName := "development"
		typ := daemonpb.SetSecretRequest_DEVELOPMENT
		if secretProdFlag {
			typName = "production"
			typ = daemonpb.SetSecretRequest_PRODUCTION
		}

		resp, err := daemon.SetSecret(ctx, &daemonpb.SetSecretRequest{
			AppRoot: appRoot,
			Key:     key,
			Value:   value,
			Type:    typ,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if resp.Created {
			fmt.Fprintf(os.Stderr, "Successfully created %s secret %s!\n", typName, key)
		} else {
			fmt.Fprintf(os.Stderr, "Successfully updated %s secret %s.\n", typName, key)
		}
	},
}

func init() {
	rootCmd.AddCommand(secretCmd)
	secretCmd.AddCommand(setSecretCmd)
	setSecretCmd.Flags().BoolVarP(&secretDevFlag, "dev", "d", false, "To set the secret for development use")
	setSecretCmd.Flags().BoolVarP(&secretProdFlag, "prod", "p", false, "To set the secret for production use")
}
