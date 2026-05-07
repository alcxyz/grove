package app

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/alcxyz/grove/internal/config"
)

func withDependencyChecks(t *testing.T, lookPath func(string) (string, error), readFile func(string) ([]byte, error), getenv func(string) string, ghAuth func() error) {
	t.Helper()
	oldLookPath := dependencyLookPath
	oldReadFile := dependencyReadFile
	oldGetenv := dependencyGetenv
	oldGHAuth := dependencyGHAuth
	dependencyLookPath = lookPath
	dependencyReadFile = readFile
	dependencyGetenv = getenv
	dependencyGHAuth = ghAuth
	t.Cleanup(func() {
		dependencyLookPath = oldLookPath
		dependencyReadFile = oldReadFile
		dependencyGetenv = oldGetenv
		dependencyGHAuth = oldGHAuth
	})
}

func TestDependencyWarningsForgejoDoesNotRequireCLI(t *testing.T) {
	withDependencyChecks(t,
		func(name string) (string, error) {
			if name == "git" {
				return "/usr/bin/git", nil
			}
			return "", exec.ErrNotFound
		},
		func(string) ([]byte, error) { return []byte("token\n"), nil },
		func(string) string { return "" },
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

func TestDependencyWarningsGitHubTokenFileDoesNotRequireEnvToken(t *testing.T) {
	withDependencyChecks(t,
		func(name string) (string, error) {
			if name == "git" {
				return "/usr/bin/git", nil
			}
			return "", exec.ErrNotFound
		},
		func(string) ([]byte, error) { return []byte("token\n"), nil },
		func(string) string { return "" },
		func() error { return nil },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:      "test",
		Owner:     "alcxyz",
		Forge:     "github",
		TokenFile: "/run/secrets/github-token",
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
		func(string) string { return "" },
		func() error { return nil },
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
		"GitHub token not configured",
	} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings missing %q: %s", want, warnings)
		}
	}
	if strings.Contains(warnings, "gh not found") {
		t.Fatalf("GitHub API provider should not require gh CLI: %s", warnings)
	}
}

func TestDependencyWarningsGitHubGHAuthModeUsesCLI(t *testing.T) {
	withDependencyChecks(t,
		func(name string) (string, error) {
			switch name {
			case "git":
				return "/usr/bin/git", nil
			case "gh":
				return "/usr/bin/gh", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string) string { return "" },
		func() error { return nil },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:     "test",
		Owner:    "bn-apps",
		Forge:    "github",
		AuthMode: "gh",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	if warnings != "" {
		t.Fatalf("expected no warnings, got %s", warnings)
	}
}

func TestDependencyWarningsGitHubGHAuthModeReportsMissingAuth(t *testing.T) {
	withDependencyChecks(t,
		func(name string) (string, error) {
			if name == "git" {
				return "/usr/bin/git", nil
			}
			return "", exec.ErrNotFound
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string) string { return "" },
		func() error { return exec.ErrNotFound },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:     "test",
		Owner:    "bn-apps",
		Forge:    "github",
		AuthMode: "gh",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	if !strings.Contains(warnings, "gh not found on PATH") {
		t.Fatalf("warnings missing gh dependency: %s", warnings)
	}
	if strings.Contains(warnings, "GitHub token not configured") {
		t.Fatalf("auth_mode gh should not require token auth: %s", warnings)
	}
}
