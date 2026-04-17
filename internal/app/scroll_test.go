package app

import (
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
