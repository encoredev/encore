package bits

import (
	"context"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/pkg/bits"
)

var addCmd = &cobra.Command{
	Use:   "add <name> [<path>]",
	Short: "Add an Encore Bit to your application",
	Args:  cobra.MinimumNArgs(1),

	DisableFlagsInUseLine: true,
	Run: func(c *cobra.Command, args []string) {
		slug := args[0]
		ctx := context.Background()
		bit, err := bits.Get(ctx, slug)
		if errors.Is(err, errBitNotFound) {
			cmdutil.Fatalf("encore bit not found: %s", slug)
		} else if err != nil {
			cmdutil.Fatalf("could not lookup encore bit: %v", err)
		}

		workdir, err := os.MkdirTemp("", "encore-bit")
		if err != nil {
			cmdutil.Fatal(err)
		}
		defer os.RemoveAll(workdir)

		//prefix := args[0]
		//if len(args) > 1 {
		//	prefix = args[1]
		//}

		fmt.Fprintf(os.Stderr, "Downloading Encore Bit: %s\n", bit.Title)
		if err := bits.Extract(ctx, bit, workdir); err != nil {
			cmdutil.Fatalf("download failed: %v", err)
		}

		meta, err := bits.Describe(ctx, workdir)
		if err != nil {
			cmdutil.Fatalf("could not parse bit metadata: %v", err)
		}

		fmt.Fprintf(os.Stderr, "successfully got bit: %+v\n", meta)

		//fmt.Fprintf(os.Stderr, "\n\nSuccessfully added Encore Bit: %s!\n", bit.Title)
		//fmt.Fprintf(os.Stderr, "You can find the new bit under the %s/ directory.\n", prefix)
	},
}

func init() {
	bitsCmd.AddCommand(addCmd)
}
