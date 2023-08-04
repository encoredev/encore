package namespace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/cli/cmd/encore/root"
	daemonpb "encr.dev/proto/encore/daemon"
)

var nsCmd = &cobra.Command{
	Use:     "namespace",
	Short:   "Manage infrastructure namespaces",
	Aliases: []string{"ns"},
}

func init() {
	output := cmdutil.Output{Value: "columns", Allowed: []string{"columns", "json"}}
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List infrastructure namespaces",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			appRoot, _ := cmdutil.AppRoot()
			daemon := cmdutil.ConnectDaemon(ctx)
			resp, err := daemon.ListNamespaces(ctx, &daemonpb.ListNamespacesRequest{AppRoot: appRoot})
			if err != nil {
				cmdutil.Fatal(err)
			}
			nss := resp.Namespaces

			// Sort by active first, then name second.
			slices.SortFunc(nss, func(a, b *daemonpb.Namespace) bool {
				if a.Active != b.Active {
					return a.Active
				}
				return a.Name < b.Name
			})

			if output.Value == "json" {
				var buf bytes.Buffer
				buf.WriteByte('[')
				for i, ns := range nss {
					data, err := protojson.MarshalOptions{
						UseProtoNames:   true,
						EmitUnpopulated: true,
					}.Marshal(ns)
					if err != nil {
						cmdutil.Fatal(err)
					}
					if i > 0 {
						buf.WriteByte(',')
					}
					buf.Write(data)
				}
				buf.WriteByte(']')

				var dst bytes.Buffer
				if err := json.Indent(&dst, buf.Bytes(), "", "  "); err != nil {
					cmdutil.Fatal(err)
				}
				fmt.Fprintln(os.Stdout, dst.String())
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.StripEscape)

			_, _ = fmt.Fprint(w, "NAME\tID\tACTIVE\n")

			for _, ns := range nss {
				active := ""
				if ns.Active {
					active = "yes"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", ns.Name, ns.Id, active)
			}
			_ = w.Flush()
		},
	}
	output.AddFlag(listCmd.Flags())

	nsCmd.AddCommand(listCmd)
}

var createCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "Create a new infrastructure namespace",

	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		appRoot, _ := cmdutil.AppRoot()
		daemon := cmdutil.ConnectDaemon(ctx)
		ns, err := daemon.CreateNamespace(ctx, &daemonpb.CreateNamespaceRequest{
			AppRoot: appRoot,
			Name:    args[0],
		})
		if err != nil {
			cmdutil.Fatal(err)
		}
		fmt.Fprintf(os.Stdout, "created namespace %s\n", ns.Name)
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete NAME",
	Short:   "Delete an infrastructure namespace",
	Aliases: []string{"del"},

	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: namespaceListCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		appRoot, _ := cmdutil.AppRoot()
		daemon := cmdutil.ConnectDaemon(ctx)
		name := args[0]
		_, err := daemon.DeleteNamespace(ctx, &daemonpb.DeleteNamespaceRequest{
			AppRoot: appRoot,
			Name:    name,
		})
		if err != nil {
			cmdutil.Fatal(err)
		}
		fmt.Fprintf(os.Stdout, "deleted namespace %s\n", name)
	},
}

func init() {
	var create bool
	switchCmd := &cobra.Command{
		Use:   "switch [--create] NAME",
		Short: "Switch to a different infrastructure namespace",
		Long: `Switch to a specified infrastructure namespace. Subsequent commands will use the given namespace by default.

If -c is specified, the namespace will first be created before switching to it.

You can use '-' as the namespace name to switch back to the previously active namespace.
`,

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: namespaceListCompletion,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			appRoot, _ := cmdutil.AppRoot()
			daemon := cmdutil.ConnectDaemon(ctx)
			ns, err := daemon.SwitchNamespace(ctx, &daemonpb.SwitchNamespaceRequest{
				AppRoot: appRoot,
				Name:    args[0],
				Create:  create,
			})
			if err != nil {
				cmdutil.Fatal(err)
			}
			fmt.Fprintf(os.Stdout, "switched to namespace %s\n", ns.Name)
		},
	}

	switchCmd.Flags().BoolVarP(&create, "create", "c", false, "create the namespace before switching")
	nsCmd.AddCommand(switchCmd)
}

func init() {
	nsCmd.AddCommand(createCmd)
	nsCmd.AddCommand(deleteCmd)
	root.Cmd.AddCommand(nsCmd)
}

func namespaceListCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// List namespaces from the daemon for completion.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	appRoot, _ := cmdutil.AppRoot()
	daemon := cmdutil.ConnectDaemon(ctx)
	resp, err := daemon.ListNamespaces(ctx, &daemonpb.ListNamespacesRequest{AppRoot: appRoot})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	namespaces := make([]string, len(resp.Namespaces))
	for i, ns := range resp.Namespaces {
		namespaces[i] = ns.Name
	}
	return namespaces, cobra.ShellCompDirectiveNoFileComp
}
