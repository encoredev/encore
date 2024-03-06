package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/tailscale/hujson"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
	"encr.dev/pkg/xos"
)

var forceLink bool
var linkAppCmd = &cobra.Command{
	Use:   "link [app-id]",
	Short: "Link an Encore app with the server",
	Args:  cobra.MaximumNArgs(1),

	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var appID string
		if len(args) > 0 {
			appID = args[0]
		}
		linkApp(appID, forceLink)
	},
	ValidArgsFunction: cmdutil.AutoCompleteAppSlug,
}

func init() {
	appCmd.AddCommand(linkAppCmd)
	linkAppCmd.Flags().BoolVarP(&forceLink, "force", "f", false, "Force link even if the app is already linked.")
}

func linkApp(appID string, force bool) {
	// Determine the app root.
	root, _, err := cmdutil.MaybeAppRoot()
	if errors.Is(err, cmdutil.ErrNoEncoreApp) {
		root, err = os.Getwd()
	}
	if err != nil {
		cmdutil.Fatal(err)
	}

	filePath := filepath.Join(root, "encore.app")
	data, err := os.ReadFile(filePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		cmdutil.Fatal(err)
		os.Exit(1)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		// Treat missing and empty files as an empty object.
		data = []byte("{}")
	}

	val, err := hujson.Parse(data)
	if err != nil {
		cmdutil.Fatal("could not parse encore.app: ", err)
	}

	appData, ok := val.Value.(*hujson.Object)
	if !ok {
		cmdutil.Fatal("could not parse encore.app: expected JSON object")
	}

	// Find the "id" value, if any.
	var idValue *hujson.Value
	for i := 0; i < len(appData.Members); i++ {
		kv := &appData.Members[i]
		lit, ok := kv.Name.Value.(hujson.Literal)
		if !ok || lit.String() != "id" {
			continue
		}
		idValue = &kv.Value
	}

	if idValue != nil {
		val, ok := idValue.Value.(hujson.Literal)
		if ok && val.String() != "" && val.String() != appID && !force {
			cmdutil.Fatal("the app is already linked.\n\nNote: to link to a different app, specify the --force flag.")
		}
	}

	if appID == "" {
		// The app is not linked. Prompt the user for an app ID.
		fmt.Println("Make sure the app is created on app.encore.dev, and then enter its ID to link it.")
		fmt.Print("App ID: ")
		if _, err := fmt.Scanln(&appID); err != nil {
			cmdutil.Fatal(err)
		} else if appID == "" {
			cmdutil.Fatal("no app id given.")
		}
	}

	if linked, err := validateAppSlug(appID); err != nil {
		cmdutil.Fatal(err)
	} else if !linked {
		fmt.Fprintln(os.Stderr, "Error: that app does not exist, or you don't have access to it.")
		os.Exit(1)
	}

	// Write it back to our data structure.
	if idValue != nil {
		idValue.Value = hujson.String(appID)
	} else {
		appData.Members = append(appData.Members, hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String("id")},
			Value: hujson.Value{Value: hujson.String(appID)},
		})
	}

	val.Format()
	if err := xos.WriteFile(filePath, val.Pack(), 0644); err != nil {
		cmdutil.Fatal(err)
		os.Exit(1)
	}

	addEncoreRemote(root, appID)
	fmt.Println("Successfully linked app!")
}

func validateAppSlug(slug string) (ok bool, err error) {
	if _, err := conf.CurrentUser(); errors.Is(err, fs.ErrNotExist) {
		cmdutil.Fatal("not logged in. Run 'encore auth login' first.")
	} else if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := platform.GetApp(ctx, slug); err != nil {
		var e platform.Error
		if errors.As(err, &e) && e.HTTPCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
