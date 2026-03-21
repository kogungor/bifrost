package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kogungor/bifrost/internal/adapters"
	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose installation and configuration problems",
	RunE:  runDoctor,
}

func init() {
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
			ui.Error(fmt.Sprintf("%-20s missing: %s", a.DisplayName(), strings.Join(missing, ", ")),
				"Run 'bifrost install' to register commands.")
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
			ui.Warning("Project              no BIFROST.md (optional — run 'bifrost init' to create)")
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
				ui.Error("Gitignore            .bifrost/ not excluded",
					"Run 'bifrost init' to add it, or add '.bifrost/' to .gitignore manually.")
			}
		} else {
			ui.Warning("Gitignore            no .gitignore found")
		}
	}

	ui.Blank()
	if issues == 0 {
		ui.Plain("All checks passed.")
	} else {
		ui.Plain(fmt.Sprintf("%d issue(s) found. See above for fixes.", issues))
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
