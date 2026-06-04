package forge

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestForgejoListMilestonesUsesMilestonesEndpoint(t *testing.T) {
	requestURIs := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURIs <- r.URL.RequestURI()
		if r.URL.Path != "/api/v1/repos/alcxyz/grove/milestones" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 4,
				"title": "v1.0",
				"description": "Release train",
				"state": "open",
				"open_issues": 2,
				"closed_issues": 3,
				"due_on": "2026-06-30T00:00:00Z",
				"created_at": "2026-06-01T12:00:00Z",
				"updated_at": "2026-06-05T12:00:00Z",
				"closed_at": null
			}
		]`))
	}))
	defer server.Close()

	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	milestones, err := provider.ListMilestones("alcxyz/grove")
	if err != nil {
		t.Fatal(err)
	}
	requestURI := <-requestURIs
	if requestURI != "/api/v1/repos/alcxyz/grove/milestones?state=open&page=1&limit=50" {
		t.Fatalf("unexpected request URI %q", requestURI)
	}
	if len(milestones) != 1 {
		t.Fatalf("len(milestones) = %d, want 1", len(milestones))
	}
	ms := milestones[0]
	if ms.Repo != "alcxyz/grove" || ms.Number != 4 || ms.Title != "v1.0" || ms.State != "open" {
		t.Fatalf("unexpected milestone: %+v", ms)
	}
	if ms.OpenIssues != 2 || ms.ClosedIssues != 3 {
		t.Fatalf("issue counts = (%d, %d), want (2, 3)", ms.OpenIssues, ms.ClosedIssues)
	}
	if ms.DueOn == nil || !ms.DueOn.Equal(time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("DueOn = %v, want 2026-06-30", ms.DueOn)
	}
	if ms.URL != server.URL+"/alcxyz/grove/milestones/4" {
		t.Fatalf("URL = %q", ms.URL)
	}
}

func TestForgejoListWorkflowRunsUsesTasksEndpoint(t *testing.T) {
	requestURIs := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURIs <- r.URL.RequestURI()
		if r.URL.Path != "/api/v1/repos/alcxyz/grove/actions/tasks" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"workflow_runs": [
				{
					"id": 4023,
					"name": "cloudflare",
					"head_branch": "main",
					"head_sha": "abc123",
					"run_number": 376,
					"event": "push",
					"display_title": "Add client Gatus status pages",
					"status": "success",
					"workflow_id": "cloudflare-tofu.yml",
					"url": "https://git.alc.xyz/alcxyz/grove/actions/runs/376",
					"created_at": "2026-06-03T14:52:23+02:00",
					"updated_at": "2026-06-03T14:53:21+02:00",
					"run_started_at": "2026-06-03T14:52:23+02:00"
				}
			],
			"total_count": 1
		}`))
	}))
	defer server.Close()

	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	runs, err := provider.ListWorkflowRuns("alcxyz/grove")
	if err != nil {
		t.Fatal(err)
	}
	requestURI := <-requestURIs
	if requestURI != "/api/v1/repos/alcxyz/grove/actions/tasks?page=1&limit=20" {
		t.Fatalf("unexpected request URI %q", requestURI)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 workflow run, got %d", len(runs))
	}
	run := runs[0]
	if run.Repo != "alcxyz/grove" ||
		run.WorkflowName != "cloudflare" ||
		run.WorkflowFile != "cloudflare-tofu.yml" ||
		run.Branch != "main" ||
		run.Event != "push" ||
		run.Status != "completed" ||
		run.Conclusion != "success" ||
		run.RunID != 4023 ||
		run.Number != 376 ||
		run.URL != "https://git.alc.xyz/alcxyz/grove/actions/runs/376" ||
		run.StartedAt.IsZero() ||
		run.UpdatedAt.IsZero() {
		t.Fatalf("unexpected workflow run: %+v", run)
	}
}

func TestForgejoWorkflowStatusMapsTaskStatuses(t *testing.T) {
	cases := []struct {
		status         string
		wantStatus     string
		wantConclusion string
	}{
		{"success", "completed", "success"},
		{"failure", "completed", "failure"},
		{"cancelled", "completed", "cancelled"},
		{"skipped", "completed", "skipped"},
		{"timed_out", "completed", "timed_out"},
		{"startup_failure", "completed", "startup_failure"},
		{"running", "in_progress", ""},
		{"waiting", "queued", ""},
		{"pending", "queued", ""},
		{"blocked", "queued", ""},
	}
	for _, c := range cases {
		gotStatus, gotConclusion := forgejoWorkflowStatus(c.status)
		if gotStatus != c.wantStatus || gotConclusion != c.wantConclusion {
			t.Fatalf("forgejoWorkflowStatus(%q) = (%q, %q), want (%q, %q)",
				c.status, gotStatus, gotConclusion, c.wantStatus, c.wantConclusion)
		}
	}
}
