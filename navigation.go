package main

import (
	"github.com/alcxyz/grove/internal/ui"
)

func (m appModel) listLen() int {
	switch m.activeTab {
	case tabDashboard:
		return len(m.filteredRepos())
	case tabPRs:
		return len(m.filteredPRs())
	case tabBranches:
		return len(m.filteredBranches())
	case tabActivity:
		return len(m.filteredActivity())
	}
	return 0
}

func (m *appModel) clampCursor() {
	top := m.listLen() - 1
	if top < 0 {
		top = 0
	}
	if m.cursor > top {
		m.cursor = top
	}
	m.adjustScroll()
}

// contentHeight returns the number of scrollable lines available in the
// terminal after accounting for fixed chrome (title, tabs, header row,
// status bar, help line).
func (m appModel) contentHeight() int {
	h := m.height - 8 // 2 title + 2 tabs + 1 col-header + 1 blank + 1 status + 1 help
	if h < 1 {
		return 1
	}
	return h
}

// adjustScroll keeps the cursor's visual line inside the visible viewport.
func (m *appModel) adjustScroll() {
	if m.height == 0 {
		return
	}
	ch := m.contentHeight()

	var cvl int
	switch m.activeTab {
	case tabDashboard:
		cvl = ui.RepoCursorLine(m.groupedRepos(), m.cursor)
	case tabPRs:
		cvl = ui.PRCursorLine(m.groupedPRs(), m.cursor)
	case tabBranches:
		cvl = ui.BranchCursorLine(m.groupedBranches(), m.cursor)
	case tabActivity:
		cvl = ui.CommitCursorLine(m.groupedActivity(), m.cursor)
	}

	so := m.scrollOffset[m.activeTab]
	if cvl < so {
		so = cvl
	}
	if cvl >= so+ch {
		so = cvl - ch + 1
	}
	if so < 0 {
		so = 0
	}
	m.scrollOffset[m.activeTab] = so
}

// maxScrollOffset returns the maximum scroll offset for the current tab so the
// last item is visible at the bottom of the viewport.
func (m appModel) maxScrollOffset() int {
	var totalVL int
	switch m.activeTab {
	case tabDashboard:
		groups := m.groupedRepos()
		if len(groups) > 0 {
			g := groups[len(groups)-1]
			if len(g.Repos) > 0 {
				totalVL = ui.RepoCursorLine(groups, g.StartIdx+len(g.Repos)-1) + 1
			}
		}
	case tabPRs:
		groups := m.groupedPRs()
		if len(groups) > 0 {
			g := groups[len(groups)-1]
			if len(g.PRs) > 0 {
				totalVL = ui.PRCursorLine(groups, g.StartIdx+len(g.PRs)-1) + 1
			}
		}
	case tabBranches:
		groups := m.groupedBranches()
		if len(groups) > 0 {
			g := groups[len(groups)-1]
			if len(g.Branches) > 0 {
				totalVL = ui.BranchCursorLine(groups, g.StartIdx+len(g.Branches)-1) + 1
			}
		}
	case tabActivity:
		groups := m.groupedActivity()
		if len(groups) > 0 {
			g := groups[len(groups)-1]
			if len(g.Commits) > 0 {
				totalVL = ui.CommitCursorLine(groups, g.StartIdx+len(g.Commits)-1) + 1
			}
		}
	}
	mso := totalVL - m.contentHeight()
	if mso < 0 {
		return 0
	}
	return mso
}

// termRowToCursor maps a terminal row (0-indexed) from a mouse click to a flat
// item index, or -1 if the row is part of the chrome (title, tabs, col header)
// or a non-selectable visual line (group header / blank separator).
//
// Chrome layout (rows 0-4):
//
//	0  title "grove"
//	1  blank
//	2  tabs + inline filter/sort indicator
//	3  blank
//	4  column header
//	5+ scrollable content
func (m appModel) termRowToCursor(termRow int) int {
	const headerRows = 5
	if termRow < headerRows {
		return -1
	}
	vl := (termRow - headerRows) + m.scrollOffset[m.activeTab]
	switch m.activeTab {
	case tabDashboard:
		return ui.RepoIndexAtVL(m.groupedRepos(), vl)
	case tabPRs:
		return ui.PRIndexAtVL(m.groupedPRs(), vl)
	case tabBranches:
		return ui.BranchIndexAtVL(m.groupedBranches(), vl)
	case tabActivity:
		return ui.CommitIndexAtVL(m.groupedActivity(), vl)
	}
	return -1
}

// tabAtX returns the tab index (0-3) for a click at column x in the tab bar,
// or -1 if x falls outside all tabs.  Both TabStyle and ActiveTabStyle use
// Padding(0, 2), so each tab occupies len(name)+4 columns.
func tabAtX(x int) int {
	cur := 0
	for i, name := range tabNames {
		w := len(name) + 4 // 2-char padding on each side
		if x >= cur && x < cur+w {
			return i
		}
		cur += w
	}
	return -1
}

func (m appModel) groupedRepos() []ui.RepoGroup {
	repos := m.filteredRepos()
	if !m.grouped {
		return []ui.RepoGroup{{Name: "", Repos: repos, StartIdx: 0}}
	}
	return ui.BuildGroups(repos, m.cfg.GroupFor, m.cfg.GroupOrder)
}

func (m appModel) groupedPRs() []ui.PRGroup {
	prs := m.filteredPRs()
	if !m.grouped {
		return []ui.PRGroup{{Name: "", PRs: prs, StartIdx: 0}}
	}
	return ui.BuildPRGroups(prs, m.cfg.GroupFor, m.cfg.GroupOrder)
}

func (m appModel) groupedBranches() []ui.BranchGroup {
	branches := m.filteredBranches()
	if !m.grouped {
		return []ui.BranchGroup{{Name: "", Branches: branches, StartIdx: 0}}
	}
	return ui.BuildBranchGroups(branches, m.cfg.GroupFor, m.cfg.GroupOrder)
}

func (m appModel) groupedActivity() []ui.CommitGroup {
	commits := m.filteredActivity()
	if !m.grouped {
		return []ui.CommitGroup{{Name: "", Commits: commits, StartIdx: 0}}
	}
	return ui.BuildCommitGroups(commits, m.cfg.GroupFor, m.cfg.GroupOrder)
}

// blockHighlightValue returns the match string for the current highlight field
// by inspecting the item at the cursor in grouped order.
func (m appModel) blockHighlightValue() string {
	if m.highlightField == "" {
		return ""
	}
	flat := 0
	switch m.activeTab {
	case tabDashboard:
		if m.highlightField != "subject" {
			return ""
		}
		for _, g := range m.groupedRepos() {
			for _, r := range g.Repos {
				if flat == m.cursor {
					return r.Branch
				}
				flat++
			}
		}
	case tabPRs:
		for _, g := range m.groupedPRs() {
			for _, pr := range g.PRs {
				if flat == m.cursor {
					if m.highlightField == "repo" {
						return repoBaseName(pr.Repo)
					}
					return cyclePrefix(pr.Title)
				}
				flat++
			}
		}
	case tabBranches:
		for _, g := range m.groupedBranches() {
			for _, br := range g.Branches {
				if flat == m.cursor {
					if m.highlightField == "repo" {
						return repoBaseName(br.Repo)
					}
					return cyclePrefix(br.Name)
				}
				flat++
			}
		}
	case tabActivity:
		for _, g := range m.groupedActivity() {
			for _, c := range g.Commits {
				if flat == m.cursor {
					if m.highlightField == "repo" {
						return c.Repo
					}
					return cyclePrefix(c.Subject)
				}
				flat++
			}
		}
	}
	return ""
}

// jumpRepo moves the cursor to the start of the next/previous repo block.
// In tabs 2–4 a "block" is a run of items sharing the same repo name in the
// current display order.  In tab 1 it jumps between repos that need attention
// (dirty or behind origin).
// Uses grouped order so cursor indices match the rendered view.
func (m *appModel) jumpRepo(dir int) {
	var starts []int
	switch m.activeTab {
	case tabDashboard:
		for _, g := range m.groupedRepos() {
			for j, r := range g.Repos {
				if r.Dirty || r.Behind > 0 {
					starts = append(starts, g.StartIdx+j)
				}
			}
		}
	case tabPRs:
		last := ""
		for _, g := range m.groupedPRs() {
			for j, pr := range g.PRs {
				if name := repoBaseName(pr.Repo); name != last {
					starts = append(starts, g.StartIdx+j)
					last = name
				}
			}
		}
	case tabBranches:
		last := ""
		for _, g := range m.groupedBranches() {
			for j, br := range g.Branches {
				if name := repoBaseName(br.Repo); name != last {
					starts = append(starts, g.StartIdx+j)
					last = name
				}
			}
		}
	case tabActivity:
		last := ""
		for _, g := range m.groupedActivity() {
			for j, c := range g.Commits {
				if c.Repo != last {
					starts = append(starts, g.StartIdx+j)
					last = c.Repo
				}
			}
		}
	}
	m.jumpTo(starts, dir)
}

// jumpSubject moves the cursor to the next/previous block of items sharing
// the same subject prefix (commit message / PR title / branch name).
// On the dashboard it jumps by branch name.
func (m *appModel) jumpSubject(dir int) {
	var starts []int
	switch m.activeTab {
	case tabDashboard:
		last := ""
		for _, g := range m.groupedRepos() {
			for j, r := range g.Repos {
				if r.Branch != last {
					starts = append(starts, g.StartIdx+j)
					last = r.Branch
				}
			}
		}
	case tabPRs:
		last := ""
		for _, g := range m.groupedPRs() {
			for j, pr := range g.PRs {
				if p := cyclePrefix(pr.Title); p != last {
					starts = append(starts, g.StartIdx+j)
					last = p
				}
			}
		}
	case tabBranches:
		last := ""
		for _, g := range m.groupedBranches() {
			for j, br := range g.Branches {
				if p := cyclePrefix(br.Name); p != last {
					starts = append(starts, g.StartIdx+j)
					last = p
				}
			}
		}
	case tabActivity:
		last := ""
		for _, g := range m.groupedActivity() {
			for j, c := range g.Commits {
				if p := cyclePrefix(c.Subject); p != last {
					starts = append(starts, g.StartIdx+j)
					last = p
				}
			}
		}
	}
	m.jumpTo(starts, dir)
}

// jumpTo is the shared navigation kernel for jumpRepo/jumpSubject/jumpGroup.
func (m *appModel) jumpTo(starts []int, dir int) {
	if len(starts) == 0 {
		return
	}
	cur := len(starts) - 1
	for i, si := range starts {
		if m.cursor < si {
			cur = i - 1
			break
		}
	}
	next := cur + dir
	if next < 0 {
		next = 0
	}
	if next >= len(starts) {
		next = len(starts) - 1
	}
	m.cursor = starts[next]
	m.adjustScroll()
	// Pull the viewport up so the group header / blank separator above the first
	// item of the target block is visible (adjustScroll pins the cursor to the
	// very top, which would hide the header line sitting one row above it).
	t := m.activeTab
	if so := m.scrollOffset[t]; so >= 2 {
		m.scrollOffset[t] = so - 2
	} else {
		m.scrollOffset[t] = 0
	}
}

// jumpGroup moves the cursor to the start of the next (+1) or previous (-1) group.
func (m *appModel) jumpGroup(dir int) {
	var starts []int
	switch m.activeTab {
	case tabDashboard:
		for _, g := range m.groupedRepos() {
			starts = append(starts, g.StartIdx)
		}
	case tabPRs:
		starts = ui.PRGroupStarts(m.groupedPRs())
	case tabBranches:
		starts = ui.BranchGroupStarts(m.groupedBranches())
	case tabActivity:
		starts = ui.CommitGroupStarts(m.groupedActivity())
	}
	m.jumpTo(starts, dir)
}
