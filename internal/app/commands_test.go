package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alcxyz/grove/internal/config"
)

func TestDiscoverRepoPathsHonorsProfileExcludes(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"dms-plugins", "forge-tidy", "grove.bak"} {
		if err := os.MkdirAll(filepath.Join(base, name, ".git"), 0o755); err != nil {
			t.Fatalf("create test repo %s: %v", name, err)
		}
	}

	got := discoverRepoPaths(config.Profile{
		BasePaths:    []string{base},
		Prefixes:     []string{},
		ExcludePaths: []string{filepath.Join(base, "dms-plugins")},
		ExcludeRepos: []string{"grove.bak"},
	})

	want := []string{filepath.Join(base, "forge-tidy")}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("discoverRepoPaths() = %v, want %v", got, want)
	}
}

func TestDiscoverRepoPathsIncludesExactRepoPaths(t *testing.T) {
	base := t.TempDir()
	repoPath := filepath.Join(base, "grove")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatalf("create exact repo path: %v", err)
	}

	got := discoverRepoPaths(config.Profile{
		BasePaths: []string{filepath.Join(base, "empty")},
		Prefixes:  []string{"does-not-match-"},
		RepoPaths: []string{repoPath},
	})

	if len(got) != 1 || got[0] != repoPath {
		t.Fatalf("discoverRepoPaths() = %v, want [%s]", got, repoPath)
	}
}
