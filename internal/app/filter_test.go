package app

import (
	"testing"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

func TestRepoDerivedMapsUseProfileScopedLocalIdentity(t *testing.T) {
	m := Model{
		prs: []model.PR{
			{Repo: "alcxyz/shared", RepoPath: "/tmp/github/shared", Profile: "github"},
		},
		branches: []model.BranchInfo{
			{Repo: "alcxyz/shared", RepoPath: "/tmp/github/shared", Profile: "github"},
		},
		runs: []model.WorkflowRun{
			{Repo: "alcxyz/shared", RepoPath: "/tmp/github/shared", Profile: "github", Status: "completed", Conclusion: "success"},
			{Repo: "alcxyz/shared", RepoPath: "/tmp/forgejo/shared", Profile: "forgejo", Status: "completed", Conclusion: "failure"},
		},
	}

	githubKey := repoDataKey("github", "shared")
	forgejoKey := repoDataKey("forgejo", "shared")

	if got := m.repoPRCounts()[githubKey]; got != 1 {
		t.Fatalf("github PR count = %d, want 1", got)
	}
	if got := m.repoPRCounts()[forgejoKey]; got != 0 {
		t.Fatalf("forgejo PR count = %d, want 0", got)
	}
	if got := m.repoBranchCounts()[githubKey]; got != 1 {
		t.Fatalf("github branch count = %d, want 1", got)
	}
	if got := m.repoBranchCounts()[forgejoKey]; got != 0 {
		t.Fatalf("forgejo branch count = %d, want 0", got)
	}
	if got := m.repoLatestCI()[githubKey]; got != "success" {
		t.Fatalf("github CI = %q, want success", got)
	}
	if got := m.repoLatestCI()[forgejoKey]; got != "failure" {
		t.Fatalf("forgejo CI = %q, want failure", got)
	}
}

func TestLocalRepoNamePrefersRepoPathForRemoteOverrides(t *testing.T) {
	if got := localRepoName("alcxyz/madideal", "/tmp/sites/madideal.bak"); got != "madideal.bak" {
		t.Fatalf("localRepoName with path = %q, want madideal.bak", got)
	}
	if got := localRepoName("alcxyz/madideal", ""); got != "madideal" {
		t.Fatalf("localRepoName without path = %q, want madideal", got)
	}
}

// ── repoBaseName ──────────────────────────────────────────────────────────

func TestRepoBaseName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"org/repo", "repo"},
		{"my-org/my-service", "my-service"},
		{"repo", "repo"}, // no slash → unchanged
		{"a/b/c", "c"},   // last segment
		{"", ""},         // empty
		{"org/", ""},     // trailing slash
	}
	for _, c := range cases {
		if got := repoBaseName(c.in); got != c.want {
			t.Errorf("repoBaseName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── cyclePrefix ───────────────────────────────────────────────────────────

func TestCyclePrefix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"feat: add login", "feat:"},
		{"fix: timeout", "fix:"},
		{"feature/my-thing", "feature"},
		{"bugfix/foo", "bugfix"},
		{"nodelimiter", "nodelimiter"},
		{"", ""},
		{"/leading-slash", "/leading-slash"}, // IndexAny returns 0 → no split
	}
	for _, c := range cases {
		if got := cyclePrefix(c.in); got != c.want {
			t.Errorf("cyclePrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── dateInBucket ──────────────────────────────────────────────────────────

func TestDateInBucket_Today(t *testing.T) {
	now := time.Now()
	if !dateInBucket(now, "today") {
		t.Error("now should be in 'today' bucket")
	}
	yesterday := now.AddDate(0, 0, -1)
	if dateInBucket(yesterday, "today") {
		t.Error("yesterday should not be in 'today' bucket")
	}
}

func TestDateInBucket_Yesterday(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	// Use noon yesterday to avoid boundary issues
	noon := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 12, 0, 0, 0, yesterday.Location())
	if !dateInBucket(noon, "yesterday") {
		t.Error("noon yesterday should be in 'yesterday' bucket")
	}
	if dateInBucket(now, "yesterday") {
		t.Error("now should not be in 'yesterday' bucket")
	}
}

func TestDateInBucket_ZeroTime(t *testing.T) {
	if dateInBucket(time.Time{}, "today") {
		t.Error("zero time should not match any bucket")
	}
	if dateInBucket(time.Time{}, "this week") {
		t.Error("zero time should not match any bucket")
	}
}

func TestDateInBucket_UnknownLabel(t *testing.T) {
	if dateInBucket(time.Now(), "bogus-bucket") {
		t.Error("unknown bucket label should return false")
	}
}

func TestDateInBucket_ThisMonth(t *testing.T) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if !dateInBucket(startOfMonth, "this month") {
		t.Error("start of month should be in 'this month' bucket")
	}
	lastMonth := startOfMonth.AddDate(0, -1, 0)
	if dateInBucket(lastMonth, "this month") {
		t.Error("last month should not be in 'this month' bucket")
	}
}

// ── IsReleaseVersion ──────────────────────────────────────────────────────

func TestIsReleaseVersion_FilterCompat(t *testing.T) {
	valid := []string{"0.1.0", "v1.2.3", "10.20.30"}
	for _, v := range valid {
		if !IsReleaseVersion(v) {
			t.Errorf("IsReleaseVersion(%q) should be true", v)
		}
	}
	invalid := []string{"dev", "abc123", "1.2", "1.2.3.4", "1.2.x"}
	for _, v := range invalid {
		if IsReleaseVersion(v) {
			t.Errorf("IsReleaseVersion(%q) should be false", v)
		}
	}
}
