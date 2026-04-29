package app

import (
	"strings"
	"time"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/model"
	"github.com/alcxyz/grove/internal/ui"
)

func (m Model) listLen() int {
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
	case tabIssues:
		return len(m.filteredIssues())
	}
	return 0
}

func (m *Model) clampCursor() {
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
func (m Model) showProfileBar() bool {
	return len(m.cfg.Profiles) > 1
}

// activeProfileObj returns the currently active profile, or the first profile
// as a fallback. Used for single-profile grouping.
func (m Model) activeProfileObj() config.Profile {
	if m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles) {
		return m.cfg.Profiles[m.activeProfile]
	}
	if len(m.cfg.Profiles) > 0 {
		return m.cfg.Profiles[0]
	}
	return config.Profile{}
}

// profileOrder returns a func(string) int for ordering groups by profile index.
func (m Model) profileOrder() func(string) int {
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
func (m Model) profileAtX(x int) int {
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
func (m Model) profileTabNames() []string {
	names := make([]string, len(m.cfg.Profiles)+1)
	for i, p := range m.cfg.Profiles {
		names[i] = p.Name
	}
	names[len(m.cfg.Profiles)] = "All"
	return names
}

// activeProfileTabIdx returns the render index for the active profile
// (0..N-1 for profiles, N for "All").
func (m Model) activeProfileTabIdx() int {
	if m.activeProfile == -1 {
		return len(m.cfg.Profiles)
	}
	return m.activeProfile
}

// contentHeight returns the number of scrollable lines available in the
// terminal after accounting for fixed chrome (title, tabs, owl bottom area).
func (m Model) contentHeight() int {
	extra := 0
	if m.showProfileBar() {
		extra = 1
	}
	// 6 top (title + blank + 2 tab rows + indicator + blank) + ui.BottomChromeHeight bottom + extra for profile bar
	h := m.height - 6 - ui.BottomChromeHeight - extra
	if h < 1 {
		return 1
	}
	return h
}

// tabJumpDistances maps streak level (0-3) to jump distance.
var tabJumpDistances = [4]int{5, 10, 20, 25}

// tabJump moves the cursor by an exponential amount based on how rapidly tab
// is pressed.  dir is +1 (tab) or -1 (shift+tab).
func (m *Model) tabJump(dir int) {
	now := time.Now()
	if now.Sub(m.lastTabAt) < 300*time.Millisecond && m.tabStreak < 3 {
		m.tabStreak++
	} else {
		m.tabStreak = 0
	}
	m.lastTabAt = now

	dist := tabJumpDistances[m.tabStreak] * dir

	if m.showDiff {
		m.diffScroll += dist
		m.clampDiffScroll()
		if m.diffScroll < 0 {
			m.diffScroll = 0
		}
		return
	}
	if m.showDetail {
		target := m.detailCursor + dist
		if target < 0 {
			target = 0
		}
		if target >= len(m.detailItems) {
			target = len(m.detailItems) - 1
		}
		if target >= 0 {
			m.detailCursor = target
			m.adjustDetailScroll()
		}
		return
	}
	// Main list
	m.cursor += dist
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampCursor()
}

// scrollHeight returns the number of lines available for scrollable list items
// after subtracting the fixed column header that each Render* function emits.
// Detail and diff views don't have a column header — use contentHeight() there.
func (m Model) scrollHeight() int {
	h := m.contentHeight() - 1 // -1 for the column header line
	if h < 1 {
		return 1
	}
	return h
}

// adjustScroll keeps the cursor's visual line inside the visible viewport.
func (m *Model) adjustScroll() {
	if m.height == 0 {
		return
	}
	sh := m.scrollHeight()

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
	case tabIssues:
		cvl = ui.IssueCursorLine(m.groupedIssues(), m.cursor)
	}

	so := m.scrollOffset[m.activeTab]
	if cvl < so {
		so = cvl
	}
	if cvl >= so+sh {
		so = cvl - sh + 1
	}
	if so < 0 {
		so = 0
	}
	m.scrollOffset[m.activeTab] = so
}

// maxScrollOffset returns the maximum scroll offset for the current tab so the
// last item is visible at the bottom of the viewport.
func (m Model) maxScrollOffset() int {
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
	case tabIssues:
		groups := m.groupedIssues()
		if len(groups) > 0 {
			g := groups[len(groups)-1]
			if len(g.Issues) > 0 {
				totalVL = ui.IssueCursorLine(groups, g.StartIdx+len(g.Issues)-1) + 1
			}
		}
	}
	mso := totalVL - m.scrollHeight()
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
func (m Model) termRowToCursor(termRow int) int {
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
	case tabIssues:
		return ui.IssueIndexAtVL(m.groupedIssues(), vl)
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

// repoPathLookup builds a name→path map from the model's repo list.
func (m Model) repoPathLookup() map[string]string {
	lk := make(map[string]string, len(m.repos))
	for _, r := range m.repos {
		lk[r.Name] = r.Path
	}
	return lk
}

// groupForFunc returns a func(string) string closure that wraps p.GroupFor
// with path lookup from the model's repo list.
func (m Model) groupForFunc(p config.Profile) func(string) string {
	paths := m.repoPathLookup()
	return func(name string) string {
		return p.GroupFor(name, paths[name])
	}
}

func (m Model) groupedRepos() []ui.RepoGroup {
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
	return ui.BuildGroups(repos, m.groupForFunc(p), p.GroupOrder)
}

func (m Model) groupedPRs() []ui.PRGroup {
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
	return ui.BuildPRGroups(prs, m.groupForFunc(p), p.GroupOrder)
}

func (m Model) groupedBranches() []ui.BranchGroup {
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
	return ui.BuildBranchGroups(branches, m.groupForFunc(p), p.GroupOrder)
}

func (m Model) groupedRuns() []ui.CIGroup {
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
	return ui.BuildCIGroups(runs, m.groupForFunc(p), p.GroupOrder)
}

func (m Model) groupedIssues() []ui.IssueGroup {
	issues := m.filteredIssues()
	if !m.grouped {
		return []ui.IssueGroup{{Name: "", Issues: issues, StartIdx: 0}}
	}
	if m.activeProfile == -1 && len(m.cfg.Profiles) > 1 {
		lookup := make(map[string]string, len(m.issues))
		for _, iss := range m.issues {
			lookup[repoBaseName(iss.Repo)] = iss.Profile
		}
		po := m.profileOrder()
		return ui.BuildIssueGroups(issues,
			func(name string) string {
				if p, ok := lookup[repoBaseName(name)]; ok {
					return p
				}
				return "other"
			}, po)
	}
	p := m.activeProfileObj()
	return ui.BuildIssueGroups(issues, m.groupForFunc(p), p.GroupOrder)
}

func (m Model) groupedActivity() []ui.CommitGroup {
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
	return ui.BuildCommitGroups(commits, m.groupForFunc(p), p.GroupOrder)
}

// blockHighlightValue returns the match string for the current highlight field
// by inspecting the item at the cursor in grouped order.
func (m Model) blockHighlightValue() string {
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
	case tabIssues:
		for _, g := range m.groupedIssues() {
			for _, iss := range g.Issues {
				if flat == m.cursor {
					if m.highlightField == "repo" {
						return repoBaseName(iss.Repo)
					}
					return cyclePrefix(iss.Title)
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
func (m *Model) jumpRepo(dir int) {
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
	case tabIssues:
		last := ""
		for _, g := range m.groupedIssues() {
			for j, iss := range g.Issues {
				if name := repoBaseName(iss.Repo); name != last {
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
func (m *Model) jumpSubject(dir int) {
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
	case tabIssues:
		last := ""
		for _, g := range m.groupedIssues() {
			for j, iss := range g.Issues {
				if p := cyclePrefix(iss.Title); p != last {
					starts = append(starts, g.StartIdx+j)
					last = p
				}
			}
		}
	}
	m.jumpTo(starts, dir)
}

// jumpTo is the shared navigation kernel for jumpRepo/jumpSubject/jumpGroup.
func (m *Model) jumpTo(starts []int, dir int) {
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
	// Try to show context (group header / blank separator) above the cursor
	// by pulling the viewport up by 2 lines.  Re-call adjustScroll afterward
	// so the cursor is never pushed below the visible area.
	t := m.activeTab
	if so := m.scrollOffset[t]; so >= 2 {
		m.scrollOffset[t] = so - 2
	} else {
		m.scrollOffset[t] = 0
	}
	m.adjustScroll()
}

// repoForDetailNav resolves a cursor index (in the current tab's grouped order)
// to the associated Repo for loading into the detail pane.  Used by [/] navigation
// so the detail pane cycles through the source tab's items, not always Dashboard repos.
func (m Model) repoForDetailNav(idx int) (model.Repo, bool) {
	flat := 0
	switch m.activeTab {
	case tabDashboard:
		for _, g := range m.groupedRepos() {
			for _, r := range g.Repos {
				if flat == idx {
					return r, true
				}
				flat++
			}
		}
	case tabPRs:
		for _, g := range m.groupedPRs() {
			for _, pr := range g.PRs {
				if flat == idx {
					return m.repoByName(repoBaseName(pr.Repo))
				}
				flat++
			}
		}
	case tabBranches:
		for _, g := range m.groupedBranches() {
			for _, br := range g.Branches {
				if flat == idx {
					return m.repoByName(repoBaseName(br.Repo))
				}
				flat++
			}
		}
	case tabActivity:
		for _, g := range m.groupedActivity() {
			for _, c := range g.Commits {
				if flat == idx {
					return m.repoByName(c.Repo)
				}
				flat++
			}
		}
	case tabCI:
		for _, g := range m.groupedRuns() {
			for _, r := range g.Runs {
				if flat == idx {
					return m.repoByName(repoBaseName(r.Repo))
				}
				flat++
			}
		}
	case tabIssues:
		for _, g := range m.groupedIssues() {
			for _, iss := range g.Issues {
				if flat == idx {
					return m.repoByName(repoBaseName(iss.Repo))
				}
				flat++
			}
		}
	}
	return model.Repo{}, false
}

// repoPathFor returns the local filesystem path for a repo matched by base name.
func (m Model) repoPathFor(baseName string) string {
	for _, r := range m.repos {
		if r.Name == baseName {
			return r.Path
		}
	}
	return ""
}

// repoByName returns the full Repo struct matched by base name.
func (m Model) repoByName(baseName string) (model.Repo, bool) {
	for _, r := range m.repos {
		if r.Name == baseName {
			return r, true
		}
	}
	return model.Repo{}, false
}

// repoPathAtCursor returns the local path for the currently selected item by
// walking the grouped structure. This is necessary because BuildGroups sorts
// groups and reassigns StartIdx in group order, so m.cursor is an index into
// the grouped flat sequence — not the original filteredRepos() order.
func (m Model) repoPathAtCursor() string {
	flat := 0
	switch m.activeTab {
	case tabDashboard:
		for _, g := range m.groupedRepos() {
			for _, r := range g.Repos {
				if flat == m.cursor {
					return r.Path
				}
				flat++
			}
		}
	case tabPRs:
		for _, g := range m.groupedPRs() {
			for _, pr := range g.PRs {
				if flat == m.cursor {
					return m.repoPathFor(repoBaseName(pr.Repo))
				}
				flat++
			}
		}
	case tabBranches:
		for _, g := range m.groupedBranches() {
			for _, br := range g.Branches {
				if flat == m.cursor {
					return m.repoPathFor(repoBaseName(br.Repo))
				}
				flat++
			}
		}
	case tabActivity:
		for _, g := range m.groupedActivity() {
			for _, c := range g.Commits {
				if flat == m.cursor {
					return c.RepoPath
				}
				flat++
			}
		}
	case tabCI:
		for _, g := range m.groupedRuns() {
			for _, r := range g.Runs {
				if flat == m.cursor {
					return m.repoPathFor(repoBaseName(r.Repo))
				}
				flat++
			}
		}
	case tabIssues:
		for _, g := range m.groupedIssues() {
			for _, iss := range g.Issues {
				if flat == m.cursor {
					return m.repoPathFor(repoBaseName(iss.Repo))
				}
				flat++
			}
		}
	}
	return ""
}

// repoAtCursor returns the repo at the cursor by walking grouped order.
// Returns (repo, true) or (zero, false) if cursor is out of range.
func (m Model) repoAtCursor() (model.Repo, bool) {
	flat := 0
	for _, g := range m.groupedRepos() {
		for _, r := range g.Repos {
			if flat == m.cursor {
				return r, true
			}
			flat++
		}
	}
	return model.Repo{}, false
}

// prAtCursor returns the PR at the cursor by walking grouped order.
func (m Model) prAtCursor() (model.PR, bool) {
	flat := 0
	for _, g := range m.groupedPRs() {
		for _, pr := range g.PRs {
			if flat == m.cursor {
				return pr, true
			}
			flat++
		}
	}
	return model.PR{}, false
}

// branchAtCursor returns the branch at the cursor by walking grouped order.
func (m Model) branchAtCursor() (model.BranchInfo, bool) {
	flat := 0
	for _, g := range m.groupedBranches() {
		for _, br := range g.Branches {
			if flat == m.cursor {
				return br, true
			}
			flat++
		}
	}
	return model.BranchInfo{}, false
}

// commitAtCursor returns the Activity commit at the cursor by walking grouped order.
func (m Model) commitAtCursor() (model.Commit, bool) {
	flat := 0
	for _, g := range m.groupedActivity() {
		for _, c := range g.Commits {
			if flat == m.cursor {
				return c, true
			}
			flat++
		}
	}
	return model.Commit{}, false
}

// runAtCursor returns the CI run at the cursor by walking grouped order.
func (m Model) runAtCursor() (model.WorkflowRun, bool) {
	flat := 0
	for _, g := range m.groupedRuns() {
		for _, r := range g.Runs {
			if flat == m.cursor {
				return r, true
			}
			flat++
		}
	}
	return model.WorkflowRun{}, false
}

func (m Model) issueAtCursor() (model.Issue, bool) {
	flat := 0
	for _, g := range m.groupedIssues() {
		for _, iss := range g.Issues {
			if flat == m.cursor {
				return iss, true
			}
			flat++
		}
	}
	return model.Issue{}, false
}

// detailSectionStarts returns the line indices where each section of the detail pane
// starts, plus a sentinel total-line-count as the last element.
// Indices: [0]header, [1]local branches, [2]remote branches,
//          [3]open PRs, [4]CI runs, [5]recent commits, [6]sentinel.
func (m Model) detailSectionStarts() []int {
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
	repoName := m.detailRepo.Name
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

	pos := headerLines
	starts := []int{0, pos} // header=0, local_branches=headerLines

	pos += sh(len(m.detailBranches), 12, true)
	starts = append(starts, pos) // remote_branches

	pos += sh(remoteCount, 12, true)
	starts = append(starts, pos) // prs

	pos += sh(len(m.detailPRs), 10, true)
	starts = append(starts, pos) // issues

	pos += sh(len(m.detailIssues), 10, true)
	starts = append(starts, pos) // ci_runs

	pos += sh(ciCount, 8, true)
	starts = append(starts, pos) // commits

	pos += sh(len(m.detailCommits), 10, false) // last section: no trailing blank
	starts = append(starts, pos)               // sentinel = total line count

	return starts
}

// detailRemoteBranches returns the remote branches for the repo currently shown
// in the detail pane, filtered from the global branch list.
func (m Model) detailRemoteBranches() []model.BranchInfo {
	name := m.detailRepo.Name
	if name == "" {
		return nil
	}
	var out []model.BranchInfo
	for _, br := range m.branches {
		if repoBaseName(br.Repo) == name {
			out = append(out, br)
		}
	}
	return out
}

// detailCIRuns returns the CI runs for the repo currently shown in the detail
// pane, filtered from the global runs list.
func (m Model) detailCIRuns() []model.WorkflowRun {
	name := m.detailRepo.Name
	if name == "" {
		return nil
	}
	var out []model.WorkflowRun
	for _, r := range m.runs {
		if repoBaseName(r.Repo) == name {
			out = append(out, r)
		}
	}
	return out
}

// buildDetailItems constructs the flat list of selectable items in the detail
// pane, recording each item's section type, data index, and rendered line.
// The line counting must exactly match RenderRepoDetail's output.
func (m *Model) buildDetailItems() {
	m.detailItems = m.detailItems[:0]

	// Header block: name, divider, blank, branch, status, sync, path = 7 lines
	line := 7
	if m.detailStats.CommitCount > 0 || m.detailStats.Contributors > 0 {
		line++ // stats line
	}
	line++ // trailing blank after header

	// Helper: process one section. Returns next line after the section.
	// For each rendered item (up to cap), appends a detailItem.
	type sectionDef struct {
		sect  detailSect
		count int
		cap   int
	}
	sections := []sectionDef{
		{detailLocalBranch, len(m.detailBranches), 12},
		{detailRemoteBranch, len(m.detailRemoteBranches()), 12},
		{detailPR, len(m.detailPRs), 10},
		{detailIssue, len(m.detailIssues), 10},
		{detailCIRun, len(m.detailCIRuns()), 8},
		{detailCommit, len(m.detailCommits), 10},
	}

	for si, s := range sections {
		line++ // section header line
		if s.count == 0 {
			line++ // "(none)" line
		} else {
			rendered := s.count
			if rendered > s.cap {
				rendered = s.cap
			}
			for i := 0; i < rendered; i++ {
				m.detailItems = append(m.detailItems, detailItem{
					Section: s.sect,
					Index:   i,
					Line:    line,
				})
				line++
			}
			if s.count > s.cap {
				line++ // "… N more" line
			}
		}
		// Trailing blank after every section except the last
		if si < len(sections)-1 {
			line++
		}
	}
}

// diffTotalLines returns the total number of rendered lines in the diff pane,
// matching what RenderDiff emits (2 header lines + content lines + trailing blank).
func (m Model) diffTotalLines() int {
	content := ui.RenderDiff(m.diffRepo, m.diffHash, m.diffContent, m.diffPreColored)
	return len(strings.Split(content, "\n"))
}

// diffFileStarts returns the rendered-line indices where each file change begins
// ("diff --git ..." lines), offset by the 2-line RenderDiff header.
func (m Model) diffFileStarts() []int {
	var starts []int
	for i, line := range strings.Split(m.diffContent, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			starts = append(starts, i+2) // +2 for title + divider added by RenderDiff
		}
	}
	return starts
}

// detailCursorLine returns the rendered line number of the current detail cursor.
func (m Model) detailCursorLine() int {
	if m.detailCursor >= 0 && m.detailCursor < len(m.detailItems) {
		return m.detailItems[m.detailCursor].Line
	}
	return 0
}

// adjustDetailScroll keeps the detail cursor's line inside the visible viewport.
func (m *Model) adjustDetailScroll() {
	if m.height == 0 || len(m.detailItems) == 0 {
		return
	}
	ch := m.contentHeight()
	line := m.detailCursorLine()
	if line < m.detailScroll {
		m.detailScroll = line
	}
	if line >= m.detailScroll+ch {
		m.detailScroll = line - ch + 1
	}
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
}

// detailJumpSection moves the detail cursor to the first item of the
// next (+1) or previous (-1) section.
func (m *Model) detailJumpSection(dir int) {
	if len(m.detailItems) == 0 {
		return
	}
	cur := m.detailItems[m.detailCursor].Section
	if dir > 0 {
		for i := m.detailCursor + 1; i < len(m.detailItems); i++ {
			if m.detailItems[i].Section != cur {
				m.detailCursor = i
				m.adjustDetailScroll()
				return
			}
		}
		// Already in last section — go to last item.
		m.detailCursor = len(m.detailItems) - 1
		m.adjustDetailScroll()
	} else {
		// Find start of current section, then go to start of previous.
		secStart := m.detailCursor
		for secStart > 0 && m.detailItems[secStart-1].Section == cur {
			secStart--
		}
		if secStart == 0 {
			// Already at first section — stay at first item.
			m.detailCursor = 0
			m.adjustDetailScroll()
			return
		}
		// secStart-1 is last item of previous section; find its start.
		prev := m.detailItems[secStart-1].Section
		target := secStart - 1
		for target > 0 && m.detailItems[target-1].Section == prev {
			target--
		}
		m.detailCursor = target
		m.adjustDetailScroll()
	}
}

// clampDiffScroll clamps m.diffScroll to [0, totalLines-contentHeight].
func (m *Model) clampDiffScroll() {
	maxScroll := max(0, m.diffTotalLines()-m.contentHeight())
	if m.diffScroll > maxScroll {
		m.diffScroll = maxScroll
	}
	if m.diffScroll < 0 {
		m.diffScroll = 0
	}
}

// clampDetailScroll clamps m.detailScroll to [0, totalLines-contentHeight].
func (m *Model) clampDetailScroll() {
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
func (m *Model) jumpGroup(dir int) {
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
	case tabIssues:
		starts = ui.IssueGroupStarts(m.groupedIssues())
	}
	m.jumpTo(starts, dir)
}
