package cli

import (
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagNoColor bool
	flagQuiet   bool
	flagProject string
)

var rootCmd = &cobra.Command{
	Use:   "bifrost",
	Short: "Session context bridge between AI coding tools",
	Long:  "Bifrost transfers session context between AI coding tools. Write /handoff in one tool, /handin in another.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if flagNoColor {
			ui.NoColor = true
		}
		if flagQuiet {
			ui.Quiet = true
		}
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "Print only errors")
	rootCmd.PersistentFlags().StringVar(&flagProject, "project", "", "Use this directory as project root")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// ProjectFlag returns the --project flag value.
func ProjectFlag() string {
	return flagProject
}
