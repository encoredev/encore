package cmdutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
)

func AutoCompleteFromStaticList(args ...string) func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) (rtn []string, dir cobra.ShellCompDirective) {
		toComplete = strings.ToLower(toComplete)

		for _, option := range args {
			before, _, _ := strings.Cut(option, "\t")

			if strings.HasPrefix(before, toComplete) {
				rtn = append(rtn, option)
			}
		}

		return rtn, cobra.ShellCompDirectiveNoFileComp
	}
}

func AutoCompleteAppSlug(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// incase of not being logged in or an error, we give no auto competition
	_, err := conf.CurrentUser()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	apps, err := platform.ListApps(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	toComplete = strings.ToLower(toComplete)

	rtn := make([]string, 0, len(apps))
	for _, app := range apps {
		if strings.HasPrefix(strings.ToLower(app.Slug), toComplete) {
			desc := app.Description
			if desc == "" {
				desc = app.Name
			}

			rtn = append(rtn, fmt.Sprintf("%s\t%s", app.Slug, desc))
		}
	}

	return rtn, cobra.ShellCompDirectiveNoFileComp
}

func AutoCompleteEnvSlug(cmd *cobra.Command, args []string, toComplete string) (rtn []string, dir cobra.ShellCompDirective) {
	toComplete = strings.ToLower(toComplete)

	// Support the local environment
	if strings.HasPrefix("local", toComplete) {
		rtn = append(rtn, "local\tThis local development environment")
	}

	_, err := conf.CurrentUser()
	if err != nil {
		return rtn, cobra.ShellCompDirectiveError
	}

	// Assume the app slug is the first argument
	appSlug := args[len(args)-1]

	// Get the environments for the app and filter by what the user has already entered
	envs, err := platform.ListEnvs(cmd.Context(), appSlug)
	if err != nil {
		return rtn, cobra.ShellCompDirectiveError
	}

	for _, env := range envs {
		if strings.HasPrefix(strings.ToLower(env.Slug), toComplete) {
			rtn = append(rtn, fmt.Sprintf("%s\tA %s enviroment running on %s", env.Slug, env.Type, env.Cloud))
		}
	}

	return rtn, cobra.ShellCompDirectiveNoFileComp
}
