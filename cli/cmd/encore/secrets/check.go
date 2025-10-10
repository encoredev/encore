package secrets

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/internal/platform"
	"encr.dev/cli/internal/platform/gql"
)

var checkSecretCmd = &cobra.Command{
	Use:   "check [envs...]",
	Short: "Check if secrets are properly set across environments",
	Long: `Check if secrets are properly set across specified environments.
This command validates that all secrets required by your application
are configured in the specified environments.

Example usage:
  encore secret check prod dev
  encore secret check production development`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmdutil.Fatal("at least one environment must be specified")
		}

		appSlug := cmdutil.AppSlug()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get all secrets for the app
		secrets, err := platform.ListSecretGroups(ctx, appSlug, nil)
		if err != nil {
			cmdutil.Fatal(err)
		}

		// Normalize environment names
		envNames := normalizeEnvNames(args)
		
		// Check secrets across environments
		result := CheckSecretsAcrossEnvs(secrets, envNames)
		
		// Display results
		DisplayCheckResults(result, envNames)
		
		// Exit with error if any secrets are missing
		if result.HasMissing {
			os.Exit(1)
		}
	},
}

type CheckResult struct {
	SecretResults []SecretCheckResult
	HasMissing    bool
}

type SecretCheckResult struct {
	Key     string
	EnvStatus map[string]bool // env name -> has secret
}

func normalizeEnvNames(args []string) []string {
	var normalized []string
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "prod", "production":
			normalized = append(normalized, "production")
		case "dev", "development":
			normalized = append(normalized, "development")
		case "local":
			normalized = append(normalized, "local")
		case "preview":
			normalized = append(normalized, "preview")
		default:
			// Assume it's a specific environment name
			normalized = append(normalized, arg)
		}
	}
	return normalized
}

func CheckSecretsAcrossEnvs(secrets []*gql.Secret, envNames []string) CheckResult {
	var result CheckResult
	
	for _, secret := range secrets {
		secretResult := SecretCheckResult{
			Key:       secret.Key,
			EnvStatus: make(map[string]bool),
		}
		
		// Initialize all environments as missing
		for _, envName := range envNames {
			secretResult.EnvStatus[envName] = false
		}
		
		// Check which environments have this secret
		for _, group := range secret.Groups {
			if group.ArchivedAt != nil {
				continue // Skip archived secrets
			}
			
			for _, selector := range group.Selector {
				switch sel := selector.(type) {
				case *gql.SecretSelectorEnvType:
					if contains(envNames, sel.Kind) {
						secretResult.EnvStatus[sel.Kind] = true
					}
				case *gql.SecretSelectorSpecificEnv:
					if contains(envNames, sel.Env.Name) {
						secretResult.EnvStatus[sel.Env.Name] = true
					}
				}
			}
		}
		
		// Check if any environment is missing this secret
		for _, hasSecret := range secretResult.EnvStatus {
			if !hasSecret {
				result.HasMissing = true
				break
			}
		}
		
		result.SecretResults = append(result.SecretResults, secretResult)
	}
	
	return result
}

func DisplayCheckResults(result CheckResult, envNames []string) {
	if len(result.SecretResults) == 0 {
		fmt.Println("No secrets found.")
		return
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', tabwriter.StripEscape)

	// Header
	header := "Secret Key"
	for _, envName := range envNames {
		header += fmt.Sprintf("\t%s", strings.Title(envName))
	}
	header += "\t\n"
	fmt.Fprint(w, header)

	const (
		checkYes = "\u2713"
		checkNo  = "\u2717"
	)

	missingCount := 0
	
	// Sort secrets by key for consistent output
	slices.SortFunc(result.SecretResults, func(a, b SecretCheckResult) int {
		return strings.Compare(a.Key, b.Key)
	})

	for _, secretResult := range result.SecretResults {
		line := secretResult.Key
		secretHasMissing := false
		
		for _, envName := range envNames {
			hasSecret := secretResult.EnvStatus[envName]
			if hasSecret {
				line += fmt.Sprintf("\t%s", checkYes)
			} else {
				line += fmt.Sprintf("\t%s", checkNo)
				secretHasMissing = true
			}
		}
		line += "\t\n"
		
		if secretHasMissing {
			missingCount++
		}
		
		fmt.Fprint(w, line)
	}

	w.Flush()

	// Add color to the checkmarks
	r := strings.NewReplacer(checkYes, color.GreenString(checkYes), checkNo, color.RedString(checkNo))
	r.WriteString(os.Stdout, buf.String())

	// Print summary
	if result.HasMissing {
		if missingCount == 1 {
			fmt.Printf("\nError: There is 1 secret missing.\n")
		} else {
			fmt.Printf("\nError: There are %d secrets missing.\n", missingCount)
		}
	} else {
		fmt.Printf("\nAll secrets are properly configured across specified environments.\n")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func init() {
	secretCmd.AddCommand(checkSecretCmd)
}