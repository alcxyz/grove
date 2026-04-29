package app

import (
	"fmt"
	"strings"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/ui"
)

// infoBarParts returns the context-aware pieces for the bottom info bar.
func (m Model) infoBarParts() []string {
	if m.showHelp {
		return []string{"? close help"}
	}
	if m.showDiff {
		return []string{
			fmt.Sprintf("%s  %s", m.diffRepo, m.diffHash),
			"j/k scroll",
			"[/] next/prev commit",
			"{/} files",
			"o open on GitHub",
			"esc back",
		}
	}
	if m.showDetail {
		return []string{"j/k select", "[/] next/prev repo", "{/} sections", "space/o/e act", "esc back"}
	}
	if m.filtering {
		return []string{"type to filter", "enter confirm", "esc clear"}
	}

	var parts []string
	switch m.activeTab {
	case tabDashboard:
		repos := m.filteredRepos()
		dirty, behind, ahead := 0, 0, 0
		for _, r := range repos {
			if r.Dirty {
				dirty++
			}
			if r.Behind > 0 {
				behind++
			}
			if r.Ahead > 0 {
				ahead++
			}
		}
		parts = append(parts, fmt.Sprintf("%d repos", len(repos)))
		if dirty > 0 {
			parts = append(parts, ui.DirtyStyle.Render(fmt.Sprintf("%d dirty", dirty)))
		}
		if behind > 0 {
			parts = append(parts, ui.BehindStyle.Render(fmt.Sprintf("%d behind", behind)))
		}
		if ahead > 0 {
			parts = append(parts, ui.AheadStyle.Render(fmt.Sprintf("%d ahead", ahead)))
		}
	case tabPRs:
		prs := m.filteredPRs()
		approved, changes, review := 0, 0, 0
		for _, pr := range prs {
			switch pr.ReviewDecision {
			case "APPROVED":
				approved++
			case "CHANGES_REQUESTED":
				changes++
			case "REVIEW_REQUIRED":
				review++
			}
		}
		parts = append(parts, fmt.Sprintf("%d open PRs", len(prs)))
		if approved > 0 {
			parts = append(parts, ui.ApprovedStyle.Render(fmt.Sprintf("%d ✓ approved", approved)))
		}
		if changes > 0 {
			parts = append(parts, ui.ChangesStyle.Render(fmt.Sprintf("%d ✗ changes", changes)))
		}
		if review > 0 {
			parts = append(parts, ui.ReviewStyle.Render(fmt.Sprintf("%d ● review", review)))
		}
	case tabBranches:
		branches := m.filteredBranches()
		repos := map[string]struct{}{}
		for _, br := range branches {
			repos[br.Repo] = struct{}{}
		}
		parts = append(parts,
			fmt.Sprintf("%d branches", len(branches)),
			fmt.Sprintf("%d repos", len(repos)),
		)
	case tabActivity:
		commits := m.filteredActivity()
		repos := map[string]struct{}{}
		for _, c := range commits {
			repos[c.Repo] = struct{}{}
		}
		parts = append(parts,
			fmt.Sprintf("%d commits", len(commits)),
			fmt.Sprintf("%d repos", len(repos)),
		)
	case tabCI:
		runs := m.filteredRuns()
		repos := map[string]struct{}{}
		failed := 0
		for _, r := range runs {
			repos[repoBaseName(r.Repo)] = struct{}{}
			if r.Conclusion == "failure" || r.Conclusion == "timed_out" || r.Conclusion == "startup_failure" {
				failed++
			}
		}
		parts = append(parts,
			fmt.Sprintf("%d runs", len(runs)),
			fmt.Sprintf("%d repos", len(repos)),
		)
		if failed > 0 {
			parts = append(parts, ui.FailStyle.Render(fmt.Sprintf("%d failed", failed)))
		}
	}
	if !m.grouped {
		parts = append(parts, ui.CycleStyle.Render("flat"))
	}
	if !m.autoRefresh {
		parts = append(parts, ui.DimStyle.Render("auto-refresh off"))
	}
	parts = append(parts, ui.DimStyle.Render("? help"))
	parts = append(parts, ui.DimStyle.Render("v"+m.version))
	if m.latestVersion != "" {
		parts = append(parts, ui.PendingStyle.Render("↑ "+m.latestVersion+" available"))
	}
	return parts
}

func (m Model) View() string {
	// Screensaver takes over the full frame
	if m.ssActive {
		return ui.RenderScreensaver(m.ssX, m.ssY, m.ssColor, m.width, m.height)
	}

	var b strings.Builder

	// Title
	b.WriteString(ui.TitleStyle.Render("grove"))
	b.WriteString("\n\n")

	// Profile bar — only when multiple profiles are configured
	if m.showProfileBar() {
		b.WriteString(ui.RenderProfileTabs(m.profileTabNames(), m.activeProfileTabIdx()))
		b.WriteString("\n")
	}

	// Tabs
	b.WriteString(ui.RenderTabs(tabNames, int(m.activeTab)))

	// Indicator line: text filter + cycle filter + sort
	if m.filtering {
		b.WriteString(fmt.Sprintf("  %s%s▏",
			ui.DimStyle.Render("filter: "), ui.HeaderStyle.Render(m.filterQuery)))
	} else if m.filterQuery != "" {
		b.WriteString(ui.RenderFilter(m.filterQuery))
	}
	if m.cycleField != "" && m.cycleIdx >= 0 && m.cycleIdx < len(m.cycleValues) {
		b.WriteString(ui.RenderCycleFilter(m.cycleField, m.cycleValues[m.cycleIdx],
			m.cycleIdx+1, len(m.cycleValues)))
	}
	if ts := m.tabSort[m.activeTab]; ts.Field != "" {
		// Label the sort field per-tab
		label := ts.Field
		if ts.Field == "date" {
			label = map[tab]string{
				tabDashboard: "date",
				tabPRs:       "updated",
				tabBranches:  "date",
				tabActivity:  "date",
				tabCI:        "updated",
			}[m.activeTab]
		} else if ts.Field == "subject" {
			label = map[tab]string{
				tabDashboard: "name",
				tabPRs:       "title",
				tabBranches:  "branch",
				tabActivity:  "subject",
				tabCI:        "workflow",
			}[m.activeTab]
		} else if ts.Field == "repo" {
			label = "repo"
		} else if ts.Field == "prcount" {
			if m.activeTab == tabBranches {
				label = "has PR"
			} else {
				label = "PR count"
			}
		} else if ts.Field == "brcount" {
			label = "branch count"
		} else if ts.Field == "ci" {
			label = "CI status"
		} else if ts.Field == "review" {
			label = "review"
		} else if ts.Field == "checks" {
			label = "checks"
		} else if ts.Field == "merged" {
			label = "merged"
		} else if ts.Field == "branch" {
			label = "branch"
		}
		b.WriteString(ui.RenderSortIndicator(label, ts.Order == sortAsc))
	}
	b.WriteString("\n\n")

	width := m.width
	if width == 0 {
		width = 120
	}
	cw := width

	// Splash overlay replaces main content
	if m.showSplash {
		b.WriteString(ui.RenderSplash(config.ConfigPath(), m.cacheDir, m.logPath, m.version, width, m.splashBlink))
	} else if m.showHelp {
		b.WriteString(ui.RenderHelp(width, m.helpPage, m.version))
	} else if m.showDiff {
		// Diff pane — render full content, slice to viewport using diffScroll.
		content := ui.RenderDiff(m.diffRepo, m.diffHash, m.diffContent, m.diffPreColored)
		lines := strings.Split(content, "\n")
		start := m.diffScroll
		if start > len(lines) {
			start = len(lines)
		}
		end := start + m.contentHeight()
		if end > len(lines) {
			end = len(lines)
		}
		b.WriteString(strings.Join(lines[start:end], "\n"))
	} else if m.showDetail {
		// Detail pane — use extracted helpers for remote branches and CI runs.
		if repo := m.detailRepo; repo.Path != "" {
			remoteBranches := m.detailRemoteBranches()
			ciRuns := m.detailCIRuns()
			hlLine := -1
			if m.detailCursor >= 0 && m.detailCursor < len(m.detailItems) {
				hlLine = m.detailItems[m.detailCursor].Line
			}
			content := ui.RenderRepoDetail(repo, m.detailCommits, m.detailPRs, m.detailBranches, remoteBranches, ciRuns, m.detailStats, cw, hlLine)
			lines := strings.Split(content, "\n")
			start := m.detailScroll
			if start > len(lines) {
				start = len(lines)
			}
			end := start + m.contentHeight()
			if end > len(lines) {
				end = len(lines)
			}
			b.WriteString(strings.Join(lines[start:end], "\n"))
		}
	} else {
		so := m.scrollOffset[m.activeTab]
		sh := m.scrollHeight()
		hlField, hlValue := m.highlightField, m.blockHighlightValue()
		switch m.activeTab {
		case tabDashboard:
			prCounts := map[string]int{}
			for _, pr := range m.prs {
				prCounts[repoBaseName(pr.Repo)]++
			}
			branchCounts := map[string]int{}
			for _, br := range m.branches {
				branchCounts[repoBaseName(br.Repo)]++
			}
			// Latest CI run per repo (runs are sorted newest-first)
			ciStatus := map[string]string{}
			for _, r := range m.runs {
				name := repoBaseName(r.Repo)
				if _, seen := ciStatus[name]; !seen {
					ciStatus[name] = ui.CIStatusIcon(r.Status, r.Conclusion)
				}
			}
			b.WriteString(ui.RenderDashboard(m.groupedRepos(), m.cursor, cw, so, sh, prCounts, branchCounts, ciStatus, hlField, hlValue))
		case tabPRs:
			if m.authErr {
				b.WriteString(ui.RenderAuthError())
			} else {
				b.WriteString(ui.RenderPRs(m.groupedPRs(), m.cursor, cw, so, sh, hlField, hlValue))
				if len(m.prs) == 0 && len(m.errLog) > 0 {
					b.WriteString(ui.RenderErrors(m.errLog))
				}
			}
		case tabBranches:
			if m.authErr {
				b.WriteString(ui.RenderAuthError())
			} else {
				prBranches := map[string]bool{}
				for _, pr := range m.prs {
					prBranches[pr.Branch] = true
				}
				b.WriteString(ui.RenderBranches(m.groupedBranches(), m.cursor, cw, so, sh, prBranches, hlField, hlValue))
				if len(m.branches) == 0 && len(m.errLog) > 0 {
					b.WriteString(ui.RenderErrors(m.errLog))
				}
			}
		case tabActivity:
			b.WriteString(ui.RenderActivity(m.groupedActivity(), m.cursor, cw, so, sh, hlField, hlValue))
		case tabCI:
			if m.authErr {
				b.WriteString(ui.RenderAuthError())
			} else {
				b.WriteString(ui.RenderCI(m.groupedRuns(), m.cursor, cw, so, sh, hlField, hlValue))
				if len(m.runs) == 0 && len(m.errLog) > 0 {
					b.WriteString(ui.RenderErrors(m.errLog))
				}
			}
		}
	}

	// Bottom chrome — owl mascot + status/info.  No leading or trailing newline:
	// the content area above ends with its own "\n" per item, and a trailing "\n"
	// here would create an extra line that shifts the title off-screen.
	b.WriteString(ui.RenderBottomArea(m.statusMsg, m.loading, m.infoBarParts(), m.splashBlink, width))

	return b.String()
}
