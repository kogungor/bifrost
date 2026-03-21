package project

import (
	"os"
	"path/filepath"
	"strings"
)

const bifrostIgnoreEntry = ".bifrost/"
const bifrostIgnoreBlock = "\n# Bifrost runtime data\n.bifrost/\n"

// EnsureGitignore ensures .bifrost/ is listed in the project's .gitignore.
// Creates the file if it doesn't exist. Returns true if the file was modified.
func EnsureGitignore(projectRoot string) (bool, error) {
	path := filepath.Join(projectRoot, ".gitignore")

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	content := string(data)

	// Check if .bifrost/ is already covered
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == bifrostIgnoreEntry || line == ".bifrost" {
			return false, nil
		}
	}

	// Append the entry
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.WriteString(bifrostIgnoreBlock)
	if err != nil {
		return false, err
	}

	return true, nil
}
