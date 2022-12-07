package root

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"encr.dev/pkg/errlist"
)

var verbosity int

var Cmd = &cobra.Command{
	Use:           "encore",
	Short:         "encore is the fastest way of developing backend applications",
	SilenceErrors: true, // We'll handle displaying an error in our main func
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true, // Hide the "completion" command from help (used for generating auto-completions for the shell)
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := zerolog.InfoLevel
		if verbosity == 1 {
			level = zerolog.DebugLevel
		} else if verbosity >= 2 {
			level = zerolog.TraceLevel
		}

		if verbosity >= 1 {
			errlist.Verbose = true
		}
		log.Logger = log.Logger.Level(level)
	},
}

func init() {
	Cmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbose output")
}
