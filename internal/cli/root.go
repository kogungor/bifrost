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
	Short: "Local, trust-aware handoffs for AI coding agents",
	Long:  "Bifrost carries AI coding context across tools with local integrity checks.\n\nWrite /handoff in one tool, /handin --verify in another, and continue with a briefing that separates observed repo facts from model interpretation. The snapshot carries task state, active files, decisions, evidence, trust signals, risks, open questions, and plan health.\n\nIntegrity commands such as verify, brief, diff, scrub, context, and timeline help a new agent know what to trust, what to verify first, and what not to assume.\n\nCreate implementation plans with /plan. Get a critical review from another AI with /review.\n\nQuick start:\n  1. bifrost install          register /handoff, /handin, /plan, /review\n  2. /handoff                 run in your AI tool when context is full\n  3. Switch tools\n  4. /handin --verify         load a trust-aware briefing in your new session\n\n  bifrost install --mcp       enable MCP server for richer handoffs (recommended)\n  bifrost init                set up a project with BIFROST.md\n  bifrost verify              check whether the handoff is still trustworthy\n  bifrost doctor              verify everything is configured correctly",
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
