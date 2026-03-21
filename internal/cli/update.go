package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kog/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for or install updates",
	RunE:  runUpdate,
}

var updateCheck bool

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Check for updates without installing")
	rootCmd.AddCommand(updateCmd)
}

const githubReleasesURL = "https://api.github.com/repos/kog/bifrost/releases/latest"

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ui.Blank()

	latest, err := checkLatestVersion()
	if err != nil {
		ui.Error("Could not check for updates.", err.Error())
		return err
	}

	current := strings.TrimPrefix(Version, "v")
	remote := strings.TrimPrefix(latest.TagName, "v")

	if current == remote || Version == "dev" && latest.TagName == "" {
		ui.Success(fmt.Sprintf("Up to date  (%s)", Version))
		return nil
	}

	if updateCheck {
		ui.Warning(fmt.Sprintf("Update available: %s → %s", Version, latest.TagName))
		ui.Dim(fmt.Sprintf("  %s", latest.HTMLURL))
		ui.Dim("  Run 'bifrost update' to install.")
		return nil
	}

	// Full update: for now, point to the release page.
	// Self-replacing binary update can be added later.
	ui.Warning(fmt.Sprintf("Update available: %s → %s", Version, latest.TagName))
	ui.Blank()
	ui.Dim("Auto-update is not yet implemented.")
	ui.Dim(fmt.Sprintf("Download the latest release: %s", latest.HTMLURL))
	ui.Dim("Or run: brew upgrade bifrost")

	return nil
}

func checkLatestVersion() (*githubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(githubReleasesURL)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("could not parse response: %w", err)
	}

	return &release, nil
}
