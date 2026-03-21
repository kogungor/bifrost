package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignoreCreatesFile(t *testing.T) {
	tmp := t.TempDir()

	modified, err := EnsureGitignore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !modified {
		t.Error("expected file to be created")
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if !strings.Contains(string(data), ".bifrost/") {
		t.Errorf("expected .bifrost/ in gitignore, got: %q", data)
	}
}

func TestEnsureGitignoreAppendsToExisting(t *testing.T) {
	tmp := t.TempDir()
	existing := "node_modules/\n*.log\n"
	os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(existing), 0644)

	modified, err := EnsureGitignore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !modified {
		t.Error("expected file to be modified")
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(content, ".bifrost/") {
		t.Error("expected .bifrost/ appended")
	}
}

func TestEnsureGitignoreSkipsIfPresent(t *testing.T) {
	tmp := t.TempDir()
	existing := "node_modules/\n.bifrost/\n"
	os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(existing), 0644)

	modified, err := EnsureGitignore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if modified {
		t.Error("expected no modification when .bifrost/ already present")
	}
}

func TestEnsureGitignoreIdempotent(t *testing.T) {
	tmp := t.TempDir()

	EnsureGitignore(tmp)
	EnsureGitignore(tmp)

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	count := strings.Count(string(data), ".bifrost/")
	if count != 1 {
		t.Errorf("expected exactly 1 .bifrost/ entry, got %d", count)
	}
}
