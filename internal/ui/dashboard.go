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
	b      strings.Builder
	vl     int // current visual line (0-indexed, excludes column header)
	offset int
	maxVL  int // offset + maxLines
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

func prHeat(n int) string     { return heatCount(n, 1, 3, 6) }
func branchHeat(n int) string { return heatCount(n, 3, 8, 16) }
func issueHeat(n int) string  { return heatCount(n, 1, 5, 10) }

// ── Flex column computation ─────────────────────────────────────────────

type colSpec struct {
	Min    int
	Weight int
}

// flexCols distributes avail width across columns proportionally to their
// weights, never going below each column's Min.
func flexCols(avail int, cols []colSpec) []int {
	widths := make([]int, len(cols))
	minTotal := 0
	for i, c := range cols {
		widths[i] = c.Min
		minTotal += c.Min
	}
	extra := avail - minTotal
	if extra <= 0 {
		return widths
	}
	totalWeight := 0
	for _, c := range cols {
		totalWeight += c.Weight
	}
	if totalWeight == 0 {
		return widths
	}
	for i, c := range cols {
		widths[i] += extra * c.Weight / totalWeight
	}
	return widths
}

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

func renderRepoRow(r model.Repo, selected bool, prCount, branchCount, issueCount int, ciIcon, hlField, hlValue string, nameW, branchW, authorW, agoW int) string {
	name := truncate(r.Name, nameW-2)
	branch := truncate(r.Branch, branchW-2)
	ago := timeAgo(r.LastCommit)
	author := truncate(r.LastAuthor, authorW-2)

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
		cell(nameStyled, nameW) + cell(branch, branchW) + cell(statusStyled, 8) +
		cell(syncStyled, 10) + cell(prHeat(prCount), 4) + cell(branchHeat(branchCount), 4) +
		cell(issueHeat(issueCount), 4) + cell(ciIcon, 4) + cell(DimStyle.Render(author), authorW) +
		cell(DimStyle.Render(ago), agoW)
	if selected {
		return selRow(row)
	}
	return row
}

func RenderDashboard(groups []RepoGroup, cursor, width, scrollOffset, maxLines int, prCounts, branchCounts, issueCounts map[string]int, ciStatus map[string]string, hlField, hlValue string) string {
	// indent(2) + status(8) + sync(10) + pr(4) + br(4) + is(4) + ci(4) = 36 fixed
	fixedW := 2 + 8 + 10 + 4 + 4 + 4 + 4
	cols := flexCols(width-fixedW, []colSpec{
		{Min: 16, Weight: 3}, // name
		{Min: 8, Weight: 1},  // branch
		{Min: 8, Weight: 1},  // author
		{Min: 7, Weight: 1},  // ago
	})
	nameW, branchW, authorW, agoW := cols[0], cols[1], cols[2], cols[3]

	var b strings.Builder
	header := "  " +
		cell("Repository", nameW) + cell("Branch", branchW) + cell("Status", 8) +
		cell("Sync", 10) + cell("PR", 4) + cell("Br", 4) + cell("Is", 4) + cell("CI", 4) +
		cell("Author", authorW) + cell("Last Commit", agoW)
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
			sw.writeLine(renderRepoRow(r, flatIdx == cursor, prCounts[r.Name], branchCounts[r.Name], issueCounts[r.Name], ciStatus[r.Name], hlField, hlValue, nameW, branchW, authorW, agoW))
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

func renderCIRow(r model.WorkflowRun, selected bool, hlField, hlValue string, repoW, wfW, branchW, agoW int) string {
	repo := truncate(repoShortName(r.Repo), repoW-2)
	wf := truncate(r.WorkflowName, wfW-2)
	branch := truncate(r.Branch, branchW-2)
	event := truncate(r.Event, 10)
	ago := timeAgo(r.UpdatedAt)

	repoStyled := hlText(repo, "repo", hlField, hlValue)
	wfStyled := hlText(wf, "subject", hlField, hlValue)
	statusStyled := formatCIStatus(r.Status, r.Conclusion)

	row := "  " +
		cell(repoStyled, repoW) +
		cell(wfStyled, wfW) +
		cell(DimStyle.Render(branch), branchW) +
		cell(statusStyled, 18) +
		cell(DimStyle.Render(event), 12) +
		cell(DimStyle.Render(ago), agoW)
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

	// indent(2) + status(18) + event(12) = 32 fixed
	fixedW := 2 + 18 + 12
	cols := flexCols(width-fixedW, []colSpec{
		{Min: 16, Weight: 2}, // repo
		{Min: 12, Weight: 2}, // workflow
		{Min: 8, Weight: 1},  // branch
		{Min: 7, Weight: 1},  // ago
	})
	repoW, wfW, branchW, agoW := cols[0], cols[1], cols[2], cols[3]

	var b strings.Builder
	header := "  " +
		cell("Repository", repoW) + cell("Workflow", wfW) + cell("Branch", branchW) +
		cell("Status", 18) + cell("Event", 12) + cell("When", agoW)
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
			sw.writeLine(renderCIRow(r, flatIdx == cursor, hlField, hlValue, repoW, wfW, branchW, agoW))
			flatIdx++
		}
	}
	b.WriteString(sw.string())
	return b.String()
}

// ── PR row ────────────────────────────────────────────────────────────────

func renderPRRow(pr model.PR, selected bool, hlField, hlValue string, repoW, titleW, authorW, agoW int) string {
	repo := truncate(repoShortName(pr.Repo), repoW-2)
	title := truncate(pr.Title, titleW-2)
	author := truncate(pr.Author, authorW-2)
	ago := timeAgo(pr.UpdatedAt)

	repoStyled := hlText(repo, "repo", hlField, hlValue)
	titleStyled := hlText(title, "subject", hlField, hlValue)
	row := "  " +
		cell(repoStyled, repoW) +
		cell(fmt.Sprintf("#%-4d", pr.Number), 7) +
		cell(titleStyled, titleW) +
		cell(DimStyle.Render(author), authorW) +
		cell(formatReview(pr.ReviewDecision), 14) +
		cell(formatChecks(pr.Checks), 12) +
		cell(DimStyle.Render(ago), agoW)
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

func formatChecks(checks string) string {
	switch checks {
	case "pass":
		return PassStyle.Render("✓ pass")
	case "fail":
		return FailStyle.Render("✗ fail")
	case "pending":
		return PendingStyle.Render("● pending")
	default:
		return DimStyle.Render("—")
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

	// indent(2) + PR#(7) + review(14) + checks(12) = 35 fixed
	fixedW := 2 + 7 + 14 + 12
	cols := flexCols(width-fixedW, []colSpec{
		{Min: 16, Weight: 2}, // repo
		{Min: 16, Weight: 3}, // title
		{Min: 8, Weight: 1},  // author
		{Min: 7, Weight: 1},  // ago
	})
	repoW, titleW, authorW, agoW := cols[0], cols[1], cols[2], cols[3]

	var b strings.Builder
	header := "  " +
		cell("Repository", repoW) + cell("PR#", 7) + cell("Title", titleW) +
		cell("Author", authorW) + cell("Review", 14) + cell("Checks", 12) + cell("Updated", agoW)
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
			sw.writeLine(renderPRRow(pr, flatIdx == cursor, hlField, hlValue, repoW, titleW, authorW, agoW))
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

	// indent(2) + PR(3) + merged(2) = 7 fixed
	fixedW := 2 + 3 + 2
	cols := flexCols(width-fixedW, []colSpec{
		{Min: 16, Weight: 2}, // repo
		{Min: 16, Weight: 2}, // branch
		{Min: 10, Weight: 1}, // author
		{Min: 7, Weight: 1},  // when
	})
	repoW, branchW, authorW, agoW := cols[0], cols[1], cols[2], cols[3]

	var b strings.Builder
	header := "  " + cell("Repository", repoW) + cell("Branch", branchW) + cell("PR", 3) + cell("∈", 2) + cell("Author", authorW) + cell("When", agoW)
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
			repo := truncate(repoShortName(br.Repo), repoW-2)
			name := truncate(br.Name, branchW-2)
			author := truncate(br.Author, authorW-2)
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
				nameStyled = nameHL
			} else if br.IsDefault {
				nameStyled = CleanStyle.Render(name)
			} else {
				nameStyled = DimStyle.Render(name)
			}
			repoStyled := hlText(repo, "repo", hlField, hlValue)
			row := "  " +
				cell(repoStyled, repoW) + cell(nameStyled, branchW) +
				cell(prStyled, 3) + cell(mergedStyled, 2) +
				cell(DimStyle.Render(author), authorW) +
				cell(DimStyle.Render(ago), agoW)
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

	// indent(2) + hash(9) = 11 fixed
	fixedW := 2 + 9
	cols := flexCols(width-fixedW, []colSpec{
		{Min: 14, Weight: 2}, // repo
		{Min: 20, Weight: 5}, // message
		{Min: 10, Weight: 1}, // author
		{Min: 7, Weight: 1},  // ago
	})
	repoW, msgW, authorW, agoW := cols[0], cols[1], cols[2], cols[3]

	var b strings.Builder
	header := "  " +
		cell("Repository", repoW) + cell("Hash", 9) +
		cell("Message", msgW) + cell("Author", authorW) + cell("When", agoW)
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
			repo := truncate(c.Repo, repoW-2)
			subject := truncate(c.Subject, msgW-2)
			ago := timeAgo(c.Date)
			author := truncate(c.Author, authorW-2)
			repoStyled := hlText(repo, "repo", hlField, hlValue)
			subjectStyled := hlText(subject, "subject", hlField, hlValue)
			row := "  " +
				cell(repoStyled, repoW) + cell(DimStyle.Render(c.Hash), 9) +
				cell(subjectStyled, msgW) + cell(DimStyle.Render(author), authorW) +
				cell(DimStyle.Render(ago), agoW)
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

// ── Issue row ────────────────────────────────────────────────────────────

func renderIssueRow(iss model.Issue, selected bool, hlField, hlValue string, repoW, titleW, authorW, labelsW, assigneesW, milestoneW, agoW int) string {
	repo := truncate(repoShortName(iss.Repo), repoW-2)
	title := truncate(iss.Title, titleW-2)
	author := truncate(iss.Author, authorW-2)
	labels := truncate(strings.Join(iss.Labels, ", "), labelsW-2)
	assignees := truncate(strings.Join(iss.Assignees, ", "), assigneesW-2)
	milestone := truncate(iss.Milestone, milestoneW-2)
	ago := timeAgo(iss.UpdatedAt)

	repoStyled := hlText(repo, "repo", hlField, hlValue)
	titleStyled := hlText(title, "subject", hlField, hlValue)
	row := "  " +
		cell(repoStyled, repoW) +
		cell(fmt.Sprintf("#%-4d", iss.Number), 7) +
		cell(titleStyled, titleW) +
		cell(DimStyle.Render(author), authorW) +
		cell(DimStyle.Render(labels), labelsW) +
		cell(DimStyle.Render(assignees), assigneesW) +
		cell(DimStyle.Render(milestone), milestoneW) +
		cell(DimStyle.Render(ago), agoW)
	if selected {
		return selRow(row)
	}
	return row
}

func RenderIssues(groups []IssueGroup, cursor, width, scrollOffset, maxLines int, hlField, hlValue string) string {
	total := 0
	for _, g := range groups {
		total += len(g.Issues)
	}
	if total == 0 {
		return DimStyle.Render("\n  No open issues found.\n")
	}

	// indent(2) + #(7) = 9 fixed
	fixedW := 2 + 7
	cols := flexCols(width-fixedW, []colSpec{
		{Min: 14, Weight: 2}, // repo
		{Min: 16, Weight: 3}, // title
		{Min: 8, Weight: 1},  // author
		{Min: 10, Weight: 2}, // labels
		{Min: 8, Weight: 1},  // assignees
		{Min: 8, Weight: 1},  // milestone
		{Min: 7, Weight: 1},  // ago
	})
	repoW, titleW, authorW := cols[0], cols[1], cols[2]
	labelsW, assigneesW, milestoneW, agoW := cols[3], cols[4], cols[5], cols[6]

	var b strings.Builder
	header := "  " +
		cell("Repository", repoW) + cell("#", 7) + cell("Title", titleW) +
		cell("Author", authorW) + cell("Labels", labelsW) + cell("Assignee", assigneesW) +
		cell("Milestone", milestoneW) + cell("Updated", agoW)
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
		for _, iss := range g.Issues {
			sw.writeLine(renderIssueRow(iss, flatIdx == cursor, hlField, hlValue, repoW, titleW, authorW, labelsW, assigneesW, milestoneW, agoW))
			flatIdx++
		}
	}
	b.WriteString(sw.string())
	return b.String()
}

// ── Tabs ──────────────────────────────────────────────────────────────────

func RenderTabs(tabs []string, active, width int) string {
	// Find the longest tab label to set a uniform column width.
	colW := 0
	for _, t := range tabs {
		if len(t) > colW {
			colW = len(t)
		}
	}

	// Measure the rendered width of a single tab cell (label + padding).
	cellW := lipgloss.Width(TabStyle.Render(fmt.Sprintf("%-*s", colW, "")))

	// Determine how many tabs fit per row; fall back to all-on-one-row
	// when width is unknown (zero) or large enough.
	perRow := len(tabs)
	if width > 0 && cellW > 0 {
		perRow = width / cellW
		if perRow < 1 {
			perRow = 1
		}
		if perRow > len(tabs) {
			perRow = len(tabs)
		}
	}

	var rows []string
	for start := 0; start < len(tabs); start += perRow {
		end := min(start+perRow, len(tabs))
		var row []string
		for i := start; i < end; i++ {
			label := fmt.Sprintf("%-*s", colW, tabs[i])
			if i == active {
				row = append(row, ActiveTabStyle.Render(label))
			} else {
				row = append(row, TabStyle.Render(label))
			}
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(rows, "\n")
}

// TabHitTest returns the tab index for a mouse click at column x on the
// given row (0-based, relative to the first tab row). Returns -1 if outside
// all tabs. The layout mirrors RenderTabs exactly.
func TabHitTest(tabs []string, x, row, width int) int {
	colW := 0
	for _, t := range tabs {
		if len(t) > colW {
			colW = len(t)
		}
	}
	cellW := lipgloss.Width(TabStyle.Render(fmt.Sprintf("%-*s", colW, "")))
	if cellW == 0 {
		return -1
	}

	perRow := len(tabs)
	if width > 0 {
		perRow = width / cellW
		if perRow < 1 {
			perRow = 1
		}
		if perRow > len(tabs) {
			perRow = len(tabs)
		}
	}

	col := x / cellW
	if col >= perRow {
		return -1
	}
	idx := row*perRow + col
	if idx < 0 || idx >= len(tabs) {
		return -1
	}
	// Verify click is within the cell bounds
	if x < col*cellW || x >= (col+1)*cellW {
		return -1
	}
	return idx
}

// TabRows returns how many rows the tab bar occupies at the given width.
func TabRows(tabs []string, width int) int {
	colW := 0
	for _, t := range tabs {
		if len(t) > colW {
			colW = len(t)
		}
	}
	cellW := lipgloss.Width(TabStyle.Render(fmt.Sprintf("%-*s", colW, "")))
	if cellW == 0 {
		return 1
	}
	perRow := len(tabs)
	if width > 0 {
		perRow = width / cellW
		if perRow < 1 {
			perRow = 1
		}
		if perRow > len(tabs) {
			perRow = len(tabs)
		}
	}
	return (len(tabs) + perRow - 1) / perRow
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
