package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

func TestRenderMilestonesEmpty(t *testing.T) {
	out := RenderMilestones(nil, 0, 100, 0, 40, "", "")
	plain := stripANSI(out)
	if !strings.Contains(plain, "No milestones found") {
		t.Fatalf("expected empty-state message, got: %q", plain)
	}
}

func TestRenderMilestonesSingleMilestone(t *testing.T) {
	due := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	groups := []MilestoneGroup{{
		Name: "",
		Milestones: []model.Milestone{{
			Repo:         "alcxyz/grove",
			Title:        "v1.0",
			State:        "open",
			OpenIssues:   2,
			ClosedIssues: 3,
			DueOn:        &due,
			UpdatedAt:    time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC),
		}},
		StartIdx: 0,
	}}

	out := RenderMilestones(groups, 0, 120, 0, 40, "", "")
	plain := stripANSI(out)
	for _, want := range []string{"1 milestone", "grove", "v1.0", "60%", "2026-06-30"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected %q in output, got: %q", want, plain)
		}
	}
}

func TestRenderMilestonesShowsGroupedTotalCount(t *testing.T) {
	groups := []MilestoneGroup{
		{
			Name:       "GitHub",
			Milestones: []model.Milestone{{Repo: "alcxyz/grove", Title: "v1.0"}},
			StartIdx:   0,
		},
		{
			Name: "Forgejo",
			Milestones: []model.Milestone{
				{Repo: "alcxyz/gitops", Title: "infra"},
				{Repo: "alcxyz/site", Title: "content"},
			},
			StartIdx: 1,
		},
	}

	out := RenderMilestones(groups, 0, 120, 0, 40, "", "")
	plain := stripANSI(out)
	if !strings.Contains(plain, "3 milestones") {
		t.Fatalf("expected grouped total milestone count in output, got: %q", plain)
	}
}
