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
	Short: "Register Bifrost commands, and optionally the MCP server and hooks, for detected AI tools",
	RunE:  runInstall,
}

var (
	installAdapterFlag string
	installForce       bool
	installDryRun      bool
	installMCP         bool
	installHooks       bool
)

func init() {
	installCmd.Flags().StringVar(&installAdapterFlag, "adapter", "", "Install for a specific adapter only (claude-code, opencode, codex)")
	installCmd.Flags().BoolVar(&installForce, "force", false, "Overwrite existing command files (run after upgrading Bifrost)")
	installCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "Print what would happen without doing it")
	installCmd.Flags().BoolVar(&installMCP, "mcp", false, "Also register Bifrost as an MCP server")
	installCmd.Flags().BoolVar(&installHooks, "hooks", false, "Also register lifecycle hooks for automatic handoff (Claude Code)")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	var targets []adapters.Adapter

	if installAdapterFlag != "" {
		a := adapters.Get(installAdapterFlag)
		if a == nil {
			ui.Error(fmt.Sprintf("Unknown adapter: %s", installAdapterFlag), "Available: claude-code, opencode, codex")
			return fmt.Errorf("unknown adapter")
		}
		targets = append(targets, a)
	} else {
		targets = adapters.All()
	}

	installed, failed := 0, 0
	for _, a := range targets {
		if !a.IsInstalled() {
			ui.Warning(fmt.Sprintf("%s  not detected", a.DisplayName()))
			continue
		}

		if err := installAdapterCommands(a); err != nil {
			ui.Error(fmt.Sprintf("%s  failed", a.DisplayName()), err.Error())
			failed++
			continue
		}
		installed++
	}

	if installed == 0 && installAdapterFlag == "" {
		ui.Blank()
		ui.Warning("No AI tools detected.")
		ui.Dim("Install Claude Code, OpenCode, or Codex, then run 'bifrost install' again.")
	}

	if installMCP {
		for _, a := range targets {
			if !a.IsInstalled() {
				continue
			}
			mcpPath := a.MCPConfigPath()
			if mcpPath == "" {
				continue
			}
			if err := installMCPConfig(a, mcpPath); err != nil {
				ui.Error(fmt.Sprintf("%s  MCP registration failed", a.DisplayName()), err.Error())
				failed++
			}
		}
	}

	if installHooks {
		for _, a := range targets {
			if !a.IsInstalled() {
				continue
			}
			h, ok := a.(adapters.HookInstaller)
			if !ok {
				ui.Dim(fmt.Sprintf("  %s has no lifecycle hooks; skipped", a.DisplayName()))
				continue
			}
			if err := installHooksConfig(a.DisplayName(), h); err != nil {
				ui.Error(fmt.Sprintf("%s  hook registration failed", a.DisplayName()), err.Error())
				failed++
			}
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d install step(s) failed", failed)
	}
	if installed > 0 && !installDryRun {
		printInstallNextSteps()
	}
	return nil
}

// printInstallNextSteps tells a first-time user what to do after install.
func printInstallNextSteps() {
	ui.Blank()
	ui.Plain("Next steps:")
	ui.Plain("  1. Restart your AI tools so they load the new commands.")
	ui.Plain("  2. In a project, run 'bifrost init' once.")
	ui.Plain("  3. Run /handoff in one tool and /handin --verify in the next.")
	if !installMCP {
		ui.Dim("Recommended: 'bifrost install --mcp' for structured, verifiable handoffs.")
	}
	if !installHooks {
		if a := adapters.Get("claude-code"); a != nil && a.IsInstalled() {
			ui.Dim("Optional: 'bifrost install --hooks' records Claude Code sessions automatically.")
		}
	}
}

// bifrostCommands are the commands installed for every adapter.
var bifrostCommands = []string{"handoff", "handin", "plan", "review"}

func installAdapterCommands(a adapters.Adapter) error {
	cmdsDir := a.CommandsDir()
	adapterName := a.Name()

	if installDryRun {
		ui.Section(a.DisplayName(), fmt.Sprintf("would install to %s", cmdsDir))
		return nil
	}

	for _, name := range bifrostCommands {
		src := fmt.Sprintf("commands/%s/%s.md", adapterName, name)
		data, err := CommandsFS.ReadFile(src)
		if err != nil {
			return fmt.Errorf("embedded command %s: %w", src, err)
		}

		dest := a.CommandPath(name)

		if !installForce {
			if _, err := os.Stat(dest); err == nil {
				ui.Dim(fmt.Sprintf("  %s already exists, skipping (use --force to overwrite)", dest))
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0600); err != nil {
			return err
		}
	}

	ui.Success(fmt.Sprintf("%s  commands registered  (%s)", a.DisplayName(), cmdsDir))
	return nil
}

func installMCPConfig(a adapters.Adapter, mcpPath string) error {
	data, err := readConfigFile(mcpPath)
	if err != nil {
		return err
	}

	// The adapter knows the tool's format (JSON "mcpServers", JSON "mcp", TOML "mcp_servers")
	out, err := a.AddMCPServer(data)
	if err != nil {
		return fmt.Errorf("%s: %w", mcpPath, err)
	}

	if installDryRun {
		ui.Section(a.DisplayName(), fmt.Sprintf("would write MCP config to %s", mcpPath))
		return nil
	}

	if err := writeConfigFile(mcpPath, out); err != nil {
		return err
	}

	ui.Success(fmt.Sprintf("%s  MCP server registered  (%s)", a.DisplayName(), mcpPath))
	return nil
}

func installHooksConfig(displayName string, h adapters.HookInstaller) error {
	path := h.HooksConfigPath()
	data, err := readConfigFile(path)
	if err != nil {
		return err
	}
	out, err := h.AddHooks(data)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if installDryRun {
		ui.Section(displayName, fmt.Sprintf("would write hooks to %s", path))
		return nil
	}
	if err := writeConfigFile(path, out); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("%s  hooks registered  (%s)", displayName, path))
	return nil
}

// readConfigFile returns a tool's config file contents, or nil when the file
// does not exist yet.
func readConfigFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// writeConfigFile replaces path atomically, keeping its mode and following a
// symlink so dotfile-managed configs stay linked.
func writeConfigFile(path string, data []byte) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp := path + ".bifrost.tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
