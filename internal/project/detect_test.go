package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootFindsGit(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	os.Mkdir(gitDir, 0755)

	sub := filepath.Join(tmp, "src", "pkg")
	os.MkdirAll(sub, 0755)

	root, ind, err := Root(sub)
	if err != nil {
		t.Fatal(err)
	}
	if root != tmp {
		t.Errorf("expected root %s, got %s", tmp, root)
	}
	if ind != ".git" {
		t.Errorf("expected indicator .git, got %s", ind)
	}
}

func TestRootFindsGoMod(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	sub := filepath.Join(tmp, "internal")
	os.MkdirAll(sub, 0755)

	root, ind, err := Root(sub)
	if err != nil {
		t.Fatal(err)
	}
	if root != tmp {
		t.Errorf("expected root %s, got %s", tmp, root)
	}
	if ind != "go.mod" {
		t.Errorf("expected indicator go.mod, got %s", ind)
	}
}

func TestRootFindsPackageJSON(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte("{}"), 0644)

	root, ind, err := Root(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if root != tmp {
		t.Errorf("expected root %s, got %s", tmp, root)
	}
	if ind != "package.json" {
		t.Errorf("expected indicator package.json, got %s", ind)
	}
}

func TestRootFindsBifrostMd(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "BIFROST.md"), []byte("# proj"), 0644)

	root, ind, err := Root(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if root != tmp {
		t.Errorf("expected root %s, got %s", tmp, root)
	}
	if ind != "BIFROST.md" {
		t.Errorf("expected indicator BIFROST.md, got %s", ind)
	}
}

func TestRootFallsToCWD(t *testing.T) {
	tmp := t.TempDir()
	// No indicators at all

	root, ind, err := Root(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if root != tmp {
		t.Errorf("expected fallback to %s, got %s", tmp, root)
	}
	if ind != "" {
		t.Errorf("expected empty indicator on fallback, got %s", ind)
	}
}

func TestRootPrefersGitOverGoMod(t *testing.T) {
	tmp := t.TempDir()
	os.Mkdir(filepath.Join(tmp, ".git"), 0755)
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	root, ind, err := Root(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if root != tmp {
		t.Errorf("expected root %s, got %s", tmp, root)
	}
	if ind != ".git" {
		t.Errorf("expected .git to take priority, got %s", ind)
	}
}
