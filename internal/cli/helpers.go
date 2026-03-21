package cli

import (
	"os"

	"github.com/kog/bifrost/internal/project"
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
