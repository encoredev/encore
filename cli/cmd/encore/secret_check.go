package encore

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"encore.dev/cli/pkg/config" // Hypothetical: Adjust actual import path for config/environment
	"encore.dev/cli/pkg/log"    // Hypothetical: Adjust actual import path for logging
	"encore.dev/internal/app"   // Hypothetical: Adjust actual import path for app logic
	"encore.dev/pkg/secrets"    // Hypothetical: Adjust actual import path for secrets logic
)

// secretCheckCmd represents the "encore secret check" command
var secretCheckCmd = &cobra.Command{
	Use:   "check [ENV_NAME...]",
	Short: "Checks which required secrets are missing in the specified environments.",
	Long: `Checks which required secrets are missing in the specified environments.

If no environment names are provided, it defaults to checking:
- The current local environment.
- The default development environment ("dev").
- The default production environment ("prod").

Example:
  encore secret check prod staging
`,
	Args: cobra.ArbitraryArgs, // Allows 0 or more environment names
	Run:  wrapRun(runSecretCheck),
}

// init adds the check command to the secret command (which is assumed to exist).
func init() {
	// secretCmd is assumed to be defined in cli/cmd/encore/secret.go (which must be edited/created)
	// For this exercise, we define it locally for context:
	secretCmd.AddCommand(secretCheckCmd)
}

func runSecretCheck(ctx context.Context, cmd *cobra.Command, args []string) error {
	// 1. Determine target environments
	targetEnvs := args
	if len(targetEnvs) == 0 {
		// Default environments if none are specified
		targetEnvs = []string{"local", "dev", "prod"}
	}
	
	// Ensure unique environment names and sort for deterministic output
	envSet := make(map[string]struct{})
	for _, env := range targetEnvs {
		envSet[env] = struct{}{}
	}
	targetEnvs = make([]string, 0, len(envSet))
	for env := range envSet {
		targetEnvs = append(targetEnvs, env)
	}
	sort.Strings(targetEnvs)

	// 2. Load and parse application requirements
	// NOTE: This function (app.RequiredSecrets) is hypothetical and MUST be implemented
	// in an existing Encore core package (e.g., internal/app).
	requiredSecrets, err := app.RequiredSecrets(ctx) 
	if err != nil {
		return fmt.Errorf("could not determine required secrets: %w", err)
	}

	if len(requiredSecrets) == 0 {
		log.Println("✅ Your application does not require any secrets.")
		return nil
	}

	// 3. Check secret status for each required secret in each environment
	var missingSecrets []SecretStatus

	for _, secretName := range requiredSecrets {
		for _, envName := range targetEnvs {
			// NOTE: This function (secrets.CheckEnvironmentStatus) is hypothetical and MUST be 
			// implemented in an existing Encore core package (e.g., pkg/secrets).
			isSet, err := secrets.CheckEnvironmentStatus(ctx, secretName, envName) 
			if err != nil {
				// Log the error but continue checking other secrets/envs
				log.Printf("Warning: Could not check status for secret %q in environment %q: %v\n", secretName, envName, err)
				continue
			}

			if !isSet {
				missingSecrets = append(missingSecrets, SecretStatus{
					SecretName: secretName,
					Environment: envName,
				})
			}
		}
	}

	// 4. Report results
	if len(missingSecrets) == 0 {
		log.Printf("✅ All %d required secrets are set for environments: %s\n", len(requiredSecrets), strings.Join(targetEnvs, ", "))
		return nil
	}

	fmt.Printf("❌ Found %d missing secrets in the specified environments:\n\n", len(missingSecrets))

	// Create a tabwriter for clean table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SECRET\tENVIRONMENT")
	fmt.Fprintln(w, "------\t-----------")

	// Sort missing secrets for consistent output
	sort.Slice(missingSecrets, func(i, j int) bool {
		if missingSecrets[i].SecretName != missingSecrets[j].SecretName {
			return missingSecrets[i].SecretName < missingSecrets[j].SecretName
		}
		return missingSecrets[i].Environment < missingSecrets[j].Environment
	})

	for _, status := range missingSecrets {
		fmt.Fprintf(w, "%s\t%s\n", status.SecretName, status.Environment)
	}

	w.Flush()

	return fmt.Errorf("\nSecret check failed: %d secrets are missing. Use 'encore secret set' to provide them.", len(missingSecrets))
}

// SecretStatus holds the status of a single secret in a single environment.
type SecretStatus struct {
	SecretName  string
	Environment string
}

// NOTE: You would also need to update a file like `cli/cmd/encore/secret.go` 
// to include the `secretCmd` definition and the `init` function call.