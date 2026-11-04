package cli

import (
	"os"

	"github.com/kogungor/bifrost/internal/project"
	"github.com/spf13/cobra"
)

// resolveProject returns the project root, using --project flag or auto-detection.
func resolveProject() (string, error) {
	if flagProject != "" {
		return flagProject, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, _, err := project.Root(cwd)
	return root, err
}

// showGroupHelp prints help for a command that only groups subcommands.
// Paired with cobra.NoArgs, it makes an unknown subcommand an error instead
// of silently printing help.
func showGroupHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
