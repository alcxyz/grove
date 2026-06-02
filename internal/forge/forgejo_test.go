package forge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgejoCloneURLUsesConfiguredSSHHost(t *testing.T) {
	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		CloneProto:  "ssh",
		SSHHost:     "ssh-git.alc.xyz",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := provider.cloneURL("alcxyz", "grove")
	want := "git@ssh-git.alc.xyz:alcxyz/grove.git"
	if got != want {
		t.Fatalf("cloneURL() = %q, want %q", got, want)
	}
}

func TestForgejoCloneURLFallsBackToInstanceHost(t *testing.T) {
	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://git.example.com",
		CloneProto:  "ssh",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := provider.cloneURL("team", "repo")
	want := "git@git.example.com:team/repo.git"
	if got != want {
		t.Fatalf("cloneURL() = %q, want %q", got, want)
	}
}

func TestForgejoTeaAuthModeUsesTeaAPI(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	teaPath := filepath.Join(dir, "tea")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsPath + `"
printf '[{"number":8,"title":"API via tea","user":{"login":"alc"},"state":"open","updated_at":"2026-05-05T12:00:00Z","html_url":"https://git.alc.xyz/alcxyz/grove/issues/8"}]'
`
	if err := os.WriteFile(teaPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		AuthMode:    "tea",
	})
	if err != nil {
		t.Fatal(err)
	}

	issues, err := provider.ListIssues("alcxyz/grove")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Title != "API via tea" {
		t.Fatalf("unexpected issues from tea API: %+v", issues)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "api\nhttps://git.alc.xyz/api/v1/repos/alcxyz/grove/issues?state=open&type=issues\n"
	if string(args) != want {
		t.Fatalf("unexpected tea args:\n%s\nwant:\n%s", args, want)
	}
}

func TestForgejoTeaAuthModeWrapsAuthFailures(t *testing.T) {
	dir := t.TempDir()
	teaPath := filepath.Join(dir, "tea")
	script := `#!/bin/sh
echo "Error: no available login" >&2
exit 1
`
	if err := os.WriteFile(teaPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		AuthMode:    "tea",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.ListIssues("alcxyz/grove")
	if !errors.Is(err, ErrNotAuthenticated) {
		t.Fatalf("expected ErrNotAuthenticated, got %v", err)
	}
}

func TestForgejoTeaAuthModeClonesWithTea(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	teaPath := filepath.Join(dir, "tea")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsPath + `"
`
	if err := os.WriteFile(teaPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		AuthMode:    "tea",
	})
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "grove")
	if err := provider.CloneRepo("alcxyz", "grove", target); err != nil {
		t.Fatal(err)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(args))
	want := "clone\nhttps://git.alc.xyz/alcxyz/grove.git\n" + target
	if got != want {
		t.Fatalf("unexpected tea clone args:\n%s\nwant:\n%s", got, want)
	}
}
