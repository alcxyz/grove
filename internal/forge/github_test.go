package forge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGitHubListPRsUsesRESTAPI(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "github-token")
	if err := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want bearer token", got)
		}
		switch r.URL.Path {
		case "/repos/alcxyz/grove/pulls":
			if got := r.URL.Query().Get("state"); got != "open" {
				t.Errorf("state query = %q, want open", got)
			}
			writeJSON(t, w, []map[string]any{{
				"number":     26,
				"title":      "Use GitHub API",
				"user":       map[string]any{"login": "alc"},
				"head":       map[string]any{"ref": "dev", "sha": "abc123"},
				"state":      "open",
				"updated_at": "2026-05-05T12:00:00Z",
				"html_url":   "https://github.com/alcxyz/grove/pull/26",
			}})
		case "/repos/alcxyz/grove/commits/abc123/status":
			writeJSON(t, w, map[string]any{"state": "success"})
		case "/repos/alcxyz/grove/actions/runs":
			if got := r.URL.Query().Get("head_sha"); got != "abc123" {
				t.Errorf("head_sha query = %q, want abc123", got)
			}
			writeJSON(t, w, map[string]any{
				"total_count": 1,
				"workflow_runs": []map[string]any{{
					"status":     "completed",
					"conclusion": "success",
				}},
			})
		case "/repos/alcxyz/grove/pulls/26/reviews":
			writeJSON(t, w, []map[string]any{{
				"state": "APPROVED",
				"user":  map[string]any{"login": "reviewer"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewGitHubProvider(ProviderConfig{TokenFile: tokenPath})
	provider.apiURL = server.URL

	prs, err := provider.ListPRs("alcxyz/grove")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 {
		t.Fatalf("len(prs) = %d, want 1", len(prs))
	}
	pr := prs[0]
	if pr.Number != 26 || pr.Title != "Use GitHub API" || pr.Author != "alc" || pr.Branch != "dev" {
		t.Fatalf("unexpected PR: %+v", pr)
	}
	if pr.Checks != "pass" {
		t.Fatalf("Checks = %q, want pass", pr.Checks)
	}
	if pr.ReviewDecision != "APPROVED" {
		t.Fatalf("ReviewDecision = %q, want APPROVED", pr.ReviewDecision)
	}
	if want := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC); !pr.UpdatedAt.Equal(want) {
		t.Fatalf("UpdatedAt = %s, want %s", pr.UpdatedAt, want)
	}
}

func TestGitHubListIssuesFiltersPullRequests(t *testing.T) {
	t.Setenv("GH_TOKEN", "env-token")
	t.Setenv("GITHUB_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer env-token" {
			t.Errorf("Authorization header = %q, want env token", got)
		}
		if r.URL.Path != "/repos/alcxyz/grove/issues" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, []map[string]any{
			{
				"number":     8,
				"title":      "Add Azure DevOps provider",
				"user":       map[string]any{"login": "alc"},
				"state":      "open",
				"labels":     []map[string]any{{"name": "enhancement"}},
				"assignees":  []map[string]any{{"login": "alc"}},
				"milestone":  map[string]any{"title": "v1"},
				"created_at": "2026-05-01T12:00:00Z",
				"updated_at": "2026-05-05T12:00:00Z",
				"html_url":   "https://github.com/alcxyz/grove/issues/8",
			},
			{
				"number":       9,
				"title":        "PR-shaped issue",
				"user":         map[string]any{"login": "alc"},
				"pull_request": map[string]any{"url": "https://api.github.com/repos/alcxyz/grove/pulls/9"},
			},
		})
	}))
	defer server.Close()

	provider := NewGitHubProvider(ProviderConfig{})
	provider.apiURL = server.URL

	issues, err := provider.ListIssues("alcxyz/grove")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	issue := issues[0]
	if issue.Number != 8 || issue.Title != "Add Azure DevOps provider" || issue.Author != "alc" {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if !reflect.DeepEqual(issue.Labels, []string{"enhancement"}) {
		t.Fatalf("Labels = %#v, want enhancement", issue.Labels)
	}
	if !reflect.DeepEqual(issue.Assignees, []string{"alc"}) {
		t.Fatalf("Assignees = %#v, want alc", issue.Assignees)
	}
	if issue.Milestone != "v1" {
		t.Fatalf("Milestone = %q, want v1", issue.Milestone)
	}
}

func TestGitHubListWorkflowRunsUsesWorkflowPathFallback(t *testing.T) {
	t.Setenv("GH_TOKEN", "env-token")
	t.Setenv("GITHUB_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/alcxyz/grove/actions/workflows":
			writeJSON(t, w, map[string]any{
				"workflows": []map[string]any{{
					"id":   42,
					"path": ".github/workflows/ci.yml",
				}},
			})
		case "/repos/alcxyz/grove/actions/runs":
			writeJSON(t, w, map[string]any{
				"workflow_runs": []map[string]any{{
					"id":            123,
					"run_number":    5,
					"status":        "completed",
					"conclusion":    "success",
					"name":          "CI",
					"workflow_id":   42,
					"head_branch":   "dev",
					"event":         "push",
					"created_at":    "2026-05-05T12:00:00Z",
					"updated_at":    "2026-05-05T12:05:00Z",
					"html_url":      "https://github.com/alcxyz/grove/actions/runs/123",
					"display_title": "dev",
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewGitHubProvider(ProviderConfig{})
	provider.apiURL = server.URL

	runs, err := provider.ListWorkflowRuns("alcxyz/grove")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	run := runs[0]
	if run.WorkflowFile != ".github/workflows/ci.yml" || run.Branch != "dev" || run.Conclusion != "success" {
		t.Fatalf("unexpected workflow run: %+v", run)
	}
}

func TestGitHubCloneURLUsesConfiguredSSHHost(t *testing.T) {
	provider := NewGitHubProvider(ProviderConfig{
		CloneProto: "ssh",
		SSHHost:    "ssh.github.example",
	})

	got := provider.cloneURL("alcxyz", "grove")
	want := "git@ssh.github.example:alcxyz/grove.git"
	if got != want {
		t.Fatalf("cloneURL() = %q, want %q", got, want)
	}
}

func TestGitHubGHAuthModeUsesGHAPI(t *testing.T) {
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
if [ "$1" != "api" ]; then
  echo "unexpected command: $*" >&2
  exit 1
fi
case "$2" in
  /repos/alcxyz/grove/issues*)
    printf '[{"number":8,"title":"API via gh","user":{"login":"alc"},"state":"open","created_at":"2026-05-01T12:00:00Z","updated_at":"2026-05-05T12:00:00Z","html_url":"https://github.com/alcxyz/grove/issues/8"}]'
    ;;
  *)
    echo "unexpected path: $2" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	provider := NewGitHubProvider(ProviderConfig{AuthMode: "gh"})
	issues, err := provider.ListIssues("alcxyz/grove")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Title != "API via gh" {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestGitHubGHAuthModeWrapsAuthFailures(t *testing.T) {
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
echo "gh: To get started with GitHub CLI, run: gh auth login" >&2
exit 1
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider := NewGitHubProvider(ProviderConfig{AuthMode: "gh"})
	_, err := provider.ListIssues("alcxyz/grove")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), ErrNotAuthenticated.Error()) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestGitHubGHAuthModeClonesWithGHRepoClone(t *testing.T) {
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	recordPath := filepath.Join(dir, "args")
	script := `#!/bin/sh
printf '%s\n' "$*" > "` + recordPath + `"
exit 0
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider := NewGitHubProvider(ProviderConfig{AuthMode: "gh"})
	if err := provider.CloneRepo("alcxyz", "grove", filepath.Join(dir, "grove")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if !strings.HasPrefix(got, "repo clone alcxyz/grove ") || !strings.HasSuffix(got, " -- --quiet") {
		t.Fatalf("unexpected gh args: %q", got)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
