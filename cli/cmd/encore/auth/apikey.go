package auth

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/login"
	"encr.dev/internal/conf"
)

var loginApiKeyCmd = &cobra.Command{
	Use:   "login-apikey --auth-key=<KEY>",
	Short: "Log in to Encore using an API Key",
	Run: func(cmd *cobra.Command, args []string) {
		if authKey == "" {
			// If not provided via flag, try reading from stdin if piped, or fail
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				// piped
				_, _ = fmt.Fscan(os.Stdin, &authKey)
			}
		}

		if authKey == "" {
			cmdutil.Fatal("auth key must be provided via --auth-key or stdin")
		}

		if err := doLoginWithAuthKey(authKey); err != nil {
			cmdutil.Fatal(err)
		}
	},
}

func init() {
	loginApiKeyCmd.Flags().StringVarP(&authKey, "auth-key", "k", "", "Auth Key to use for login")
	authCmd.AddCommand(loginApiKeyCmd)
}

func doLoginWithAuthKey(key string) error {
	cfg, err := login.WithAuthKey(key)
	if err != nil {
		return err
	}
	if err := conf.Write(cfg); err != nil {
		return fmt.Errorf("write credentials: %v", err)
	}
	fmt.Fprintln(os.Stdout, "Successfully logged in!")
	return nil
}
