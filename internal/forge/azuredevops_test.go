package forge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAzureDevOpsListPRsUsesAzureCLI(t *testing.T) {
	dir := t.TempDir()
	azPath := filepath.Join(dir, "az")
	script := `#!/bin/sh
case "$*" in
  "repos pr list --organization https://dev.azure.com/acme --project Core --repository app --status active --top 100 --include-links --output json --only-show-errors")
    printf '[{"pullRequestId":42,"title":"Ship it","status":"active","sourceRefName":"refs/heads/feature","creationDate":"2026-05-05T12:00:00Z","createdBy":{"displayName":"Ada"},"_links":{"web":{"href":"https://dev.azure.com/acme/Core/_git/app/pullrequest/42"}}}]'
    ;;
  *)
    echo "unexpected az args: $*" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(azPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := NewAzureDevOpsProvider(ProviderConfig{Project: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	prs, err := provider.ListPRs("acme/app")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 {
		t.Fatalf("len(prs) = %d, want 1", len(prs))
	}
	pr := prs[0]
	if pr.Number != 42 || pr.Title != "Ship it" || pr.Author != "Ada" || pr.Branch != "feature" {
		t.Fatalf("unexpected PR: %+v", pr)
	}
}

func TestAzureDevOpsListReposFiltersPrefixes(t *testing.T) {
	dir := t.TempDir()
	azPath := filepath.Join(dir, "az")
	script := `#!/bin/sh
case "$*" in
  "repos list --organization https://dev.azure.com/acme --project Core --output json --only-show-errors")
    printf '[{"name":"svc-api"},{"name":"tooling"},{"name":"svc-worker"}]'
    ;;
  *)
    echo "unexpected az args: $*" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(azPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := NewAzureDevOpsProvider(ProviderConfig{Project: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	repos, err := provider.ListRepos("acme", []string{"svc-"})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(repos, ",")
	if got != "svc-api,svc-worker" {
		t.Fatalf("repos = %q, want svc-api,svc-worker", got)
	}
}

func TestAzureDevOpsListBranchesMarksDefaultBranch(t *testing.T) {
	dir := t.TempDir()
	azPath := filepath.Join(dir, "az")
	script := `#!/bin/sh
case "$*" in
  "repos show --organization https://dev.azure.com/acme --project Core --repository app --output json --only-show-errors")
    printf '{"name":"app","defaultBranch":"refs/heads/dev"}'
    ;;
  "repos ref list --organization https://dev.azure.com/acme --project Core --repository app --filter heads/ --output json --only-show-errors")
    printf '[{"name":"refs/heads/main"},{"name":"refs/heads/dev"}]'
    ;;
  *)
    echo "unexpected az args: $*" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(azPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := NewAzureDevOpsProvider(ProviderConfig{Project: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	branches, err := provider.ListBranches("acme/app")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("len(branches) = %d, want 2", len(branches))
	}
	defaults := map[string]bool{}
	for _, branch := range branches {
		defaults[branch.Name] = branch.IsDefault
	}
	if defaults["dev"] != true || defaults["main"] != false {
		t.Fatalf("unexpected default flags: %+v", branches)
	}
}

func TestAzureDevOpsCloneRepoUsesSSHURLWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	azPath := filepath.Join(dir, "az")
	gitPath := filepath.Join(dir, "git")
	recordPath := filepath.Join(dir, "git-args")
	azScript := `#!/bin/sh
case "$*" in
  "repos show --organization https://dev.azure.com/acme --project Core --repository app --output json --only-show-errors")
    printf '{"name":"app","remoteUrl":"https://dev.azure.com/acme/Core/_git/app","sshUrl":"ssh://git@ssh.dev.azure.com/v3/acme/Core/app"}'
    ;;
  *)
    echo "unexpected az args: $*" >&2
    exit 1
    ;;
esac
`
	gitScript := `#!/bin/sh
printf '%s\n' "$*" > "` + recordPath + `"
exit 0
`
	if err := os.WriteFile(azPath, []byte(azScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitPath, []byte(gitScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider, err := NewAzureDevOpsProvider(ProviderConfig{Project: "Core", CloneProto: "ssh"})
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.CloneRepo("acme", "app", filepath.Join(dir, "app")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if !strings.Contains(got, "ssh://git@ssh.dev.azure.com/v3/acme/Core/app") {
		t.Fatalf("unexpected git args: %q", got)
	}
}
