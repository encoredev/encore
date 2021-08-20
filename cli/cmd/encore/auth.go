package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"encr.dev/cli/internal/browser"
	"encr.dev/cli/internal/conf"
	"encr.dev/cli/internal/login"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

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
			doLogin()
		},
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Encore",

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			if err := doLogin(); err != nil {
				fatal(err)
			}
		},
	}

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Logs out the currently logged in user",

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			doLogout()
		},
	}

	whoamiCmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the current logged in user",

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			whoami()
		},
	}

	authCmd.AddCommand(signupCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(authCmd)
}

func doLogin() (err error) {
	flow, err := login.Begin()
	if err != nil {
		return err
	}

	if !browser.Open(flow.URL) {
		// On Windows we need a proper \r\n newline to ensure the URL detection doesn't extend to the next line.
		// fmt.Fprintln and family prints just a simple \n, so don't use that.
		fmt.Fprint(os.Stdout, "Log in to Encore using your browser here: ", flow.URL, newline)
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

func doLogout() {
	if err := conf.Logout(); err != nil {
		fmt.Fprintln(os.Stderr, "could not logout:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "encore: logged out.")
}

func whoami() {
	cfg, err := conf.CurrentUser()
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprint(os.Stdout, "not logged in.", newline)
	} else if err != nil {
		fatal(err)
	} else {
		fmt.Fprintf(os.Stdout, "logged in as %s%s", cfg.Email, newline)
	}
}

var newline string

func init() {
	switch runtime.GOOS {
	case "windows":
		newline = "\r\n"
	default:
		newline = "\n"
	}
}
