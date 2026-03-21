package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kogungor/bifrost/internal/project"
	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Bifrost in the current project",
	RunE:  runInit,
}

var initName string

func init() {
	initCmd.Flags().StringVar(&initName, "name", "", "Project name")
	rootCmd.AddCommand(initCmd)
}

const bifrostMdTemplate = `---
project: %s
---

# Project: %s

## What this is
[One to three sentences: what the project does and its current stage.]

## Stack
- [Runtime, framework, database, key libraries]

## Conventions
- [Code conventions, patterns, rules the AI must follow in this project]

## Always read before working
- [List important files the AI should read first]

## Commands
- [Development commands: build, test, run]

## Do not touch
- [Paths that should never be edited manually]
`

func runInit(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}

	name := initName
	if name == "" {
		name = filepath.Base(root)
	}

	// Create .bifrost/ and .bifrost/history/
	if err := snapshot.EnsureDir(root); err != nil {
		ui.Error("Could not create .bifrost/ directory.", err.Error())
		return err
	}

	// Ensure .gitignore
	modified, err := project.EnsureGitignore(root)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not update .gitignore: %s", err))
	}

	// Create BIFROST.md if it doesn't exist
	bifrostMd := filepath.Join(root, "BIFROST.md")
	createdConfig := false
	if _, err := os.Stat(bifrostMd); os.IsNotExist(err) {
		content := fmt.Sprintf(bifrostMdTemplate, name, name)
		if err := os.WriteFile(bifrostMd, []byte(content), 0644); err != nil {
			ui.Warning(fmt.Sprintf("Could not create BIFROST.md: %s", err))
		} else {
			createdConfig = true
		}
	}

	// Print confirmation
	ui.Blank()
	ui.Success(fmt.Sprintf("Bifrost initialized in %s", root))
	ui.Blank()

	if createdConfig {
		ui.Dim("Created BIFROST.md — edit it with your project details.")
	} else {
		ui.Dim("BIFROST.md already exists.")
	}

	if modified {
		ui.Dim("Added .bifrost/ to .gitignore.")
	}

	ui.Blank()
	ui.Dim("Run /handoff in your AI coding tool to create your first snapshot.")

	return nil
}
