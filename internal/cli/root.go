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
	Long: "Bifrost transfers session context between AI coding tools.\n\nWrite /handoff in one tool, /handin in another — the snapshot carries task state, active files, decisions, environment notes, and trust signals (intent, assumptions, open questions, risks).\n\nCreate implementation plans with /plan. Get a critical review from another AI with /review.\n\nQuick start:\n  1. bifrost install          register /handoff, /handin, /plan, /review\n  2. /handoff                 run in your AI tool when context is full\n  3. Switch tools\n  4. /handin                  load the snapshot in your new session\n\n  bifrost install --mcp       enable MCP server for richer handoffs (recommended)\n  bifrost init                set up a project with BIFROST.md\n  bifrost doctor              verify everything is configured correctly",
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
