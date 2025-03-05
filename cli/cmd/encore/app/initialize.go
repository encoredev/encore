package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/internal/conf"
	"encr.dev/pkg/xos"
)

const (
	tsEncoreAppData = `{%s
	"id": "%s",
	"lang": "typescript",
}
`
	goEncoreAppData = `{%s
	"id": "%s",
}
`
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
	promptAccountCreation()

	name, _, lang := selectTemplate(name, "", true)

	if err := validateName(name); err != nil {
		return err
	}

	appSlug := ""
	appSlugComments := ""
	// Create the app on the server.
	if _, err := conf.CurrentUser(); err == nil {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = "Creating app on encore.dev "
		s.Start()

		app, err := createAppOnServer(name, exampleConfig{})
		s.Stop()
		if err != nil {
			return fmt.Errorf("creating app on encore.dev: %v", err)
		}
		appSlug = app.Slug
	}

	// Create the encore.app file
	var encoreAppTemplate = goEncoreAppData
	if lang == "ts" {
		encoreAppTemplate = tsEncoreAppData
	}
	if appSlug == "" {
		appSlugComments = strings.Join([]string{
			"",
			"The app is not currently linked to the encore.dev platform.",
			`Use "encore app link" to link it.`,
		}, "\n\t//")
	}
	encoreAppData := []byte(fmt.Sprintf(encoreAppTemplate, appSlugComments, appSlug))
	if err := xos.WriteFile("encore.app", encoreAppData, 0644); err != nil {
		return err
	}

	// Update to latest encore.dev release if this looks to be a go module.
	if _, err := os.Stat("go.mod"); err == nil {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Prefix = "Running go get encore.dev@latest"
		s.Start()
		if err := gogetEncore("."); err != nil {
			s.FinalMSG = fmt.Sprintf("failed, skipping: %v", err.Error())
		}
		s.Stop()
	}

	green := color.New(color.FgGreen)
	_, _ = green.Fprint(os.Stdout, "Successfully initialized application on Encore Cloud!\n")
	if appSlug == "" {
		_, _ = fmt.Fprintf(os.Stdout, "The app is not currently linked to the encore.dev platform.\n")
		_, _ = fmt.Fprintf(os.Stdout, "Use \"encore app link\" to link it.\n")
		return nil
	}
	_, _ = fmt.Fprintf(os.Stdout, "- App ID:          %s\n", cyan.Sprint(appSlug))
	_, _ = fmt.Fprintf(os.Stdout, "- Cloud Dashboard: %s\n\n", cyan.Sprintf("https://app.encore.cloud/%s", appSlug))

	return nil
}
