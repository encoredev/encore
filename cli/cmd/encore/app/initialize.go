package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/auth"
	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/internal/conf"
	"encr.dev/pkg/xos"
)

// Create a new app from scratch: `encore app create`
// Link an existing app to an existing repo: `encore app link <app-id>`
// Link an existing repo to a new app: `encore app init <name>`

func init() {
	initAppCmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Create a new Encore app from an existing repository",
		Args:  cobra.MaximumNArgs(1),

		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			var name string
			if len(args) > 0 {
				name = args[0]
			}
			if err := initializeApp(name); err != nil {
				cmdutil.Fatal(err)
			}
		},
	}

	appCmd.AddCommand(initAppCmd)
}

func initializeApp(name string) error {
	// Check if encore.app file exists
	_, _, err := cmdutil.MaybeAppRoot()
	if errors.Is(err, cmdutil.ErrNoEncoreApp) {
		// expected
	} else if err != nil {
		cmdutil.Fatal(err)
	} else if err == nil {
		// There is already an app here or in a parent directory.
		cmdutil.Fatal("an encore.app file already exists (here or in a parent directory)")
	}

	cyan := color.New(color.FgCyan)
	if _, err := conf.CurrentUser(); errors.Is(err, fs.ErrNotExist) {
		_, _ = cyan.Fprint(os.Stderr, "Log in to create your app [press enter to continue]: ")
		_, _ = fmt.Scanln()
		if err := auth.DoLogin(auth.AutoFlow); err != nil {
			cmdutil.Fatal(err)
		}
	}

	if name == "" {
		name, _ = selectTemplate("", "empty")
	}
	if err := validateName(name); err != nil {
		return err
	}

	// Create the app on the server.
	if _, err := conf.CurrentUser(); err != nil {
		cmdutil.Fatal("you must be logged in to initialize a new app")
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Prefix = "Creating app on encore.dev "
	s.Start()

	app, err := createAppOnServer(name, exampleConfig{})
	s.Stop()
	if err != nil {
		return fmt.Errorf("creating app on encore.dev: %v", err)
	}

	// Create the encore.app file
	encoreAppData := []byte(`{
	"id": "` + app.Slug + `",
}
`)
	if err := xos.WriteFile("encore.app", encoreAppData, 0644); err != nil {
		return err
	}

	// Update to latest encore.dev release if this looks to be a go module.
	if _, err := os.Stat("go.mod"); err == nil {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = "Running go get encore.dev@latest"
		s.Start()
		if err := gogetEncore(name); err != nil {
			s.FinalMSG = fmt.Sprintf("failed, skipping: %v", err.Error())
		}
		s.Stop()
	}

	return nil
}
