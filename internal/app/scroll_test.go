package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/model"
	"github.com/alcxyz/grove/internal/ui"
)

// TestJumpGroupKeepsCursorVisible verifies that after jumping between groups
// the cursor's visual line stays within the scroll viewport.
func TestJumpGroupKeepsCursorVisible(t *testing.T) {
	// Build 5 groups of 8 PRs each (40 total), with profile-based grouping.
	profiles := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	var prs []model.PR
	for _, p := range profiles {
		for i := 0; i < 8; i++ {
			prs = append(prs, model.PR{
				Repo:    p + "/repo",
				Title:   "pr",
				Profile: p,
			})
		}
	}

	m := Model{
		activeTab:     tabPRs,
		prs:           prs,
		grouped:       true,
		height:        30,
		width:         120,
		scrollOffset:  map[tab]int{tabPRs: 0},
		activeProfile: -1, // "All" mode to get profile grouping
		tabSort:       map[tab]tabSortState{},
	}
	m.cfg.Profiles = []config.Profile{
		{Name: "alpha"}, {Name: "beta"}, {Name: "gamma"},
		{Name: "delta"}, {Name: "epsilon"},
	}

	sh := m.scrollHeight()
	groups := m.groupedPRs()

	// Jump forward through all groups repeatedly, verify cursor stays visible.
	for i := 0; i < 20; i++ {
		m.jumpGroup(+1)
		cvl := ui.PRCursorLine(groups, m.cursor)
		so := m.scrollOffset[tabPRs]
		if cvl < so || cvl >= so+sh {
			t.Fatalf("forward jump %d: cursor vl=%d outside viewport [%d, %d) (cursor=%d)",
				i, cvl, so, so+sh, m.cursor)
		}
	}

	// Jump backward through all groups, verify cursor stays visible.
	for i := 0; i < 20; i++ {
		m.jumpGroup(-1)
		cvl := ui.PRCursorLine(groups, m.cursor)
		so := m.scrollOffset[tabPRs]
		if cvl < so || cvl >= so+sh {
			t.Fatalf("backward jump %d: cursor vl=%d outside viewport [%d, %d) (cursor=%d)",
				i, cvl, so, so+sh, m.cursor)
		}
	}
}

// TestDependencyWarningsKeepDashboardGeometry verifies that the startup-warning
// block above the dashboard leaves keyboard scrolling and mouse hit testing in
// step with what is drawn, whether the tabs fit on one row or wrap.
func TestDependencyWarningsKeepDashboardGeometry(t *testing.T) {
	for _, width := range []int{120, 200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			testDependencyWarningGeometry(t, width)
		})
	}
}

func testDependencyWarningGeometry(t *testing.T, width int) {
	m := newTestModel()
	m.width = width
	m.height = 20
	m.loading = false
	m.depWarnings = []string{"gh not found on PATH"}
	for i := 0; i < 30; i++ {
		m.repos = append(m.repos, model.Repo{
			Name: fmt.Sprintf("repo%02d", i), Path: fmt.Sprintf("/tmp/repo%02d", i), Branch: "main", Profile: "test",
		})
	}
	repos := m.filteredRepos()

	// Moving to the last repo must scroll it into view.
	m.cursor = len(repos) - 1
	m.adjustScroll()
	last := repos[len(repos)-1].Name
	if !strings.Contains(m.View(), last) {
		t.Fatalf("cursor on %s, but it is not drawn", last)
	}
	if so, mso := m.scrollOffset[tabDashboard], m.maxScrollOffset(); so > mso {
		t.Fatalf("scroll offset %d beyond max %d", so, mso)
	}

	// Clicking a drawn repo row selects that repo.
	clicked := 0
	for row, line := range strings.Split(m.View(), "\n") {
		for idx, repo := range repos {
			if !strings.Contains(line, repo.Name+" ") {
				continue
			}
			clicked++
			if got := m.termRowToCursor(row); got != idx {
				t.Errorf("click on row %d showing %s selects index %d, want %d", row, repo.Name, got, idx)
			}
		}
	}
	if clicked == 0 {
		t.Fatal("no repo rows found in the view")
	}
}

func TestAllModeGroupsRowsByRowProfileWhenRepoNamesCollide(t *testing.T) {
	m := Model{
		activeTab:     tabPRs,
		grouped:       true,
		activeProfile: -1,
		tabSort:       map[tab]tabSortState{},
		prs: []model.PR{
			{Repo: "alcxyz/shared", Profile: "forgejo"},
			{Repo: "alcxyz/shared", Profile: "github"},
		},
	}
	m.cfg.Profiles = []config.Profile{{Name: "forgejo"}, {Name: "github"}}

	groups := m.groupedPRs()
	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}
	if groups[0].Name != "forgejo" || groups[1].Name != "github" {
		t.Fatalf("groups = %q, %q; want forgejo, github", groups[0].Name, groups[1].Name)
	}
}
