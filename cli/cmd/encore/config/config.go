package encore

import (
	"context" // NEW: Required for the RequiredSecrets function
	"fmt"
	"os"
	"strings"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/internal/userconfig"
    // NEW: Import the internal package that defines the application structure
	"encr.dev/internal/app/appgraph" // HYPOTHETICAL: Replace with the actual internal app graph path
	"github.com/spf13/cobra"
)

var (
	forceApp, forceGlobal bool
	viewAllSettings       bool
)

var autoCompleteConfigKeys = cmdutil.AutoCompleteFromStaticList(userconfig.Keys()...)

var longDocs = `Gets or sets configuration values for customizing the behavior of the Encore CLI.

Configuration options can be set both for individual Encore applications,
as well as globally for the local user.

Configuration options can be set using ` + bt("encore config <key> <value>") + `,
and options can similarly be read using ` + bt("encore config <key>") + `.

When running ` + bt("encore config") + ` within an Encore application,
it automatically sets and gets configuration for that application.

To set or get global configuration, use the ` + bt("--global") + ` flag.

Available configuration settings are:

` + userconfig.CLIDocs()

var configCmd = &cobra.Command{
	Use:   "config <key> [<value>]",
	Short: "Get or set a configuration value",
	Long:  longDocs,
	Args:  cobra.RangeArgs(0, 2),

	Run: func(cmd *cobra.Command, args []string) {
		appRoot, _, _ := cmdutil.MaybeAppRoot()

		appScope := appRoot != ""
		if forceApp {
			appScope = true
		} else if forceGlobal {
			appScope = false
		}

		if appScope && appRoot == "" {
			// If the user specified --app, error if there is no app.
			cmdutil.Fatal(cmdutil.ErrNoEncoreApp)
		}

		if len(args) == 2 {
			var err error
			if appScope {
				err = userconfig.SetForApp(appRoot, args[0], args[1])
			} else {
				err = userconfig.SetGlobal(args[0], args[1])
			}
			if err != nil {
				cmdutil.Fatal(err)
			}
		} else {
			var (
				cfg *userconfig.Config
				err error
			)
			if appScope {
				appRoot, _ := cmdutil.AppRoot()
				cfg, err = userconfig.ForApp(appRoot).Get()
			} else {
				cfg, err = userconfig.Global().Get()
			}
			if err != nil {
				cmdutil.Fatal(err)
			}

			if viewAllSettings {
				if len(args) > 0 {
					cmdutil.Fatalf("cannot specify a settings key when using --all")
				}
				s := strings.TrimSuffix(cfg.Render(), "\n")
				fmt.Println(s)
				return
			}

			if len(args) == 0 {
				// No args are only allowed when --all is specified.
				_ = cmd.Usage()
				os.Exit(1)
			}

			val, ok := cfg.GetByKey(args[0])
			if !ok {
				cmdutil.Fatalf("unknown key %q", args[0])
			}
			fmt.Printf("%v\n", val)
		}
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			// Completing the first argument, the config key
			return autoCompleteConfigKeys(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	configCmd.Flags().BoolVar(&viewAllSettings, "all", false, "view all settings")
	configCmd.Flags().BoolVar(&forceApp, "app", false, "set the value for the current app")
	configCmd.Flags().BoolVar(&forceGlobal, "global", false, "set the value at the global level")
	configCmd.MarkFlagsMutuallyExclusive("app", "global")

	root.Cmd.AddCommand(configCmd)
}

// bt renders a backtick-enclosed string.
func bt(val string) string {
	return fmt.Sprintf("`%s`", val)
}


// RequiredSecrets loads the application graph and extracts all defined secret names.
// This function acts as the public API for the secret_check command.
//
// NOTE: This implementation is conceptual and relies on internal/app/appgraph
// having a function to load the app's parsed state.
func RequiredSecrets(ctx context.Context) ([]string, error) {
	appRoot, _, err := cmdutil.MaybeAppRoot()
	if err != nil || appRoot == "" {
		return nil, cmdutil.ErrNoEncoreApp
	}

	// Load the application graph to inspect its secrets requirements.
	// You will need to find the correct internal function to load the app graph.
	appGraph, err := appgraph.LoadCurrentApp(ctx, appRoot) // HYPOTHETICAL internal function call
	if err != nil {
		return nil, fmt.Errorf("failed to load application graph: %w", err)
	}

	// Extract unique secret names from the graph.
	secretNames := make(map[string]struct{})
	
	// This loop represents iterating over all service definitions in the graph 
	// and extracting the secret names defined in their var secrets struct{...}
	for _, service := range appGraph.Services() { // HYPOTHETICAL: Assuming AppGraph has a Services() method
		for _, secret := range service.Secrets() { // HYPOTHETICAL: Assuming Service has a Secrets() method
			secretNames[secret.Name] = struct{}{}
		}
	}

	// Convert map keys to a sorted slice.
	keys := make([]string, 0, len(secretNames))
	for k := range secretNames {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Ensure deterministic output

	return keys, nil
}