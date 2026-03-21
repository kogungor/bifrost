package project

import (
	"os"
	"path/filepath"
)

// Indicators is the ordered list of files/dirs that mark a project root.
var Indicators = []string{
	".git",
	"BIFROST.md",
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"pom.xml",
	"build.gradle",
}

// Root walks up from dir searching for project root indicators.
// Returns the root path and the indicator that was found.
// Falls back to dir itself if no indicator is found.
func Root(dir string) (root string, indicator string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}

	current := abs
	for {
		for _, ind := range Indicators {
			path := filepath.Join(current, ind)
			if _, err := os.Stat(path); err == nil {
				return current, ind, nil
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root, fall back to original dir
			return abs, "", nil
		}
		current = parent
	}
}
