package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kogungor/bifrost/internal/adapters"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

// CommandsFS holds the embedded slash command files. Must be set by main before Execute().
var CommandsFS embed.FS

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Register slash commands for installed AI tools",
	RunE:  runInstall,
}

var (
	installAdapterFlag string
	installForce       bool
	installDryRun      bool
)

func init() {
	installCmd.Flags().StringVar(&installAdapterFlag, "adapter", "", "Install for a specific adapter only (claude-code, opencode)")
	installCmd.Flags().BoolVar(&installForce, "force", false, "Overwrite existing command files")
	installCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "Print what would happen without doing it")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	var targets []adapters.Adapter

	if installAdapterFlag != "" {
		a := adapters.Get(installAdapterFlag)
		if a == nil {
			ui.Error(fmt.Sprintf("Unknown adapter: %s", installAdapterFlag), "Available: claude-code, opencode")
			return fmt.Errorf("unknown adapter")
		}
		targets = append(targets, a)
	} else {
		targets = adapters.All()
	}

	installed := 0
	for _, a := range targets {
		if !a.IsInstalled() {
			ui.Warning(fmt.Sprintf("%s  not detected", a.DisplayName()))
			continue
		}

		if err := installAdapterCommands(a); err != nil {
			ui.Error(fmt.Sprintf("%s  failed", a.DisplayName()), err.Error())
			continue
		}
		installed++
	}

	if installed == 0 && installAdapterFlag == "" {
		ui.Blank()
		ui.Warning("No AI tools detected.")
		ui.Dim("Install Claude Code or OpenCode, then run 'bifrost install' again.")
	}

	return nil
}

func installAdapterCommands(a adapters.Adapter) error {
	cmdsDir := a.CommandsDir()
	adapterName := a.Name()

	if installDryRun {
		ui.Section(a.DisplayName(), fmt.Sprintf("would install to %s", cmdsDir))
		return nil
	}

	if err := os.MkdirAll(cmdsDir, 0755); err != nil {
		return err
	}

	for _, name := range []string{"handoff.md", "handin.md"} {
		src := fmt.Sprintf("commands/%s/%s", adapterName, name)
		data, err := CommandsFS.ReadFile(src)
		if err != nil {
			return fmt.Errorf("embedded command %s: %w", src, err)
		}

		dest := filepath.Join(cmdsDir, name)

		if !installForce {
			if _, err := os.Stat(dest); err == nil {
				ui.Dim(fmt.Sprintf("  %s already exists, skipping (use --force to overwrite)", dest))
				continue
			}
		}

		if err := os.WriteFile(dest, data, 0644); err != nil {
			return err
		}
	}

	ui.Success(fmt.Sprintf("%s  commands registered  (%s)", a.DisplayName(), cmdsDir))
	return nil
}
