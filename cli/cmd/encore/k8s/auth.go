package k8s

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/k8s/types"
	"encr.dev/internal/conf"
)

var genAuthCmd = &cobra.Command{
	Use:                   "exec-credentials",
	Short:                 "Used by kubectl to get an authentication token for the Encore Kubernetes Proxy",
	Args:                  cobra.NoArgs,
	Hidden:                true,
	DisableFlagsInUseLine: true,
	Run:                   func(cmd *cobra.Command, args []string) { generateExecCredentials() },
}

func init() {
	kubernetesCmd.AddCommand(genAuthCmd)
}

// GenerateExecCredentials generates the Kubernetes exec credentials and writes them to stdout.
//
// If an error occurs, it is written to stderr and the program exits with a non-zero exit code.
func generateExecCredentials() {
	// Get the OAuth token from the Encore API
	token, err := conf.DefaultTokenSource.Token()
	if err != nil {
		cmdutil.Fatalf("error getting token: %v", err)
	}

	// Generate the kuberentes exec credentials datastructures
	expiryTime := types.NewTime(token.Expiry)
	execCredentials := &types.ExecCredential{
		TypeMeta: types.TypeMeta{
			APIVersion: "client.authentication.k8s.io/v1",
			Kind:       "ExecCredential",
		},
		Status: &types.ExecCredentialStatus{
			Token:               token.AccessToken,
			ExpirationTimestamp: &expiryTime,
		},
	}

	// Marshal the exec credentials to JSON and write to stdout
	output, err := json.MarshalIndent(execCredentials, "", "  ")
	if err != nil {
		cmdutil.Fatalf("error marshalling exec credentials: %v", err)
	}
	_, _ = os.Stdout.Write(output)
}
