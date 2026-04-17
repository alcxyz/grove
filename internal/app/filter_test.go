package app

import (
	"testing"
	"time"
)

// ── repoBaseName ──────────────────────────────────────────────────────────

func TestRepoBaseName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"org/repo", "repo"},
		{"my-org/my-service", "my-service"},
		{"repo", "repo"},               // no slash → unchanged
		{"a/b/c", "c"},                 // last segment
		{"", ""},                       // empty
		{"org/", ""},                   // trailing slash
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

// ── isSemver ──────────────────────────────────────────────────────────────

func TestIsSemver(t *testing.T) {
	valid := []string{"0.1.0", "1.2.3", "10.20.30"}
	for _, v := range valid {
		if !isSemver(v) {
			t.Errorf("isSemver(%q) should be true", v)
		}
	}
	invalid := []string{"dev", "abc123", "1.2", "1.2.3.4", "v1.2.3", "1.2.x"}
	for _, v := range invalid {
		if isSemver(v) {
			t.Errorf("isSemver(%q) should be false", v)
		}
	}
}
