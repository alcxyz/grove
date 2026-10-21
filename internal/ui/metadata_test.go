package ui

import (
	"testing"

	"github.com/alcxyz/grove/internal/model"
	"github.com/charmbracelet/lipgloss"
)

func TestMetadataIsPlainBeforeStyling(t *testing.T) {
	dirty := "Blå\x1b]0;hidden\a\x1b[2J\x1b[31m界\x1b[0m\b\r\x7f\u0085\nnext"
	clean := OneLine(dirty)
	for _, selected := range []bool{false, true} {
		for name, render := range map[string]func(string) string{
			"repo": func(s string) string {
				return renderRepoRow(model.Repo{Name: s, Branch: s, LastAuthor: s}, selected, 0, 0, 0, "", "subject", clean, 60, 60, 60, 20)
			},
			"pr": func(s string) string {
				return renderPRRow(model.PR{Repo: s, Title: s, Author: s}, selected, "subject", clean, 60, 60, 60, 20)
			},
			"ci": func(s string) string {
				return renderCIRow(model.WorkflowRun{Repo: s, WorkflowName: s, Branch: s, Event: s, Status: s, Conclusion: s}, selected, "subject", clean, 60, 60, 60, 20)
			},
			"issues": func(s string) string {
				return renderIssueRow(model.Issue{Repo: s, Title: s, Author: s, Labels: []string{s}, Assignees: []string{s}, Milestone: s}, selected, "subject", clean, 60, 60, 60, 60, 60, 60, 20)
			},
			"milestones": func(s string) string {
				return renderMilestoneRow(model.Milestone{Repo: s, Title: s, State: s}, selected, "subject", clean, 60, 60, 60, 12, 12, 12, 12, 20)
			},
			"activity": func(s string) string {
				return RenderActivity([]CommitGroup{{Name: s, Commits: []model.Commit{{Repo: s, Hash: s, Subject: s, Author: s}}}}, 0, 240, 0, 40, "subject", clean)
			},
			"branches": func(s string) string {
				return RenderBranches([]BranchGroup{{Name: s, Branches: []model.BranchInfo{{Repo: s, Name: s, Author: s}}}}, 0, 240, 0, 40, nil, "subject", clean)
			},
			"detail": func(s string) string {
				return RenderRepoDetail(model.Repo{Name: s, Branch: s, Path: s}, []model.Commit{{Hash: s, Subject: s, Author: s}}, []model.PR{{Title: s, Author: s, Branch: s}}, []model.Issue{{Title: s, Author: s, Labels: []string{s}}}, []string{s}, []model.BranchInfo{{Name: s, Author: s}}, []model.WorkflowRun{{WorkflowName: s, Branch: s, Status: s, Conclusion: s}}, model.RepoStats{}, 120, 5)
			},
			"errors": func(s string) string {
				return RenderErrors([]string{s}) + RenderDependencyWarnings([]string{s}, 5) + RenderAuthError("forgejo", []string{s})
			},
			"group": func(s string) string { return groupHeader(s, 120) },
			"tabs": func(s string) string {
				return RenderTabs([]string{s, s}, 0, 120) + RenderProfileTabs([]string{s, s}, 0)
			},
			"cycle":       func(s string) string { return RenderCycleFilter("author", s, 1, 2) },
			"diff header": func(s string) string { return RenderDiff(s, s, "+normal", false) },
		} {
			t.Run(name+map[bool]string{true: " selected", false: " normal"}[selected], func(t *testing.T) {
				got, want := render(dirty), render(clean)
				if got != want {
					t.Fatalf("metadata display differs from plain text\ngot  %q\nwant %q", got, want)
				}
			})
		}
	}
}

func TestMetadataTruncateUsesDisplayWidth(t *testing.T) {
	got := truncate("\x1b[31m世界abcd\x1b[0m", 4)
	if lipgloss.Width(got) > 4 || got != "世…" {
		t.Fatalf("truncate = %q (width %d)", got, lipgloss.Width(got))
	}
}

func TestFilterPreservesSpaces(t *testing.T) {
	if got, want := RenderFilter("deploy  release\x1b[2J"), RenderFilter("deploy  release"); got != want {
		t.Fatalf("filter = %q, want %q", got, want)
	}
}

func TestPlainDiffPreservesLines(t *testing.T) {
	got := RenderDiff("example", "abc", "+Blå\x1b]0;hidden\a\n-界\b", false)
	want := RenderDiff("example", "abc", "+Blå\n-界", false)
	if got != want {
		t.Fatalf("diff = %q, want %q", got, want)
	}
}
