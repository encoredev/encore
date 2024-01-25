package k8s

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/k8s/types"
	"encr.dev/cli/internal/platform"
	"encr.dev/internal/conf"
	"encr.dev/pkg/xos"

	"sigs.k8s.io/yaml"
)

var configCmd = &cobra.Command{
	Use:   "configure --env=ENV_NAME",
	Short: "Updates your kubectl config to point to the Kubernetes cluster(s) for the specified environment",
	Run: func(cmd *cobra.Command, args []string) {
		appSlug := cmdutil.AppSlug()
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()

		if k8sEnvName == "" {
			_ = cmd.Help()
			cmdutil.Fatal("must specify environment name with --env")
		}

		err := configureForAppEnv(ctx, appSlug, k8sEnvName)
		if err != nil {
			cmdutil.Fatalf("error configuring kubectl: %v", err)
		}
	},
}

var (
	k8sEnvName string
)

func init() {
	configCmd.Flags().StringVarP(&k8sEnvName, "env", "e", "", "Environment name")
	_ = configCmd.MarkFlagRequired("env")
	kubernetesCmd.AddCommand(configCmd)
}

func configureForAppEnv(ctx context.Context, appID string, envName string) error {
	appSlug, envName, clusters, err := platform.KubernetesClusters(ctx, appID, envName)
	if err != nil {
		return errors.Wrap(err, "unable to get Kubernetes clusters for environment")
	}
	if len(clusters) == 0 {
		return errors.New("no Kubernetes clusters found for environment")
	}

	// Read the existing kubeconfig file
	configFilePath := filepath.Join(types.HomeDir(), ".kube", "config")
	cfg, err := readKubeConfig(configFilePath)
	if err != nil {
		return err
	}

	// Add the clusters
	contextPrefix := fmt.Sprintf("encore_%s_%s", appSlug, envName)
	authName := "encore-proxy-auth"
	contextNames := make([]string, len(clusters))
	for i, cluster := range clusters {
		// Create a context name for the cluster
		// by default we use the app slug and env name seperated by a underscore (e.g. encore-myapp_prod)
		// however if the environment has multiple clusters then we also include the cluster name (e.g. encore-myapp_prod_cluster1)
		contextName := contextPrefix
		if len(clusters) > 1 {
			contextName += "_" + cluster.Name
		}
		contextNames[i] = contextName

		// Add the cluster using the cluster name as the context name
		cfg.clusters = appendOrUpdate(cfg.clusters, map[string]any{
			"name": contextName,
			"cluster": map[string]any{
				"server": fmt.Sprintf("%s/k8s-api-proxy/%s/%s/", conf.APIBaseURL, cluster.EnvID, cluster.ResID),
			},
		})

		k8sContext := map[string]any{
			"cluster": contextName,
			"user":    authName,
		}
		if cluster.DefaultNamespace != "" {
			k8sContext["namespace"] = cluster.DefaultNamespace
		}

		cfg.contexts = appendOrUpdate(cfg.contexts, map[string]any{
			"name":    contextName,
			"context": k8sContext,
		})
	}

	// Remove any old contexts or clusters
	// We do this by iterating over the existing contexts and clusters and removing any that are not in the new list
	for i := len(cfg.contexts) - 1; i >= 0; i-- {
		if foundContext, ok := cfg.contexts[i].(map[string]any); ok {
			if contextName, ok := foundContext["name"].(string); ok {
				if strings.HasPrefix(contextName, contextPrefix) && !slices.Contains(contextNames, contextName) {
					cfg.contexts = append(cfg.contexts[:i], cfg.contexts[i+1:]...)
				}
			}
		}
	}
	for i := len(cfg.clusters) - 1; i >= 0; i-- {
		if foundCluster, ok := cfg.clusters[i].(map[string]any); ok {
			if clusterName, ok := foundCluster["name"].(string); ok {
				if strings.HasPrefix(clusterName, contextPrefix) && !slices.Contains(contextNames, clusterName) {
					cfg.clusters = append(cfg.clusters[:i], cfg.clusters[i+1:]...)
				}
			}
		}
	}

	// If we added a cluster then we need to update the encore-k8s-proxy user
	cfg.users = appendOrUpdate(cfg.users, map[string]any{
		"name": authName,
		"user": map[string]any{
			"exec": map[string]any{
				"apiVersion":         "client.authentication.k8s.io/v1",
				"args":               []string{"kubernetes", "exec-credentials"},
				"command":            "encore",
				"env":                nil,
				"installHint":        "Install encore for use with kubectl, see https://encore.dev",
				"interactiveMode":    "Never",
				"provideClusterInfo": false,
			},
		},
	})

	// Update the current context to the first cluster for the environment
	cfg.raw["current-context"] = contextNames[0]

	if err := writeKubeConfig(configFilePath, cfg); err != nil {
		return err
	}

	if len(clusters) == 1 {
		_, _ = fmt.Fprintf(os.Stdout, "kubectl configured for cluster %s under context %s.\n", color.CyanString(clusters[0].Name), color.CyanString(contextNames[0]))
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "kubectl configured for %d clusters:\n\n", len(clusters))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.StripEscape)
		_, _ = fmt.Fprint(w, "CLUSTER\tCONTEXT\tACTIVE\n")
		for i, cluster := range clusters {
			active := ""
			if i == 0 {
				active = "yes"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", cluster.Name, contextNames[0], active)
		}
		_ = w.Flush()
	}

	return nil
}

// readKubeConfig reads the existing kubeconfig file and returns a Cfg struct.
// however this is as untyped as possible, so that we can easily marshal it back without losing any data.
func readKubeConfig(file string) (*Cfg, error) {
	b, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, errors.Wrap(err, "unable to read kubeconfig file")
	}

	// Read the existing kubeconfig file
	var kubeConfig map[string]any
	if len(b) > 0 {
		if err = yaml.Unmarshal(b, &kubeConfig); err != nil {
			return nil, errors.Wrap(err, "unable to parse kubeconfig file")
		}
	}

	// Ensure the kubeConfig struct is valid
	if kubeConfig == nil {
		kubeConfig = map[string]any{
			"apiVersion": "v1",
			"kind":       "Config",
		}
	} else if kubeConfig["apiVersion"] != "v1" || kubeConfig["kind"] != "Config" {
		return nil, errors.New("invalid existing kubeconfig file")
	}
	cfg := &Cfg{
		raw: kubeConfig,
	}

	if clusters, ok := kubeConfig["clusters"]; ok {
		if clusters, ok := clusters.([]any); ok {
			cfg.clusters = clusters
		} else {
			return nil, errors.Newf("clusters is not an array got %T", clusters)
		}
	}

	if users, ok := kubeConfig["users"]; ok {
		if users, ok := users.([]any); ok {
			cfg.users = users
		} else {
			return nil, errors.Newf("users is not an array got %T", users)
		}
	}

	if contexts, ok := kubeConfig["contexts"]; ok {
		if contexts, ok := contexts.([]any); ok {
			cfg.contexts = contexts
		} else {
			return nil, errors.Newf("contexts is not an array got %T", contexts)
		}
	}

	return cfg, nil
}

// writeKubeConfig writes the kubeconfig back to the file.
func writeKubeConfig(file string, cfg *Cfg) error {
	// Update the raw kubeconfig struct
	cfg.raw["clusters"] = cfg.clusters
	cfg.raw["users"] = cfg.users
	cfg.raw["contexts"] = cfg.contexts

	b, err := yaml.Marshal(cfg.raw)
	if err != nil {
		return errors.Wrap(err, "unable to marshal kubeconfig back into yaml")
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return errors.Wrap(err, "unable to create kubeconfig directory")
	}

	// Then write the file
	err = xos.WriteFile(file, b, 0600)
	if err != nil {
		return errors.Wrap(err, "unable to write kubeconfig file")
	}
	return nil
}

type Cfg struct {
	raw      map[string]any
	clusters []any
	users    []any
	contexts []any
}

// appendOrUpdate looks at the array for an entry which is a map and has a "name" key which matches the name in val, if found
// it will update the entry with val, otherwise it will append val to the array.
func appendOrUpdate(dst []any, val map[string]any) []any {
	idx := slices.IndexFunc(dst, func(entry any) bool {
		if entry, ok := entry.(map[string]any); ok {
			if entry["name"] == val["name"] {
				return true
			}
		}
		return false
	})

	if idx == -1 {
		return append(dst, val)
	} else {
		dst[idx] = val
		return dst
	}
}
