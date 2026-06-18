package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kogungor/bifrost/internal/adapters"
	"github.com/kogungor/bifrost/internal/project"
	"github.com/kogungor/bifrost/internal/security"
	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose installation and configuration problems",
	Long:  "Checks binary version, slash command registration, MCP server registration, BIFROST.md presence, snapshot freshness, and .gitignore coverage. Prints warnings for optional items, errors for required ones.",
	RunE:  runDoctor,
}

var doctorFix bool
var doctorSecurity bool

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Attempt to fix detected issues automatically")
	doctorCmd.Flags().BoolVar(&doctorSecurity, "security", false, "Check Bifrost local state for secret-like values")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ui.Blank()
	ui.Plain("Bifrost Doctor")
	ui.Blank()

	issues := 0

	// 1. Binary version
	ui.Success(fmt.Sprintf("Binary               %s", Version))

	// 2. Check each adapter
	for _, a := range adapters.All() {
		if !a.IsInstalled() {
			ui.Warning(fmt.Sprintf("%s  not detected (optional)", a.DisplayName()))
			continue
		}

		cmdsDir := a.CommandsDir()
		handoff := filepath.Join(cmdsDir, "handoff.md")
		handin := filepath.Join(cmdsDir, "handin.md")

		handoffOK := fileExists(handoff)
		handinOK := fileExists(handin)

		if handoffOK && handinOK {
			ui.Success(fmt.Sprintf("%-20s commands registered  (%s)", a.DisplayName(), cmdsDir))
		} else {
			issues++
			missing := []string{}
			if !handoffOK {
				missing = append(missing, "handoff.md")
			}
			if !handinOK {
				missing = append(missing, "handin.md")
			}
			if doctorFix {
				if err := installAdapterCommands(a); err != nil {
					ui.Error(fmt.Sprintf("%-20s fix failed", a.DisplayName()), err.Error())
				} else {
					ui.Success(fmt.Sprintf("%-20s commands registered  (%s)  [fixed]", a.DisplayName(), cmdsDir))
					issues--
				}
			} else {
				ui.Error(fmt.Sprintf("%-20s missing: %s", a.DisplayName(), strings.Join(missing, ", ")),
					fmt.Sprintf("Run 'bifrost install --adapter %s' to register, or 'bifrost doctor --fix'.", a.Name()))
			}
		}

		// MCP check — recommended for /handin (falls back to direct file read without it)
		mcpPath := a.MCPConfigPath()
		if mcpPath != "" {
			if isMCPConfigured(mcpPath) {
				ui.Success(fmt.Sprintf("%-20s MCP server registered", a.DisplayName()))
			} else {
				if doctorFix {
					if err := installMCPConfig(a, mcpPath); err != nil {
						ui.Warning(fmt.Sprintf("%-20s MCP fix failed: %s", a.DisplayName(), err.Error()))
					} else {
						ui.Success(fmt.Sprintf("%-20s MCP server registered  [fixed]", a.DisplayName()))
					}
				} else {
					ui.Warning(fmt.Sprintf("%-20s MCP server not registered (optional — run 'bifrost install --mcp' to enable)", a.DisplayName()))
				}
			}
		}
	}

	// 3. Project checks (only if we're in a project)
	root, err := resolveProject()
	if err == nil {
		// BIFROST.md
		configPath := filepath.Join(root, "BIFROST.md")
		if fileExists(configPath) {
			ui.Success("Project              BIFROST.md found")
		} else {
			if doctorFix {
				name := filepath.Base(root)
				content := fmt.Sprintf(bifrostMdTemplate, name, name)
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					ui.Warning(fmt.Sprintf("Project              BIFROST.md fix failed: %s", err.Error()))
				} else {
					ui.Success("Project              BIFROST.md created  [fixed]")
				}
			} else {
				ui.Warning("Project              no BIFROST.md (optional — run 'bifrost init' to create)")
			}
		}

		// Snapshot
		snap, snapErr := snapshot.Read(root)
		if snapErr != nil {
			issues++
			ui.Error("Snapshot             none found",
				"Run /handoff in your AI coding tool to create a snapshot.")
		} else {
			age := snap.Age()
			if age > 24*time.Hour {
				ui.Warning(fmt.Sprintf("Snapshot             %s old", formatAge(snap.Timestamp)))
				ui.Dim("  The context may be stale. Consider running /handoff again.")
			} else if age > 2*time.Hour {
				ui.Warning(fmt.Sprintf("Snapshot             %s", formatAge(snap.Timestamp)))
			} else {
				ui.Success(fmt.Sprintf("Snapshot             %s", formatAge(snap.Timestamp)))
			}
		}

		// Gitignore
		gitignorePath := filepath.Join(root, ".gitignore")
		if fileExists(gitignorePath) {
			data, _ := os.ReadFile(gitignorePath)
			if strings.Contains(string(data), ".bifrost/") || strings.Contains(string(data), ".bifrost") {
				ui.Success("Gitignore            .bifrost/ excluded")
			} else {
				issues++
				if doctorFix {
					if _, err := project.EnsureGitignore(root); err != nil {
						ui.Error("Gitignore            fix failed", err.Error())
					} else {
						ui.Success("Gitignore            .bifrost/ excluded  [fixed]")
						issues--
					}
				} else {
					ui.Error("Gitignore            .bifrost/ not excluded",
						"Run 'bifrost init' to add it, or 'bifrost doctor --fix'.")
				}
			}
		} else {
			if doctorFix {
				if _, err := project.EnsureGitignore(root); err != nil {
					ui.Warning(fmt.Sprintf("Gitignore            could not create .gitignore: %s", err.Error()))
				} else {
					ui.Success("Gitignore            .gitignore created with .bifrost/ excluded  [fixed]")
				}
			} else {
				ui.Warning("Gitignore            no .gitignore found")
			}
		}

		if doctorSecurity {
			issues += runDoctorSecurity(root)
		}
	}

	ui.Blank()
	if issues == 0 {
		if doctorFix {
			ui.Plain("All checks passed.")
		} else {
			ui.Plain("All checks passed.")
		}
	} else {
		if doctorFix {
			ui.Plain(fmt.Sprintf("%d issue(s) could not be fixed automatically. See above.", issues))
		} else {
			ui.Plain(fmt.Sprintf("%d issue(s) found. Run 'bifrost doctor --fix' to attempt automatic fixes.", issues))
		}
	}

	return nil
}

func runDoctorSecurity(root string) int {
	targets, err := scrubTargets(root, true)
	if err != nil {
		ui.Error("Security             could not list Bifrost files", err.Error())
		return 1
	}
	if len(targets) == 0 {
		ui.Success("Security             no Bifrost state files found")
		return 0
	}
	cfg := security.LoadConfig(root)
	active := 0
	allowlisted := 0
	for _, target := range targets {
		data, err := os.ReadFile(target)
		if err != nil {
			ui.Warning(fmt.Sprintf("Security             %s unreadable: %s", relToRoot(root, target), err.Error()))
			continue
		}
		findings := security.ScanString(string(data), cfg)
		if security.CountActive(findings) > 0 {
			active += security.CountActive(findings)
			ui.Error("Security             secret-like values found",
				fmt.Sprintf("%s: %s", relToRoot(root, target), security.Summary(findings)))
		}
		if security.CountAllowlisted(findings) > 0 {
			allowlisted += security.CountAllowlisted(findings)
		}
	}
	if active > 0 {
		ui.Dim("  Run 'bifrost scrub --write --history' to redact Bifrost local state.")
		return 1
	}
	if allowlisted > 0 {
		ui.Success(fmt.Sprintf("Security             clean (%d allowlisted finding(s))", allowlisted))
		return 0
	}
	ui.Success("Security             no secret-like values detected")
	return 0
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isMCPConfigured returns true if the bifrost MCP server entry exists in the given config file.
func isMCPConfigured(mcpPath string) bool {
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return false
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	_, found := servers["bifrost"]
	return found
}
