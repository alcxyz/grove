package main

import (
	"github.com/alcxyz/grove/internal/config"
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
	case tabCI:
		return len(m.filteredRuns())
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

// showProfileBar returns true when the profile tab bar should be rendered.
// Only shown when there are multiple profiles (single profile = no bar needed).
func (m appModel) showProfileBar() bool {
	return len(m.cfg.Profiles) > 1
}

// activeProfileObj returns the currently active profile, or the first profile
// as a fallback. Used for single-profile grouping.
func (m appModel) activeProfileObj() config.Profile {
	if m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles) {
		return m.cfg.Profiles[m.activeProfile]
	}
	if len(m.cfg.Profiles) > 0 {
		return m.cfg.Profiles[0]
	}
	return config.Profile{}
}

// profileOrder returns a func(string) int for ordering groups by profile index.
func (m appModel) profileOrder() func(string) int {
	idx := make(map[string]int, len(m.cfg.Profiles))
	for i, p := range m.cfg.Profiles {
		idx[p.Name] = i
	}
	return func(name string) int {
		if i, ok := idx[name]; ok {
			return i
		}
		return 9999
	}
}

// profileAtX returns the profile index (0..N-1 for profiles, N for "All") for a
// mouse click at column x in the profile tab bar, or -2 if outside all tabs.
// "All" maps to index len(cfg.Profiles) in the rendered bar, but is returned as -1.
func (m appModel) profileAtX(x int) int {
	names := m.profileTabNames()
	cur := 0
	for i, name := range names {
		w := len(name) + 2 // padding(0,1) = 1 each side
		if x >= cur && x < cur+w {
			if i == len(names)-1 {
				return -1 // "All"
			}
			return i
		}
		cur += w
	}
	return -2
}

// profileTabNames returns the display names for the profile tab bar.
func (m appModel) profileTabNames() []string {
	names := make([]string, len(m.cfg.Profiles)+1)
	for i, p := range m.cfg.Profiles {
		names[i] = p.Name
	}
	names[len(m.cfg.Profiles)] = "All"
	return names
}

// activeProfileTabIdx returns the render index for the active profile
// (0..N-1 for profiles, N for "All").
func (m appModel) activeProfileTabIdx() int {
	if m.activeProfile == -1 {
		return len(m.cfg.Profiles)
	}
	return m.activeProfile
}

// contentHeight returns the number of scrollable lines available in the
// terminal after accounting for fixed chrome (title, tabs, owl bottom area).
func (m appModel) contentHeight() int {
	extra := 0
	if m.showProfileBar() {
		extra = 1
	}
	// 5 top (title + blank + tabs + indicator + blank) + ui.BottomChromeHeight bottom + extra for profile bar
	h := m.height - 5 - ui.BottomChromeHeight - extra
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
	case tabCI:
		cvl = ui.CICursorLine(m.groupedRuns(), m.cursor)
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
	case tabCI:
		groups := m.groupedRuns()
		if len(groups) > 0 {
			g := groups[len(groups)-1]
			if len(g.Runs) > 0 {
				totalVL = ui.CICursorLine(groups, g.StartIdx+len(g.Runs)-1) + 1
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
// Chrome layout without profile bar (rows 0-4):
//
//	0  title "grove"
//	1  blank
//	2  tabs + inline filter/sort indicator
//	3  blank
//	4  column header
//	5+ scrollable content
//
// With profile bar, rows shift down by 1 (profile bar is row 2, content tabs row 3).
func (m appModel) termRowToCursor(termRow int) int {
	headerRows := 5
	if m.showProfileBar() {
		headerRows = 6
	}
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
	case tabCI:
		return ui.CIIndexAtVL(m.groupedRuns(), vl)
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
	if m.activeProfile == -1 && len(m.cfg.Profiles) > 1 {
		// "All" mode: group by profile name
		lookup := make(map[string]string, len(m.repos))
		for _, r := range m.repos {
			lookup[r.Name] = r.Profile
		}
		po := m.profileOrder()
		return ui.BuildGroups(repos,
			func(name string) string {
				if p, ok := lookup[name]; ok {
					return p
				}
				return "other"
			}, po)
	}
	p := m.activeProfileObj()
	return ui.BuildGroups(repos, p.GroupFor, p.GroupOrder)
}

func (m appModel) groupedPRs() []ui.PRGroup {
	prs := m.filteredPRs()
	if !m.grouped {
		return []ui.PRGroup{{Name: "", PRs: prs, StartIdx: 0}}
	}
	if m.activeProfile == -1 && len(m.cfg.Profiles) > 1 {
		// "All" mode: group by profile name
		lookup := make(map[string]string, len(m.prs))
		for _, pr := range m.prs {
			lookup[repoBaseName(pr.Repo)] = pr.Profile
		}
		po := m.profileOrder()
		return ui.BuildPRGroups(prs,
			func(name string) string {
				if p, ok := lookup[repoBaseName(name)]; ok {
					return p
				}
				return "other"
			}, po)
	}
	p := m.activeProfileObj()
	return ui.BuildPRGroups(prs, p.GroupFor, p.GroupOrder)
}

func (m appModel) groupedBranches() []ui.BranchGroup {
	branches := m.filteredBranches()
	if !m.grouped {
		return []ui.BranchGroup{{Name: "", Branches: branches, StartIdx: 0}}
	}
	if m.activeProfile == -1 && len(m.cfg.Profiles) > 1 {
		// "All" mode: group by profile name
		lookup := make(map[string]string, len(m.branches))
		for _, br := range m.branches {
			lookup[repoBaseName(br.Repo)] = br.Profile
		}
		po := m.profileOrder()
		return ui.BuildBranchGroups(branches,
			func(name string) string {
				if p, ok := lookup[repoBaseName(name)]; ok {
					return p
				}
				return "other"
			}, po)
	}
	p := m.activeProfileObj()
	return ui.BuildBranchGroups(branches, p.GroupFor, p.GroupOrder)
}

func (m appModel) groupedRuns() []ui.CIGroup {
	runs := m.filteredRuns()
	if !m.grouped {
		return []ui.CIGroup{{Name: "", Runs: runs, StartIdx: 0}}
	}
	if m.activeProfile == -1 && len(m.cfg.Profiles) > 1 {
		lookup := make(map[string]string, len(m.runs))
		for _, r := range m.runs {
			lookup[repoBaseName(r.Repo)] = r.Profile
		}
		po := m.profileOrder()
		return ui.BuildCIGroups(runs,
			func(name string) string {
				if p, ok := lookup[name]; ok {
					return p
				}
				return "other"
			}, po)
	}
	p := m.activeProfileObj()
	return ui.BuildCIGroups(runs, p.GroupFor, p.GroupOrder)
}

func (m appModel) groupedActivity() []ui.CommitGroup {
	commits := m.filteredActivity()
	if !m.grouped {
		return []ui.CommitGroup{{Name: "", Commits: commits, StartIdx: 0}}
	}
	if m.activeProfile == -1 && len(m.cfg.Profiles) > 1 {
		// "All" mode: group by profile name
		lookup := make(map[string]string, len(m.activity))
		for _, c := range m.activity {
			lookup[c.Repo] = c.Profile
		}
		po := m.profileOrder()
		return ui.BuildCommitGroups(commits,
			func(name string) string {
				if p, ok := lookup[name]; ok {
					return p
				}
				return "other"
			}, po)
	}
	p := m.activeProfileObj()
	return ui.BuildCommitGroups(commits, p.GroupFor, p.GroupOrder)
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
		ciConclusion := map[string]string{}
		for _, r := range m.runs {
			name := repoBaseName(r.Repo)
			if _, seen := ciConclusion[name]; !seen {
				if r.Status != "completed" {
					ciConclusion[name] = "running"
				} else {
					ciConclusion[name] = r.Conclusion
				}
			}
		}
		for _, g := range m.groupedRepos() {
			for _, r := range g.Repos {
				if flat == m.cursor {
					s := ciConclusion[r.Name]
					if s == "" {
						s = "—"
					}
					return s
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
	case tabCI:
		for _, g := range m.groupedRuns() {
			for _, r := range g.Runs {
				if flat == m.cursor {
					if m.highlightField == "repo" {
						return repoBaseName(r.Repo)
					}
					return cyclePrefix(r.WorkflowName)
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
	case tabCI:
		last := ""
		for _, g := range m.groupedRuns() {
			for j, r := range g.Runs {
				if name := repoBaseName(r.Repo); name != last {
					starts = append(starts, g.StartIdx+j)
					last = name
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
		// Build a CI-conclusion lookup (runs are sorted newest-first).
		ciConclusion := map[string]string{}
		for _, r := range m.runs {
			name := repoBaseName(r.Repo)
			if _, seen := ciConclusion[name]; !seen {
				if r.Status != "completed" {
					ciConclusion[name] = "running"
				} else {
					ciConclusion[name] = r.Conclusion
				}
			}
		}
		last := ""
		for _, g := range m.groupedRepos() {
			for j, r := range g.Repos {
				status := ciConclusion[r.Name]
				if status == "" {
					status = "—"
				}
				if status != last {
					starts = append(starts, g.StartIdx+j)
					last = status
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
	case tabCI:
		last := ""
		for _, g := range m.groupedRuns() {
			for j, r := range g.Runs {
				if p := cyclePrefix(r.WorkflowName); p != last {
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

// detailSectionStarts returns the line indices where each section of the detail pane
// starts, plus a sentinel total-line-count as the last element.
// Indices: [0]header, [1]local branches, [2]remote branches,
//          [3]open PRs, [4]CI runs, [5]recent commits, [6]sentinel.
func (m appModel) detailSectionStarts() []int {
	// itemLines is the number of data rows a section with n items renders.
	itemLines := func(n, cap int) int {
		if n == 0 {
			return 1 // "(none)" line
		}
		if n <= cap {
			return n
		}
		return cap + 1 // cap rows + "… N more"
	}
	// sh is total line height of one section: title + items + optional blank separator.
	sh := func(n, cap int, blank bool) int {
		h := 1 + itemLines(n, cap)
		if blank {
			h++ // trailing blank line between sections
		}
		return h
	}

	// Header block: name + divider + blank + (branch, status, sync, path) + optional stats + blank
	headerLines := 8
	if m.detailStats.CommitCount > 0 || m.detailStats.Contributors > 0 {
		headerLines++
	}

	// Count remote branches and CI runs for the repo currently shown in detail.
	var remoteCount, ciCount int
	repos := m.filteredRepos()
	if m.cursor < len(repos) {
		repoName := repos[m.cursor].Name
		for _, br := range m.branches {
			if repoBaseName(br.Repo) == repoName {
				remoteCount++
			}
		}
		for _, r := range m.runs {
			if repoBaseName(r.Repo) == repoName {
				ciCount++
			}
		}
	}

	pos := headerLines
	starts := []int{0, pos} // header=0, local_branches=headerLines

	pos += sh(len(m.detailBranches), 12, true)
	starts = append(starts, pos) // remote_branches

	pos += sh(remoteCount, 12, true)
	starts = append(starts, pos) // prs

	pos += sh(len(m.detailPRs), 10, true)
	starts = append(starts, pos) // ci_runs

	pos += sh(ciCount, 8, true)
	starts = append(starts, pos) // commits

	pos += sh(len(m.detailCommits), 10, false) // last section: no trailing blank
	starts = append(starts, pos)               // sentinel = total line count

	return starts
}

// clampDetailScroll clamps m.detailScroll to [0, totalLines-contentHeight].
func (m *appModel) clampDetailScroll() {
	sects := m.detailSectionStarts()
	total := sects[len(sects)-1]
	maxScroll := max(0, total-m.contentHeight())
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	if m.detailScroll < 0 {
		m.detailScroll = 0
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
	case tabCI:
		starts = ui.CIGroupStarts(m.groupedRuns())
	}
	m.jumpTo(starts, dir)
}
