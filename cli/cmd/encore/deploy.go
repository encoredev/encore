package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"encr.dev/internal/conf"
	"encr.dev/internal/urlutil"

	"github.com/cockroachdb/errors"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/pkg/appfile"
)

var (
	appSlug string
	envName string
	commit  string
	branch  string
	format  = cmdutil.Oneof{
		Value:     "text",
		Allowed:   []string{"text", "json"},
		Flag:      "format",
		FlagShort: "f",
		Desc:      "Output format",
	}
)

var deployAppCmd = &cobra.Command{
	Use:                   "deploy --commit COMMIT_SHA | --branch BRANCH_NAME",
	Short:                 "Deploy an Encore app to a cloud environment",
	DisableFlagsInUseLine: true,
	Run: func(c *cobra.Command, args []string) {
		if commit != "" {
			hb, err := hex.DecodeString(commit)
			if err != nil || len(hb) != 20 {
				cmdutil.Fatalf("invalid commit: %s", commit)
			}
		}
		if appSlug == "" {
			appRoot, _, err := cmdutil.MaybeAppRoot()
			if err != nil {
				cmdutil.Fatalf("no app found. Run deploy inside an encore app directory or specify the app with --app")
			}
			appSlug, err = appfile.Slug(appRoot)
			if err != nil {
				cmdutil.Fatalf("no app found. Run deploy inside an encore app directory or specify the app with --app")
			}
		}
		rollout, err := platform.Deploy(c.Context(), appSlug, envName, commit, branch)
		var pErr platform.Error
		if ok := errors.As(err, &pErr); ok {
			switch pErr.Code {
			case "app_not_found":
				cmdutil.Fatalf("app not found: %s", appSlug)
			case "validation":
				var details platform.ValidationDetails
				err := json.Unmarshal(pErr.Detail, &details)
				if err != nil {
					cmdutil.Fatalf("failed to deploy: %v", err)
				}
				switch details.Field {
				case "commit":
					cmdutil.Fatalf("could not find commit: %s. Is it pushed to the remote repository?", commit)
				case "branch":
					cmdutil.Fatalf("could not find branch: %s. Is it pushed to the remote repository?", branch)
				case "env":
					cmdutil.Fatalf("could not find environment: %s/%s", appSlug, envName)
				}
			}
		}
		if err != nil {
			cmdutil.Fatalf("failed to deploy: %v", err)
		}
		rel := fmt.Sprintf("/%s/deploys/%s/%s", appSlug, rollout.EnvName, strings.TrimPrefix(rollout.ID, "roll_"))
		url := urlutil.JoinURL(conf.WebDashBaseURL(), rel)
		switch format.Value {
		case "text":
			fmt.Println(aurora.Sprintf("\n%s %s\n", aurora.Bold("Started Deploy:"), url))
		case "json":
			output, _ := json.Marshal(map[string]string{
				"id":  strings.TrimPrefix(rollout.ID, "roll_"),
				"env": rollout.EnvName,
				"app": appSlug,
				"url": url,
			})
			fmt.Println(string(output))
		}
	},
}

func init() {
	alphaCmd.AddCommand(deployAppCmd)
	deployAppCmd.Flags().StringVar(&appSlug, "app", "", "app slug to deploy to (default current app)")
	deployAppCmd.Flags().StringVarP(&envName, "env", "e", "", "environment to deploy to (default primary env)")
	deployAppCmd.Flags().StringVar(&commit, "commit", "", "commit to deploy")
	deployAppCmd.Flags().StringVar(&branch, "branch", "", "branch to deploy")
	format.AddFlag(deployAppCmd)
	_ = deployAppCmd.MarkFlagRequired("env")
	deployAppCmd.MarkFlagsMutuallyExclusive("commit", "branch")
	deployAppCmd.MarkFlagsOneRequired("commit", "branch")
}
