package forge

import (
	"errors"
	"fmt"
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
	want := "api\n-i\nhttps://git.alc.xyz/api/v1/repos/alcxyz/grove/issues?state=open&type=issues&page=1&limit=50\n"
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

func TestForgejoListPRsFollowsTotalCountPastServerPageCap(t *testing.T) {
	const total, serverCap = 45, 30
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/team/repo/pulls" {
			http.NotFound(w, r)
			return
		}
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			page = int(p[0] - '0')
		}
		start := (page - 1) * serverCap
		var items []map[string]any
		for n := start; n < total && n < start+serverCap; n++ {
			items = append(items, map[string]any{"number": n + 1, "title": "pr"})
		}
		w.Header().Set("X-Total-Count", "45")
		if items == nil {
			items = []map[string]any{}
		}
		writeJSON(t, w, items)
	}))
	defer server.Close()

	provider, err := NewForgejoProvider(ProviderConfig{Forge: "forgejo", InstanceURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	prs, err := provider.ListPRs("team/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != total {
		t.Fatalf("len(prs) = %d, want %d", len(prs), total)
	}
}

func TestForgejoPaginationReportsLaterPageErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		items := make([]map[string]any, forgejoPageLimit)
		for i := range items {
			items[i] = map[string]any{"number": i + 1}
		}
		writeJSON(t, w, items)
	}))
	defer server.Close()

	provider, err := NewForgejoProvider(ProviderConfig{Forge: "forgejo", InstanceURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ListIssues("team/repo"); err == nil {
		t.Fatal("expected error from failed second page, got silently truncated result")
	}
}

func TestForgejoUnreadableTokenFileIsAuthError(t *testing.T) {
	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://forge.invalid",
		TokenFile:   filepath.Join(t.TempDir(), "missing-token"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ListPRs("team/repo"); !errors.Is(err, ErrNotAuthenticated) {
		t.Fatalf("err = %v, want ErrNotAuthenticated", err)
	}
}

func TestForgejoWorkflowRunsErrorHandling(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		code    int
		wantErr bool
	}{
		{http.StatusNotFound, false},
		{http.StatusForbidden, false},
		{http.StatusUnauthorized, true},
		{http.StatusInternalServerError, true},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
		}))
		provider, err := NewForgejoProvider(ProviderConfig{Forge: "forgejo", InstanceURL: server.URL, TokenFile: tokenPath})
		if err != nil {
			t.Fatal(err)
		}
		runs, err := provider.ListWorkflowRuns("team/repo")
		server.Close()
		if (err != nil) != tc.wantErr {
			t.Errorf("status %d: err = %v, wantErr %v", tc.code, err, tc.wantErr)
		}
		if runs != nil {
			t.Errorf("status %d: runs = %v, want nil", tc.code, runs)
		}
	}
}

func TestForgejoWorkflowRunsTreatAnonymous401AsNoRuns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	provider, err := NewForgejoProvider(ProviderConfig{Forge: "forgejo", InstanceURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if runs, err := provider.ListWorkflowRuns("team/repo"); err != nil || runs != nil {
		t.Fatalf("ListWorkflowRuns = %v, %v; want nil, nil without a token", runs, err)
	}
}

// writeFakeTea installs a tea stub on PATH that runs script with the request
// URL in $url.
func writeFakeTea(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\nurl=\"$3\"\n" + script
	if err := os.WriteFile(filepath.Join(dir, "tea"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newTeaProvider(t *testing.T) *ForgejoProvider {
	t.Helper()
	provider, err := NewForgejoProvider(ProviderConfig{Forge: "forgejo", InstanceURL: "https://forge.invalid", AuthMode: "tea"})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestForgejoTeaWorkflowRunsStatusHandling(t *testing.T) {
	for _, tc := range []struct {
		code    int
		wantErr bool
	}{
		{http.StatusNotFound, false},
		{http.StatusForbidden, false},
		{http.StatusUnauthorized, true},
		{http.StatusInternalServerError, true},
	} {
		writeFakeTea(t, fmt.Sprintf("echo 'HTTP/2.0 %d Status' >&2\necho '{\"message\":\"x\"}'\n", tc.code))
		runs, err := newTeaProvider(t).ListWorkflowRuns("team/repo")
		if (err != nil) != tc.wantErr {
			t.Errorf("status %d: err = %v, wantErr %v", tc.code, err, tc.wantErr)
		}
		if runs != nil {
			t.Errorf("status %d: runs = %v, want nil", tc.code, runs)
		}
	}
}

func TestForgejoTeaWorkflowRunsSurfaceTeaFailures(t *testing.T) {
	writeFakeTea(t, "echo 'Error: connection refused' >&2\nexit 1\n")
	if _, err := newTeaProvider(t).ListWorkflowRuns("team/repo"); err == nil {
		t.Fatal("expected error when tea api fails")
	}
}

func TestForgejoTeaPaginationFollowsTotalCount(t *testing.T) {
	// The server caps pages at 30 items, below grove's limit of 50.
	writeFakeTea(t, `echo 'HTTP/1.1 200 OK' >&2
echo 'X-Total-Count: 45' >&2
case "$url" in
*page=1\&*) start=1; n=30 ;;
*page=2\&*) start=31; n=15 ;;
*) n=0 ;;
esac
printf '['
i=0
while [ "$i" -lt "$n" ]; do
	[ "$i" -gt 0 ] && printf ','
	printf '{"number":%d,"title":"pr","user":{"login":"a"},"head":{"ref":"b"},"state":"open","updated_at":"2026-05-05T12:00:00Z"}' $((start + i))
	i=$((i + 1))
done
printf ']'
`)
	prs, err := newTeaProvider(t).ListPRs("team/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 45 {
		t.Fatalf("got %d PRs, want 45", len(prs))
	}
}

func TestForgejoTeaListReposFallsBackToUser(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    int
		login   string
		wantErr bool
	}{
		{"not an org", http.StatusNotFound, "someone", false},
		// A token without the read:organization scope, listing its own account.
		{"own account without org scope", http.StatusForbidden, "alc", false},
		// The same 403 for another owner may be a real org permission error.
		{"other owner forbidden", http.StatusForbidden, "someone", true},
		{"server error", http.StatusInternalServerError, "alc", true},
	} {
		writeFakeTea(t, fmt.Sprintf(`case "$url" in
*/orgs/*) echo 'HTTP/2.0 %d Status' >&2; echo '{"message":"x"}' ;;
*/users/*) echo 'HTTP/2.0 200 OK' >&2; echo '[{"name":"repo"}]' ;;
*/user) echo 'HTTP/2.0 200 OK' >&2; echo '{"login":"%s"}' ;;
esac
`, tc.code, tc.login))
		names, err := newTeaProvider(t).ListRepos("alc", nil)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got %v", tc.name, names)
			}
			continue
		}
		if err != nil || len(names) != 1 || names[0] != "repo" {
			t.Errorf("%s: ListRepos = %v, %v; want [repo]", tc.name, names, err)
		}
	}
}

func TestForgejoTeaWorkflowRunsSurfaceMissingTea(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	provider, err := NewForgejoProvider(ProviderConfig{Forge: "forgejo", InstanceURL: "https://forge.invalid", AuthMode: "tea"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ListWorkflowRuns("team/repo"); err == nil {
		t.Fatal("expected error when tea cannot run")
	}
}
