package bits

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/pkg/bits"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists available Encore Bits to add to your application",
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		bits, err := bits.List(context.Background())
		if err != nil {
			cmdutil.Fatalf("could not list encore bits: %v", err)
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
		fmt.Fprintln(tw, "ID\tTitle\tDescription")
		for _, bit := range bits {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", bit.Slug, bit.Title, bit.Description)
			fmt.Fprintln(tw)
		}
		tw.Flush()
	},
}

func init() {
	bitsCmd.AddCommand(listCmd)
}
