package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/cli/internal/browser"
	"encr.dev/cli/internal/login"
	"encr.dev/internal/conf"
)

var authKey string

func init() {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Commands to authenticate with Encore",
	}

	signupCmd := &cobra.Command{
		Use:   "signup",
		Short: "Create a new Encore account",

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			if err := DoLogin(); err != nil {
				cmdutil.Fatal(err)
			}
		},
	}

	loginCmd := &cobra.Command{
		Use:   "login [--auth-key]",
		Short: "Log in to Encore",

		Run: func(cmd *cobra.Command, args []string) {
			if authKey != "" {
				if err := DoLoginWithAuthKey(); err != nil {
					cmdutil.Fatal(err)
				}
			} else {
				if err := DoLogin(); err != nil {
					cmdutil.Fatal(err)
				}
			}
		},
	}

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Logs out the currently logged in user",

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			DoLogout()
		},
	}

	whoamiCmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the current logged in user",

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			Whoami()
		},
	}

	authCmd.AddCommand(signupCmd)

	authCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVarP(&authKey, "auth-key", "k", "", "Auth Key to use for login")

	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(whoamiCmd)
	root.Cmd.AddCommand(authCmd)
}

func DoLogin() (err error) {
	flow, err := login.Begin()
	if err != nil {
		return err
	}

	if !browser.Open(flow.URL) {
		// On Windows we need a proper \r\n newline to ensure the URL detection doesn't extend to the next line.
		// fmt.Fprintln and family prints just a simple \n, so don't use that.
		fmt.Fprint(os.Stdout, "Log in to Encore using your browser here: ", flow.URL, cmdutil.Newline)
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Prefix = "Waiting for login to complete "
	s.Start()
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	select {
	case cfg := <-flow.LoginCh:
		if err := conf.Write(cfg); err != nil {
			return fmt.Errorf("write credentials: %v", err)
		}
		s.Stop()
		fmt.Fprintln(os.Stdout, "Successfully logged in!")
		return nil
	case <-time.After(10 * time.Minute):
		flow.Close()
		return fmt.Errorf("timed out")
	}
}

func DoLogout() {
	if err := conf.Logout(); err != nil {
		fmt.Fprintln(os.Stderr, "could not logout:", err)
		os.Exit(1)
	}
	// Stop running daemon to clear any cached credentials
	cmdutil.StopDaemon()
	fmt.Fprintln(os.Stdout, "encore: logged out.")
}

func DoLoginWithAuthKey() error {
	cfg, err := login.WithAuthKey(authKey)
	if err != nil {
		return err
	}
	if err := conf.Write(cfg); err != nil {
		return fmt.Errorf("write credentials: %v", err)
	}
	fmt.Fprintln(os.Stdout, "Successfully logged in!")
	return nil
}

func Whoami() {
	cfg, err := conf.CurrentUser()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprint(os.Stdout, "not logged in.", cmdutil.Newline)
			return
		}
		cmdutil.Fatal(err)
	}

	if cfg.AppSlug != "" {
		fmt.Fprintf(os.Stdout, "logged in as app %s%s", cfg.AppSlug, cmdutil.Newline)
	} else {
		fmt.Fprintf(os.Stdout, "logged in as %s%s", cfg.Email, cmdutil.Newline)
	}
}
