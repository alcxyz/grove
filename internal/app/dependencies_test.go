package app

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/alcxyz/grove/internal/config"
)

func withDependencyChecks(t *testing.T, lookPath func(string) (string, error), readFile func(string) ([]byte, error), ghAuthStatus func() error) {
	t.Helper()
	oldLookPath := dependencyLookPath
	oldReadFile := dependencyReadFile
	oldGHAuthStatus := dependencyGHAuthStatus
	dependencyLookPath = lookPath
	dependencyReadFile = readFile
	dependencyGHAuthStatus = ghAuthStatus
	t.Cleanup(func() {
		dependencyLookPath = oldLookPath
		dependencyReadFile = oldReadFile
		dependencyGHAuthStatus = oldGHAuthStatus
	})
}

func TestDependencyWarningsForgejoDoesNotRequireCLI(t *testing.T) {
	withDependencyChecks(t,
		func(name string) (string, error) {
			switch name {
			case "git":
				return "/usr/bin/git", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		func(string) ([]byte, error) { return []byte("token\n"), nil },
		func() error { return nil },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:        "test",
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.example.test",
		TokenFile:   "/run/secrets/forgejo-token",
	}}}

	warnings := DependencyWarnings(cfg)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestDependencyWarningsReportsProviderProblems(t *testing.T) {
	withDependencyChecks(t,
		func(name string) (string, error) {
			if name == "git" {
				return "/usr/bin/git", nil
			}
			return "", exec.ErrNotFound
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func() error { return errors.New("not logged in") },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:        "test",
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.example.test",
		TokenFile:   "/missing-token",
		Social: config.Remote{
			Owner: "alcxyz",
			Forge: "github",
		},
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	for _, want := range []string{
		"token_file /missing-token cannot be read",
		"gh not found on PATH",
	} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings missing %q: %s", want, warnings)
		}
	}
}
