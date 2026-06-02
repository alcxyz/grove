package app

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/alcxyz/grove/internal/config"
)

func withDependencyChecks(t *testing.T, lookPath func(string) (string, error), readFile func(string) ([]byte, error), getenv func(string) string, ghAuth func() error) {
	withDependencyChecksAndAzure(t, lookPath, readFile, getenv, ghAuth, func() error { return nil })
}

func withDependencyChecksAndAzure(t *testing.T, lookPath func(string) (string, error), readFile func(string) ([]byte, error), getenv func(string) string, ghAuth func() error, azDevOps func() error) {
	withDependencyChecksAll(t, lookPath, readFile, getenv, ghAuth, func() ([]dependencyTeaLogin, error) { return nil, nil }, azDevOps)
}

func withDependencyChecksAll(t *testing.T, lookPath func(string) (string, error), readFile func(string) ([]byte, error), getenv func(string) string, ghAuth func() error, teaLogins func() ([]dependencyTeaLogin, error), azDevOps func() error) {
	t.Helper()
	oldLookPath := dependencyLookPath
	oldReadFile := dependencyReadFile
	oldGetenv := dependencyGetenv
	oldGHAuth := dependencyGHAuth
	oldTeaLogins := dependencyTeaLogins
	oldAzureDevOps := dependencyAzureDevOps
	dependencyLookPath = lookPath
	dependencyReadFile = readFile
	dependencyGetenv = getenv
	dependencyGHAuth = ghAuth
	dependencyTeaLogins = teaLogins
	dependencyAzureDevOps = azDevOps
	t.Cleanup(func() {
		dependencyLookPath = oldLookPath
		dependencyReadFile = oldReadFile
		dependencyGetenv = oldGetenv
		dependencyGHAuth = oldGHAuth
		dependencyTeaLogins = oldTeaLogins
		dependencyAzureDevOps = oldAzureDevOps
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

func TestDependencyWarningsForgejoTeaAuthModeUsesCLI(t *testing.T) {
	withDependencyChecksAll(t,
		func(name string) (string, error) {
			switch name {
			case "git":
				return "/usr/bin/git", nil
			case "tea":
				return "/usr/bin/tea", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string) string { return "" },
		func() error { return nil },
		func() ([]dependencyTeaLogin, error) {
			return []dependencyTeaLogin{{Name: "git.example", URL: "https://git.example.test"}}, nil
		},
		func() error { return nil },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:        "test",
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.example.test",
		AuthMode:    "tea",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	if warnings != "" {
		t.Fatalf("expected no warnings, got %s", warnings)
	}
}

func TestDependencyWarningsForgejoTeaAuthModeReportsMissingLogin(t *testing.T) {
	withDependencyChecksAll(t,
		func(name string) (string, error) {
			switch name {
			case "git":
				return "/usr/bin/git", nil
			case "tea":
				return "/usr/bin/tea", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string) string { return "" },
		func() error { return nil },
		func() ([]dependencyTeaLogin, error) { return nil, nil },
		func() error { return nil },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:        "test",
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.example.test",
		AuthMode:    "tea",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	if !strings.Contains(warnings, "tea is not logged in") {
		t.Fatalf("warnings missing tea login dependency: %s", warnings)
	}
	if strings.Contains(warnings, "token_file") {
		t.Fatalf("auth_mode tea should not require token_file: %s", warnings)
	}
}

func TestDependencyWarningsForgejoReportsUnsupportedAuthMode(t *testing.T) {
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
		AuthMode:    "fj",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	if !strings.Contains(warnings, "unsupported auth_mode") || !strings.Contains(warnings, "use token or tea") {
		t.Fatalf("warnings missing unsupported Forgejo auth_mode: %s", warnings)
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

func TestDependencyWarningsAzureDevOpsRequiresAZAndProject(t *testing.T) {
	withDependencyChecksAndAzure(t,
		func(name string) (string, error) {
			if name == "git" {
				return "/usr/bin/git", nil
			}
			return "", exec.ErrNotFound
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string) string { return "" },
		func() error { return nil },
		func() error { return nil },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:  "ado",
		Owner: "acme",
		Forge: "azuredevops",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	for _, want := range []string{
		"missing project",
		"az not found on PATH",
	} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings missing %q: %s", want, warnings)
		}
	}
	if strings.Contains(warnings, "GitHub token not configured") {
		t.Fatalf("Azure DevOps should not require GitHub token auth: %s", warnings)
	}
}

func TestDependencyWarningsAzureDevOpsWithAZDoesNotRequireTokenFile(t *testing.T) {
	withDependencyChecksAndAzure(t,
		func(name string) (string, error) {
			switch name {
			case "git":
				return "/usr/bin/git", nil
			case "az":
				return "/usr/bin/az", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string) string { return "" },
		func() error { return nil },
		func() error { return nil },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:    "ado",
		Owner:   "acme",
		Forge:   "azuredevops",
		Project: "Core",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	if warnings != "" {
		t.Fatalf("expected no warnings, got %s", warnings)
	}
}

func TestDependencyWarningsAzureDevOpsReportsMissingExtension(t *testing.T) {
	withDependencyChecksAndAzure(t,
		func(name string) (string, error) {
			switch name {
			case "git":
				return "/usr/bin/git", nil
			case "az":
				return "/usr/bin/az", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string) string { return "" },
		func() error { return nil },
		func() error { return exec.ErrNotFound },
	)

	cfg := config.Config{Profiles: []config.Profile{{
		Name:    "ado",
		Owner:   "acme",
		Forge:   "azuredevops",
		Project: "Core",
	}}}

	warnings := strings.Join(DependencyWarnings(cfg), "\n")
	if !strings.Contains(warnings, "az repos unavailable") {
		t.Fatalf("warnings missing Azure DevOps extension dependency: %s", warnings)
	}
}
