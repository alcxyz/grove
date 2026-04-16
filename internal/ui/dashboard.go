package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alcxyz/grove/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// selRow applies the selected-row background across an entire pre-rendered row.
// Because lipgloss (and every inner style) terminates with \033[0m, a naive
// outer Render() wrapper loses the background after the first inner reset.
// Instead we re-inject the background escape after every reset so it persists
// through all nested colour sequences.
// catSurface1 (#45475a) → RGB(69, 71, 90)
func selRow(s string) string {
	const bg = "\033[48;2;69;71;90m"
	return bg + strings.ReplaceAll(s, "\033[0m", "\033[0m"+bg) + "\033[0m"
}

// cell renders s then pads to visible width w using lipgloss.Width so ANSI
// codes don't break column alignment.
func cell(s string, w int) string {
	pad := w - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// prefixWord extracts the leading word/prefix used for block-jump grouping.
// Matches the logic in main.cyclePrefix.
func prefixWord(s string) string {
	if i := strings.IndexAny(s, " /"); i > 0 {
		return s[:i]
	}
	return s
}

// hlText returns s rendered in BlockMatchStyle when hlField==field and
// prefixWord(s)==hlValue, otherwise returns s unstyled.
// For subject matches it highlights only the leading prefix word; for repo
// matches the whole value is highlighted.
func hlText(s, field, hlField, hlValue string) string {
	if hlField != field || hlValue == "" {
		return s
	}
	if field == "subject" {
		p := prefixWord(s)
		if p != hlValue {
			return s
		}
		if len(p) >= len(s) {
			return BlockMatchStyle.Render(s)
		}
		return BlockMatchStyle.Render(p) + s[len(p):]
	}
	// repo field: exact match on the full value
	if s == hlValue {
		return BlockMatchStyle.Render(s)
	}
	return s
}

// RepoGroup is a named slice of repos for rendering.
type RepoGroup struct {
	Name     string
	Repos    []model.Repo
	StartIdx int
}

// BuildGroups partitions repos into named groups, sorted by groupOrder.
func BuildGroups(repos []model.Repo, groupFor func(string) string, groupOrder func(string) int) []RepoGroup {
	var groups []RepoGroup
	idx := map[string]int{}
	for _, r := range repos {
		name := groupFor(r.Name)
		gi, exists := idx[name]
		if !exists {
			gi = len(groups)
			idx[name] = gi
			groups = append(groups, RepoGroup{Name: name})
		}
		groups[gi].Repos = append(groups[gi].Repos, r)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groupOrder(groups[i].Name) < groupOrder(groups[j].Name)
	})
	si := 0
	for i := range groups {
		groups[i].StartIdx = si
		si += len(groups[i].Repos)
	}
	return groups
}

// groupHeader renders the separator line for a named group.
func groupHeader(name string, width int) string {
	return GroupHeaderStyle.Render(fmt.Sprintf("── %s ", strings.ToUpper(name))) +
		DimStyle.Render(strings.Repeat("─", max(0, width-4-len(name))))
}

// scrollWriter helps render only visible lines within [offset, offset+maxLines).
type scrollWriter struct {
	b       strings.Builder
	vl      int // current visual line (0-indexed, excludes column header)
	offset  int
	maxVL   int // offset + maxLines
}

func newScrollWriter(offset, maxLines int) *scrollWriter {
	return &scrollWriter{offset: offset, maxVL: offset + maxLines}
}

func (sw *scrollWriter) writeLine(s string) {
	if sw.vl >= sw.offset && sw.vl < sw.maxVL {
		sw.b.WriteString(s)
		sw.b.WriteString("\n")
	}
	sw.vl++
}

func (sw *scrollWriter) string() string { return sw.b.String() }

// ── Heat-gradient count indicators ────────────────────────────────────────

// heatCount renders n with a cold→hot color based on the provided thresholds.
// cool/warm/hot are the lower bounds for each heat level.
func heatCount(n, cool, warm, hot int) string {
	if n == 0 {
		return DimStyle.Render("—")
	}
	s := strconv.Itoa(n)
	switch {
	case n < cool:
		return DimStyle.Render(s) // barely any — stay dim
	case n < warm:
		return ReviewStyle.Render(s) // blue/cool
	case n < hot:
		return PendingStyle.Render(s) // yellow/warm
	default:
		return DirtyStyle.Render(s) // red/hot
	}
}

func prHeat(n int) string    { return heatCount(n, 1, 3, 6) }
func branchHeat(n int) string { return heatCount(n, 3, 8, 16) }

// ── Repo row ──────────────────────────────────────────────────────────────

// CIStatusIcon returns a compact status icon for the dashboard CI column.
func CIStatusIcon(status, conclusion string) string {
	if status != "completed" {
		return PendingStyle.Render("●")
	}
	switch conclusion {
	case "success":
		return PassStyle.Render("✓")
	case "failure", "timed_out", "startup_failure":
		return FailStyle.Render("✗")
	case "cancelled":
		return DimStyle.Render("⊘")
	default:
		return DimStyle.Render("—")
	}
}

// col widths: indent(2) name(30) branch(14) status(8) sync(10) pr(4) br(4) ci(4) author(14) ago(rest)
func renderRepoRow(r model.Repo, selected bool, prCount, branchCount int, ciIcon, hlField, hlValue string) string {
	name := truncate(r.Name, 28)
	branch := truncate(r.Branch, 12)
	ago := timeAgo(r.LastCommit)
	author := truncate(r.LastAuthor, 12)

	statusStyled := CleanStyle.Render("clean")
	if r.Dirty {
		statusStyled = DirtyStyle.Render("dirty")
	}
	var syncStyled string
	if r.Ahead > 0 {
		syncStyled += AheadStyle.Render(fmt.Sprintf("↑%d", r.Ahead))
	}
	if r.Behind > 0 {
		if syncStyled != "" {
			syncStyled += " "
		}
		syncStyled += BehindStyle.Render(fmt.Sprintf("↓%d", r.Behind))
	}
	if syncStyled == "" {
		syncStyled = CleanStyle.Render("✓")
	}
	// In CI-block-jump mode (hlField=="subject") the highlight value is a CI
	// conclusion string, so we highlight the repo name to make the group clear.
	nameStyled := hlText(name, "subject", hlField, hlValue)
	if ciIcon == "" {
		ciIcon = DimStyle.Render("—")
	}

	row := "  " +
		cell(nameStyled, 30) + cell(branch, 14) + cell(statusStyled, 8) +
		cell(syncStyled, 10) + cell(prHeat(prCount), 4) + cell(branchHeat(branchCount), 4) +
		cell(ciIcon, 4) + cell(DimStyle.Render(author), 14) + DimStyle.Render(ago)
	if selected {
		return selRow(row)
	}
	return row
}

func RenderDashboard(groups []RepoGroup, cursor, width, scrollOffset, maxLines int, prCounts, branchCounts map[string]int, ciStatus map[string]string, hlField, hlValue string) string {
	var b strings.Builder

	// Column header — always visible, outside the scroll window
	header := "  " +
		cell("Repository", 30) + cell("Branch", 14) + cell("Status", 8) +
		cell("Sync", 10) + cell("PR", 4) + cell("Br", 4) + cell("CI", 4) + cell("Author", 14) + "Last Commit"
	b.WriteString(HeaderStyle.Render(header))
	b.WriteString("\n")

	sw := newScrollWriter(scrollOffset, maxLines)
	flatIdx := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			sw.writeLine("")
		}
		if g.Name != "" {
			sw.writeLine(groupHeader(g.Name, width))
		}
		for _, r := range g.Repos {
			sw.writeLine(renderRepoRow(r, flatIdx == cursor, prCounts[r.Name], branchCounts[r.Name], ciStatus[r.Name], hlField, hlValue))
			flatIdx++
		}
	}
	b.WriteString(sw.string())
	return b.String()
}

// ── CI row ────────────────────────────────────────────────────────────────

func formatCIStatus(status, conclusion string) string {
	if status != "completed" {
		return PendingStyle.Render("● " + status)
	}
	switch conclusion {
	case "success":
		return PassStyle.Render("✓ success")
	case "failure":
		return FailStyle.Render("✗ failure")
	case "timed_out":
		return FailStyle.Render("✗ timed_out")
	case "startup_failure":
		return FailStyle.Render("✗ startup_failure")
	case "cancelled":
		return DimStyle.Render("⊘ cancelled")
	case "skipped":
		return DimStyle.Render("— skipped")
	default:
		if conclusion != "" {
			return DimStyle.Render(conclusion)
		}
		return DimStyle.Render("—")
	}
}

func renderCIRow(r model.WorkflowRun, selected bool, hlField, hlValue string) string {
	repo := truncate(repoShortName(r.Repo), 26)
	wf := truncate(r.WorkflowName, 22)
	branch := truncate(r.Branch, 14)
	event := truncate(r.Event, 10)
	ago := timeAgo(r.UpdatedAt)

	repoStyled := hlText(repo, "repo", hlField, hlValue)
	wfStyled := hlText(wf, "subject", hlField, hlValue)
	statusStyled := formatCIStatus(r.Status, r.Conclusion)

	row := "  " +
		cell(repoStyled, 28) +
		cell(wfStyled, 24) +
		cell(DimStyle.Render(branch), 16) +
		cell(statusStyled, 18) +
		cell(DimStyle.Render(event), 12) +
		DimStyle.Render(ago)
	if selected {
		return selRow(row)
	}
	return row
}

func RenderCI(groups []CIGroup, cursor, width, scrollOffset, maxLines int, hlField, hlValue string) string {
	total := 0
	for _, g := range groups {
		total += len(g.Runs)
	}
	if total == 0 {
		return DimStyle.Render("\n  No CI run data found.\n")
	}

	var b strings.Builder
	header := "  " +
		cell("Repository", 28) + cell("Workflow", 24) + cell("Branch", 16) +
		cell("Status", 18) + cell("Event", 12) + "When"
	b.WriteString(HeaderStyle.Render(header))
	b.WriteString("\n")

	sw := newScrollWriter(scrollOffset, maxLines)
	flatIdx := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			sw.writeLine("")
		}
		if g.Name != "" {
			sw.writeLine(groupHeader(g.Name, width))
		}
		for _, r := range g.Runs {
			sw.writeLine(renderCIRow(r, flatIdx == cursor, hlField, hlValue))
			flatIdx++
		}
	}
	b.WriteString(sw.string())
	return b.String()
}

// ── PR row ────────────────────────────────────────────────────────────────

func renderPRRow(pr model.PR, selected bool, hlField, hlValue string) string {
	repo := truncate(repoShortName(pr.Repo), 26)
	title := truncate(pr.Title, 32)
	author := truncate(pr.Author, 22)
	ago := timeAgo(pr.UpdatedAt)

	repoStyled := hlText(repo, "repo", hlField, hlValue)
	titleStyled := hlText(title, "subject", hlField, hlValue)
	row := "  " +
		cell(repoStyled, 28) +
		cell(fmt.Sprintf("#%-4d", pr.Number), 7) +
		cell(titleStyled, 34) +
		cell(DimStyle.Render(author), 24) +
		cell(formatReview(pr.ReviewDecision), 14) +
		DimStyle.Render(ago)
	if selected {
		return selRow(row)
	}
	return row
}

func formatReview(decision string) string {
	switch decision {
	case "APPROVED":
		return ApprovedStyle.Render("✓ approved")
	case "CHANGES_REQUESTED":
		return ChangesStyle.Render("✗ changes")
	case "REVIEW_REQUIRED":
		return ReviewStyle.Render("● review")
	default:
		return DimStyle.Render("-")
	}
}

func RenderPRs(groups []PRGroup, cursor, width, scrollOffset, maxLines int, hlField, hlValue string) string {
	total := 0
	for _, g := range groups {
		total += len(g.PRs)
	}
	if total == 0 {
		return DimStyle.Render("\n  No open pull requests found.\n")
	}

	var b strings.Builder
	header := "  " +
		cell("Repository", 28) + cell("PR#", 7) + cell("Title", 34) +
		cell("Author", 24) + cell("Review", 14) + "Updated"
	b.WriteString(HeaderStyle.Render(header))
	b.WriteString("\n")

	sw := newScrollWriter(scrollOffset, maxLines)
	flatIdx := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			sw.writeLine("")
		}
		if g.Name != "" {
			sw.writeLine(groupHeader(g.Name, width))
		}
		for _, pr := range g.PRs {
			sw.writeLine(renderPRRow(pr, flatIdx == cursor, hlField, hlValue))
			flatIdx++
		}
	}
	b.WriteString(sw.string())
	return b.String()
}

// ── Branch row ────────────────────────────────────────────────────────────

func RenderBranches(groups []BranchGroup, cursor, width, scrollOffset, maxLines int, prBranches map[string]bool, hlField, hlValue string) string {
	total := 0
	for _, g := range groups {
		total += len(g.Branches)
	}
	if total == 0 {
		return DimStyle.Render("\n  No branches found.\n")
	}

	var b strings.Builder
	header := "  " + cell("Repository", 28) + cell("Branch", 28) + cell("Author", 18) + cell("When", 10) + cell("PR", 3) + "∈"
	b.WriteString(HeaderStyle.Render(header))
	b.WriteString("\n")

	sw := newScrollWriter(scrollOffset, maxLines)
	flatIdx := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			sw.writeLine("")
		}
		if g.Name != "" {
			sw.writeLine(groupHeader(g.Name, width))
		}
		for _, br := range g.Branches {
			repo := truncate(repoShortName(br.Repo), 26)
			name := truncate(br.Name, 26)
			author := truncate(br.Author, 16)
			ago := timeAgo(br.LastCommit)
			hasPR := prBranches[br.Name]
			if br.IsDefault {
				name = "* " + name
			}
			prStyled := "   "
			if hasPR {
				prStyled = ReviewStyle.Render("●") + "  "
			}
			mergedStyled := ""
			if br.IsMerged {
				mergedStyled = CleanStyle.Render("✓")
			}
			nameHL := hlText(name, "subject", hlField, hlValue)
			var nameStyled string
			if nameHL != name {
				nameStyled = nameHL // block-match overrides normal styling
			} else if br.IsDefault {
				nameStyled = CleanStyle.Render(name)
			} else {
				nameStyled = DimStyle.Render(name)
			}
			repoStyled := hlText(repo, "repo", hlField, hlValue)
			row := "  " +
				cell(repoStyled, 28) + cell(nameStyled, 28) +
				cell(DimStyle.Render(author), 18) + cell(DimStyle.Render(ago), 10) +
				cell(prStyled, 3) + mergedStyled
			if flatIdx == cursor {
				sw.writeLine(selRow(row))
			} else {
				sw.writeLine(row)
			}
			flatIdx++
		}
	}
	b.WriteString(sw.string())
	return b.String()
}

// ── Activity row ──────────────────────────────────────────────────────────

func RenderActivity(groups []CommitGroup, cursor, width, scrollOffset, maxLines int, hlField, hlValue string) string {
	total := 0
	for _, g := range groups {
		total += len(g.Commits)
	}
	if total == 0 {
		return DimStyle.Render("\n  No recent activity.\n")
	}

	var b strings.Builder
	header := "  " +
		cell("Repository", 26) + cell("Hash", 9) +
		cell("Message", 52) + cell("Author", 18) + "When"
	b.WriteString(HeaderStyle.Render(header))
	b.WriteString("\n")

	sw := newScrollWriter(scrollOffset, maxLines)
	flatIdx := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			sw.writeLine("")
		}
		if g.Name != "" {
			sw.writeLine(groupHeader(g.Name, width))
		}
		for _, c := range g.Commits {
			repo := truncate(c.Repo, 24)
			subject := truncate(c.Subject, 50)
			ago := timeAgo(c.Date)
			author := truncate(c.Author, 16)
			repoStyled := hlText(repo, "repo", hlField, hlValue)
			subjectStyled := hlText(subject, "subject", hlField, hlValue)
			row := "  " +
				cell(repoStyled, 26) + cell(DimStyle.Render(c.Hash), 9) +
				cell(subjectStyled, 52) + cell(DimStyle.Render(author), 18) +
				DimStyle.Render(ago)
			if flatIdx == cursor {
				sw.writeLine(selRow(row))
			} else {
				sw.writeLine(row)
			}
			flatIdx++
		}
	}
	b.WriteString(sw.string())
	return b.String()
}

// ── Errors ───────────────────────────────────────────────────────────────

// RenderErrors renders a list of load errors when a tab has no data.
func RenderErrors(errs []string) string {
	var b strings.Builder
	b.WriteString(DirtyStyle.Render(fmt.Sprintf("\n  Failed to load from %d repo(s):\n", len(errs))))
	for _, e := range errs {
		b.WriteString(DimStyle.Render("  • "+e) + "\n")
	}
	return b.String()
}

// RenderAuthError renders a prominent banner when gh is not authenticated.
func RenderAuthError() string {
	return "\n" +
		DirtyStyle.Render("  ✗ GitHub authentication required") + "\n\n" +
		DimStyle.Render("  Run the following command and then press r to retry:") + "\n\n" +
		HeaderStyle.Render("    gh auth login") + "\n\n" +
		DimStyle.Render("  If your org uses SAML SSO, also run:") + "\n" +
		DimStyle.Render("    gh auth refresh -h github.com --scopes read:org") + "\n"
}

// ── Tabs ──────────────────────────────────────────────────────────────────

func RenderTabs(tabs []string, active int) string {
	var rendered []string
	for i, t := range tabs {
		if i == active {
			rendered = append(rendered, ActiveTabStyle.Render(t))
		} else {
			rendered = append(rendered, TabStyle.Render(t))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func RenderProfileTabs(tabs []string, active int) string {
	var rendered []string
	for i, t := range tabs {
		if i == active {
			rendered = append(rendered, ActiveProfileTabStyle.Render(t))
		} else {
			rendered = append(rendered, ProfileTabStyle.Render(t))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}
