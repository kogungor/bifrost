package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion script",
	Long: `Generate a shell completion script for bifrost.

To load completions:

Bash:
  $ source <(bifrost completion bash)
  # Or persist:
  $ bifrost completion bash > /etc/bash_completion.d/bifrost

Zsh:
  $ source <(bifrost completion zsh)
  # Or persist (adjust path for your system):
  $ bifrost completion zsh > "${fpath[1]}/_bifrost"

Fish:
  $ bifrost completion fish | source
  # Or persist:
  $ bifrost completion fish > ~/.config/fish/completions/bifrost.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
