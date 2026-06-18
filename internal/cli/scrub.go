package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kogungor/bifrost/internal/security"
	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var scrubCmd = &cobra.Command{
	Use:   "scrub",
	Short: "Check or redact secret-like values in Bifrost local state",
	Long:  "Checks or redacts secret-like values in local Bifrost files. Use --history to include archived snapshots; raw secret values are not printed.",
	RunE:  runScrub,
}

var (
	scrubCheck   bool
	scrubWrite   bool
	scrubHistory bool
)

func init() {
	scrubCmd.Flags().BoolVar(&scrubCheck, "check", false, "Check for secret-like values without writing changes")
	scrubCmd.Flags().BoolVar(&scrubWrite, "write", false, "Redact secret-like values in Bifrost local state")
	scrubCmd.Flags().BoolVar(&scrubHistory, "history", false, "Include archived snapshot history")
	rootCmd.AddCommand(scrubCmd)
}

func runScrub(cmd *cobra.Command, args []string) error {
	check := scrubCheck
	write := scrubWrite
	if check && write {
		ui.Error("Invalid scrub flags.", "Use either --check or --write, not both.")
		return fmt.Errorf("conflicting scrub flags")
	}
	if !check && !write {
		check = true
	}
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return verifyExitError{code: 2}
	}
	targets, err := scrubTargets(root, scrubHistory)
	if err != nil {
		ui.Error("Could not list Bifrost files.", err.Error())
		return verifyExitError{code: 2}
	}
	cfg := security.LoadConfig(root)
	activeFindings := 0
	allowlistedFindings := 0
	changed := 0
	ui.Section("Bifrost Scrub", scrubMode(write, scrubHistory))
	if len(targets) == 0 {
		ui.Plain("No Bifrost state files found.")
		return nil
	}
	for _, target := range targets {
		data, err := os.ReadFile(target)
		if err != nil {
			ui.Warning(fmt.Sprintf("%s unreadable: %s", relToRoot(root, target), err.Error()))
			continue
		}
		redacted, findings := security.RedactString(string(data), cfg)
		active := security.CountActive(findings)
		allowlisted := security.CountAllowlisted(findings)
		activeFindings += active
		allowlistedFindings += allowlisted
		if active == 0 && allowlisted == 0 {
			ui.Success(fmt.Sprintf("%s clean", relToRoot(root, target)))
			continue
		}
		if active > 0 {
			ui.Warning(fmt.Sprintf("%s contains secret-like values: %s", relToRoot(root, target), security.Summary(findings)))
		}
		if allowlisted > 0 {
			ui.Dim(fmt.Sprintf("  %d allowlisted finding(s) suppressed", allowlisted))
		}
		if write && active > 0 && redacted != string(data) {
			if err := writeFileAtomic(target, []byte(redacted)); err != nil {
				ui.Error("Could not write redacted file.", err.Error())
				return verifyExitError{code: 2}
			}
			changed++
			ui.Success(fmt.Sprintf("%s redacted", relToRoot(root, target)))
		}
	}
	if write {
		ui.Section("Result", fmt.Sprintf("%d file(s) redacted", changed))
		return nil
	}
	if activeFindings > 0 {
		ui.Section("Result", fmt.Sprintf("%d active finding(s), %d allowlisted", activeFindings, allowlistedFindings))
		return verifyExitError{code: 2}
	}
	ui.Section("Result", fmt.Sprintf("No active findings, %d allowlisted", allowlistedFindings))
	return nil
}

func scrubMode(write, history bool) string {
	if write {
		if history {
			return "write + history"
		}
		return "write"
	}
	if history {
		return "check + history"
	}
	return "check"
}

func scrubTargets(projectRoot string, includeHistory bool) ([]string, error) {
	seen := map[string]bool{}
	var targets []string
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			seen[path] = true
			targets = append(targets, path)
		}
	}
	add(snapshot.SessionPath(projectRoot))
	add(snapshot.SnapshotJSONPath(projectRoot))
	addMatches := func(pattern string) error {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, match := range matches {
			add(match)
		}
		return nil
	}
	if err := addMatches(filepath.Join(snapshot.EvidenceDir(projectRoot), "*.json")); err != nil {
		return nil, err
	}
	if err := addMatches(filepath.Join(snapshot.PlansDir(projectRoot), "*.json")); err != nil {
		return nil, err
	}
	if err := addMatches(filepath.Join(snapshot.Dir(projectRoot), "*.plan.md")); err != nil {
		return nil, err
	}
	if includeHistory {
		if err := addMatches(filepath.Join(snapshot.HistoryDir(projectRoot), "*.md")); err != nil {
			return nil, err
		}
		if err := addMatches(filepath.Join(snapshot.HistoryDir(projectRoot), "*.json")); err != nil {
			return nil, err
		}
	}
	sort.Strings(targets)
	return targets, nil
}

func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func relToRoot(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	if strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}
