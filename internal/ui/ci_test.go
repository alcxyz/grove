package ui

import (
	"strings"
	"testing"

	"github.com/alcxyz/grove/internal/model"
)

// ── CIStatusIcon ──────────────────────────────────────────────────────────

func TestCIStatusIcon(t *testing.T) {
	cases := []struct {
		status     string
		conclusion string
		wantStyle  string // substring we expect in the rendered output
	}{
		{"completed", "success", "✓"},
		{"completed", "failure", "✗"},
		{"completed", "timed_out", "✗"},
		{"completed", "startup_failure", "✗"},
		{"completed", "cancelled", "⊘"},
		{"completed", "skipped", "—"},
		{"completed", "", "—"},
		{"in_progress", "", "●"},
		{"queued", "", "●"},
		{"waiting", "", "●"},
	}
	for _, c := range cases {
		got := CIStatusIcon(c.status, c.conclusion)
		// Strip ANSI to check the visible character
		plain := stripANSI(got)
		if !strings.Contains(plain, c.wantStyle) {
			t.Errorf("CIStatusIcon(%q, %q) = %q (plain: %q), want to contain %q",
				c.status, c.conclusion, got, plain, c.wantStyle)
		}
	}
}

// ── formatCIStatus ────────────────────────────────────────────────────────

func TestFormatCIStatus(t *testing.T) {
	cases := []struct {
		status     string
		conclusion string
		wantPlain  string
	}{
		{"completed", "success", "✓ success"},
		{"completed", "failure", "✗ failure"},
		{"completed", "timed_out", "✗ timed_out"},
		{"completed", "cancelled", "⊘ cancelled"},
		{"completed", "skipped", "— skipped"},
		{"in_progress", "", "● in_progress"},
		{"queued", "", "● queued"},
	}
	for _, c := range cases {
		got := formatCIStatus(c.status, c.conclusion)
		plain := stripANSI(got)
		if plain != c.wantPlain {
			t.Errorf("formatCIStatus(%q, %q) plain = %q, want %q",
				c.status, c.conclusion, plain, c.wantPlain)
		}
	}
}

// ── truncate ──────────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello w…"},
		{"", 5, ""},
		{"ab", 2, "ab"},
		{"abc", 2, "a…"},
	}
	for _, c := range cases {
		got := truncate(c.in, c.n)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

// ── RenderCI empty state ──────────────────────────────────────────────────

func TestRenderCI_Empty(t *testing.T) {
	out := RenderCI(nil, 0, 80, 0, 40, "", "")
	plain := stripANSI(out)
	if !strings.Contains(plain, "No CI run data") {
		t.Errorf("expected empty-state message, got: %q", plain)
	}
}

func TestRenderCI_SingleRun(t *testing.T) {
	run := model.WorkflowRun{
		Repo:         "acme/my-service",
		WorkflowName: "ci.yml",
		Branch:       "main",
		Status:       "completed",
		Conclusion:   "success",
		Event:        "push",
	}
	groups := []CIGroup{{Name: "", Runs: []model.WorkflowRun{run}, StartIdx: 0}}
	out := RenderCI(groups, 0, 120, 0, 40, "", "")
	plain := stripANSI(out)
	if !strings.Contains(plain, "my-service") {
		t.Errorf("expected repo name in output, got: %q", plain)
	}
	if !strings.Contains(plain, "ci.yml") {
		t.Errorf("expected workflow name in output, got: %q", plain)
	}
}

func TestRenderCIShowsUngroupedTotalRunCount(t *testing.T) {
	groups := []CIGroup{{
		Name: "",
		Runs: []model.WorkflowRun{
			{Repo: "acme/api", WorkflowName: "ci", Status: "completed", Conclusion: "success"},
			{Repo: "acme/web", WorkflowName: "ci", Status: "completed", Conclusion: "success"},
		},
		StartIdx: 0,
	}}

	out := RenderCI(groups, 0, 120, 0, 40, "", "")
	plain := stripANSI(out)
	if !strings.Contains(plain, "2 runs") {
		t.Fatalf("expected total run count in output, got: %q", plain)
	}
}

func TestRenderCIShowsGroupedTotalRunCount(t *testing.T) {
	groups := []CIGroup{
		{
			Name: "GitHub",
			Runs: []model.WorkflowRun{
				{Repo: "acme/api", WorkflowName: "ci", Status: "completed", Conclusion: "success"},
			},
			StartIdx: 0,
		},
		{
			Name: "Forgejo",
			Runs: []model.WorkflowRun{
				{Repo: "acme/web", WorkflowName: "ci", Status: "completed", Conclusion: "success"},
				{Repo: "acme/ops", WorkflowName: "deploy", Status: "completed", Conclusion: "success"},
			},
			StartIdx: 1,
		},
	}

	out := RenderCI(groups, 0, 120, 0, 40, "", "")
	plain := stripANSI(out)
	if !strings.Contains(plain, "3 runs") {
		t.Fatalf("expected grouped total run count in output, got: %q", plain)
	}
}

// ── BuildCIGroups ─────────────────────────────────────────────────────────

func TestBuildCIGroups(t *testing.T) {
	runs := []model.WorkflowRun{
		{Repo: "org/svc-api", WorkflowName: "ci"},
		{Repo: "org/svc-worker", WorkflowName: "ci"},
		{Repo: "org/infra-vpc", WorkflowName: "ci"},
	}
	groupFor := func(name string) string {
		if strings.HasPrefix(name, "svc-") {
			return "Services"
		}
		return "Other"
	}
	groupOrder := func(name string) int {
		if name == "Services" {
			return 0
		}
		return 1
	}
	groups := BuildCIGroups(runs, groupFor, groupOrder)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "Services" {
		t.Errorf("expected first group Services, got %q", groups[0].Name)
	}
	if len(groups[0].Runs) != 2 {
		t.Errorf("expected 2 runs in Services, got %d", len(groups[0].Runs))
	}
	if groups[1].StartIdx != 2 {
		t.Errorf("expected StartIdx=2 for second group, got %d", groups[1].StartIdx)
	}
}

// ── CICursorLine ─────────────────────────────────────────────────────────

func TestCICursorLine(t *testing.T) {
	runs := func(n int) []model.WorkflowRun {
		s := make([]model.WorkflowRun, n)
		return s
	}
	groups := []CIGroup{
		{Name: "A", Runs: runs(2), StartIdx: 0},
		{Name: "B", Runs: runs(2), StartIdx: 2},
	}
	// Group A: header at vl=0, items at vl=1,2
	// Blank sep before B: vl=3, header B: vl=4, items at vl=5,6
	if got := CICursorLine(groups, 0); got != 1 {
		t.Errorf("cursor 0 → vl %d, want 1", got)
	}
	if got := CICursorLine(groups, 1); got != 2 {
		t.Errorf("cursor 1 → vl %d, want 2", got)
	}
	if got := CICursorLine(groups, 2); got != 5 {
		t.Errorf("cursor 2 → vl %d, want 5", got)
	}
	if got := CICursorLine(groups, 3); got != 6 {
		t.Errorf("cursor 3 → vl %d, want 6", got)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

// stripANSI removes ANSI escape sequences for plain-text comparison in tests.
func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
