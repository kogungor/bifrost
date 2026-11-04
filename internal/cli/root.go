package cli

import (
	"errors"
	"fmt"

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
	Long:  "Bifrost carries AI coding context across tools with local integrity checks.\n\nWrite /handoff in one tool, /handin --verify in another, and continue with a briefing that separates observed repo facts from model interpretation. The snapshot carries task state, active files, decisions, evidence, trust signals, risks, open questions, and plan health.\n\nIntegrity commands such as verify, brief, diff, scrub, context, and timeline help a new agent know what to trust, what to verify first, and what not to assume.\n\nCreate implementation plans with /plan. Get a critical review from another AI with /review.\n\nQuick start:\n  1. bifrost install --mcp     register the commands and the MCP server for every detected tool\n  2. bifrost init              set up the project (BIFROST.md, .gitignore)\n  3. /handoff                  in your AI tool when its context is full\n  4. /handin --verify          in the next tool, for a trust-aware briefing\n\n  In Codex the commands are $bifrost-handoff and $bifrost-handin.\n\n  bifrost install --hooks     record Claude Code sessions automatically (optional)\n  bifrost verify              check whether the handoff is still trustworthy\n  bifrost doctor              verify everything is configured correctly",
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
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("bifrost {{.Version}}\n")
}

// Execute runs the root command. Commands report their own failures through
// ui.Error; anything they return unreported (unknown commands, bad flags,
// bare errors) is printed here so the CLI never fails silently.
func Execute() error {
	cmd, err := rootCmd.ExecuteC()
	if err == nil || ui.ErrorShown() {
		return err
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) {
		return err
	}
	ui.Error(err.Error(), fmt.Sprintf("Run '%s --help' for usage.", cmd.CommandPath()))
	return err
}

// ProjectFlag returns the --project flag value.
func ProjectFlag() string {
	return flagProject
}
