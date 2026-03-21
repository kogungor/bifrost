package cli

import (
	"github.com/kog/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Plain("bifrost " + Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
