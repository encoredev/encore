package config

import (
	"fmt"
	"os"
	"strings"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	"encr.dev/internal/userconfig"
	"github.com/spf13/cobra"
)

var (
	forceApp, forceGlobal bool
	viewAllSettings       bool
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
	Use:   "config <key> [<value>]",
	Short: "Get or set a configuration value",
	Long:  longDocs,
	Args:  cobra.RangeArgs(0, 2),

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
