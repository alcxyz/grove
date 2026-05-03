package ui

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/alcxyz/grove/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// detailSection writes a section header and returns a helper that writes rows,
// capped at maxRows, appending a "… N more" dim line when truncated.
// lineNum tracks the current rendered line; highlightLine selects one row.
func detailSection(b *strings.Builder, title string, total, maxRows int, lineNum *int, highlightLine int) func(row string) {
	b.WriteString(HeaderStyle.Render(title))
	b.WriteString("\n")
	*lineNum++
	written := 0
	return func(row string) {
		if written < maxRows {
			if *lineNum == highlightLine {
				b.WriteString(selRow(row))
			} else {
				b.WriteString(row)
			}
			b.WriteString("\n")
			*lineNum++
		} else if written == maxRows {
			remaining := total - maxRows
			if remaining > 0 {
				b.WriteString(DimStyle.Render(fmt.Sprintf("  … %d more", remaining)))
				b.WriteString("\n")
				*lineNum++
			}
		}
		written++
	}
}

func RenderRepoDetail(repo model.Repo, commits []model.Commit, prs []model.PR, issues []model.Issue, localBranches []string, remoteBranches []model.BranchInfo, runs []model.WorkflowRun, stats model.RepoStats, width int, highlightLine int) string {
	var b strings.Builder
	lineNum := 0

	// Repo header
	b.WriteString(HeaderStyle.Render(fmt.Sprintf("  %s", repo.Name)))
	b.WriteString("\n")
	lineNum++
	b.WriteString(DimStyle.Render(strings.Repeat("─", min(width, 90))))
	b.WriteString("\n\n")
	lineNum += 2

	// Status info
	status := CleanStyle.Render("clean")
	if repo.Dirty {
		status = DirtyStyle.Render("dirty")
	}
	syncInfo := ""
	if repo.Ahead > 0 {
		syncInfo += AheadStyle.Render(fmt.Sprintf("↑%d ahead", repo.Ahead))
	}
	if repo.Behind > 0 {
		if syncInfo != "" {
			syncInfo += "  "
		}
		syncInfo += BehindStyle.Render(fmt.Sprintf("↓%d behind", repo.Behind))
	}
	if syncInfo == "" {
		syncInfo = CleanStyle.Render("up to date")
	}

	fmt.Fprintf(&b, "  Branch:  %s\n", HeaderStyle.Render(repo.Branch))
	lineNum++
	fmt.Fprintf(&b, "  Status:  %s\n", status)
	lineNum++
	fmt.Fprintf(&b, "  Sync:    %s\n", syncInfo)
	lineNum++
	fmt.Fprintf(&b, "  Path:    %s\n", DimStyle.Render(repo.Path))
	lineNum++
	if stats.CommitCount > 0 || stats.Contributors > 0 {
		fmt.Fprintf(&b, "  Commits: %s", HeaderStyle.Render(fmt.Sprintf("%d", stats.CommitCount)))
		if stats.Contributors > 0 {
			fmt.Fprintf(&b, "   Contributors: %s", HeaderStyle.Render(fmt.Sprintf("%d", stats.Contributors)))
		}
		b.WriteString("\n")
		lineNum++
	}
	b.WriteString("\n")
	lineNum++

	// ── Local branches ───────────────────────────────────────────────────────
	write := detailSection(&b, fmt.Sprintf("  Local branches (%d)", len(localBranches)), len(localBranches), 12, &lineNum, highlightLine)
	if len(localBranches) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
		lineNum++
	} else {
		for _, br := range localBranches {
			marker := "  "
			if br == repo.Branch {
				marker = SelectedStyle.Render("* ")
			}
			write(fmt.Sprintf("  %s%s", marker, br))
		}
	}
	b.WriteString("\n")
	lineNum++

	// ── Remote branches ──────────────────────────────────────────────────────
	prBranches := map[string]bool{}
	for _, pr := range prs {
		prBranches[pr.Branch] = true
	}
	write = detailSection(&b, fmt.Sprintf("  Remote branches (%d)", len(remoteBranches)), len(remoteBranches), 12, &lineNum, highlightLine)
	if len(remoteBranches) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
		lineNum++
	} else {
		for _, br := range remoteBranches {
			marker := "  "
			if br.Name == repo.Branch {
				marker = SelectedStyle.Render("* ")
			}
			name := truncate(br.Name, 28)
			author := truncate(br.Author, 14)
			ago := timeAgo(br.LastCommit)
			prMark := "  "
			if prBranches[br.Name] {
				prMark = ReviewStyle.Render("●") + " "
			}
			mergedMark := "  "
			if br.IsMerged {
				mergedMark = CleanStyle.Render("∈") + " "
			}
			write("  " + marker + cell(name, 30) + prMark + mergedMark +
				cell(DimStyle.Render(author), 16) + DimStyle.Render(ago))
		}
	}
	b.WriteString("\n")
	lineNum++

	// ── Open PRs ─────────────────────────────────────────────────────────────
	write = detailSection(&b, fmt.Sprintf("  Open PRs (%d)", len(prs)), len(prs), 10, &lineNum, highlightLine)
	if len(prs) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
		lineNum++
	} else {
		for _, pr := range prs {
			title := truncate(pr.Title, 52)
			write(fmt.Sprintf("  #%-4d %s  %s  %s",
				pr.Number, title, formatReview(pr.ReviewDecision), DimStyle.Render(pr.Author)))
		}
	}
	b.WriteString("\n")
	lineNum++

	// ── Open Issues ──────────────────────────────────────────────────────────
	write = detailSection(&b, fmt.Sprintf("  Open issues (%d)", len(issues)), len(issues), 10, &lineNum, highlightLine)
	if len(issues) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
		lineNum++
	} else {
		for _, iss := range issues {
			title := truncate(iss.Title, 48)
			labels := ""
			if len(iss.Labels) > 0 {
				labels = DimStyle.Render(truncate(strings.Join(iss.Labels, ", "), 20))
			}
			write(fmt.Sprintf("  #%-4d %s  %s  %s",
				iss.Number, title, labels, DimStyle.Render(iss.Author)))
		}
	}
	b.WriteString("\n")
	lineNum++

	// ── CI runs ──────────────────────────────────────────────────────────────
	write = detailSection(&b, fmt.Sprintf("  CI runs (%d)", len(runs)), len(runs), 8, &lineNum, highlightLine)
	if len(runs) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
		lineNum++
	} else {
		for _, r := range runs {
			wf := truncate(r.WorkflowName, 24)
			branch := truncate(r.Branch, 18)
			ago := timeAgo(r.UpdatedAt)
			write("  " + cell(CIStatusIcon(r.Status, r.Conclusion), 14) +
				cell(wf, 26) + cell(DimStyle.Render(branch), 20) + DimStyle.Render(ago))
		}
	}
	b.WriteString("\n")
	lineNum++

	// ── Recent commits ───────────────────────────────────────────────────────
	write = detailSection(&b, fmt.Sprintf("  Recent commits (%d)", len(commits)), len(commits), 10, &lineNum, highlightLine)
	if len(commits) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
		lineNum++
	} else {
		for _, c := range commits {
			subject := truncate(c.Subject, 50)
			ago := timeAgo(c.Date)
			write("  " + cell(DimStyle.Render(c.Hash), 9) +
				cell(subject, 52) + cell(DimStyle.Render(c.Author), 16) + DimStyle.Render(ago))
		}
	}

	return b.String()
}

// splashArt is the ASCII art for the splash/about screen (! key) and screensaver.
// Source of truth: internal/ui/logo.txt
//
//go:embed logo.txt
var splashArt string

// splashArtLines are the art lines pre-split for screensaver positioning.
var splashArtLines = strings.Split(splashArt, "\n")

// SplashArtWidth / SplashArtHeight are exported for the screensaver bounce bounds.
var SplashArtWidth = func() int {
	w := 0
	for _, l := range splashArtLines {
		if len(l) > w {
			w = len(l)
		}
	}
	return w
}()

var SplashArtHeight = len(splashArtLines)

// BottomChromeHeight is the number of fixed lines at the bottom of the screen
// (status bar row + info bar row). Exported for contentHeight calculations.
const BottomChromeHeight = 2

// ssColors is the Catppuccin Mocha palette the screensaver cycles through.
var ssColors = []string{"#cba6f7", "#89b4fa", "#a6e3a1", "#f38ba8", "#fab387", "#94e2d5", "#f9e2af"}

// RenderScreensaver renders the bouncing grove logo at position (x, y) with
// a cycling color index, filling a width×height terminal frame.
func RenderScreensaver(x, y, colorIdx, width, height int) string {
	artH := len(splashArtLines)
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ssColors[colorIdx%len(ssColors)]))

	var b strings.Builder
	for row := 0; row < height; row++ {
		if row >= y && row < y+artH {
			line := splashArtLines[row-y]
			pad := max(0, x)
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(style.Render(line))
		}
		if row < height-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RenderOwlEyes renders the animated owl eyes "{o,o}" with teal-coloured irises.
// blinkState controls which eyes are closed: 0=both open, 1=left closed,
// 2=right closed, 3=both closed.
func RenderOwlEyes(blinkState int) string {
	eyeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(catTeal))
	left, right := "o", "o"
	if blinkState == 1 || blinkState == 3 {
		left = "-"
	}
	if blinkState == 2 || blinkState == 3 {
		right = "-"
	}
	return HeaderStyle.Render("{") +
		eyeStyle.Render(left) +
		HeaderStyle.Render(",") +
		eyeStyle.Render(right) +
		HeaderStyle.Render("}")
}

// RenderBottomArea renders the two-line bottom chrome: status bar then info bar.
// It outputs exactly BottomChromeHeight (2) lines with no leading or trailing newline.
// The owl eyes are anchored on the left of the status bar line.
func RenderBottomArea(statusMsg string, loading bool, infoParts []string, blinkState, _ int) string {
	s := statusMsg
	if loading {
		s = "⏳ " + s
	}
	return RenderOwlEyes(blinkState) + " " + StatusBarStyle.Render(s) + "\n" + RenderInfoBar(infoParts)
}

// renderSplashLine renders one line of the ASCII art.
// The eyes in "{o,o}" are coloured teal; blinkState controls which are closed:
// 0=both open  1=left closed  2=right closed  3=both closed.
func renderSplashLine(l string, blinkState int) string {
	const eyes = "{o,o}"
	idx := strings.Index(l, eyes)
	if idx < 0 {
		return HeaderStyle.Render(l)
	}
	eyeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(catTeal))
	left, right := "o", "o"
	if blinkState == 1 || blinkState == 3 {
		left = "-"
	}
	if blinkState == 2 || blinkState == 3 {
		right = "-"
	}
	return HeaderStyle.Render(l[:idx]) +
		HeaderStyle.Render("{") +
		eyeStyle.Render(left) +
		HeaderStyle.Render(",") +
		eyeStyle.Render(right) +
		HeaderStyle.Render("}") +
		HeaderStyle.Render(l[idx+len(eyes):])
}

// RenderSplash renders the about/splash overlay (! key).
// blinkState drives the eye-blink animation (0=open, 1=left, 2=right, 3=both).
func RenderSplash(configPath, cacheDir, logPath, version string, width, blinkState int) string {
	var lines []string
	lines = append(lines, "")
	for _, l := range strings.Split(splashArt, "\n") {
		lines = append(lines, "  "+renderSplashLine(l, blinkState))
	}
	lines = append(lines, "")
	lines = append(lines, "  "+DimStyle.Render("press ! to close"))
	lines = append(lines, "")
	lines = append(lines, "  "+cell(DimStyle.Render("version"), 10)+version)
	lines = append(lines, "  "+cell(DimStyle.Render("config"), 10)+configPath)
	lines = append(lines, "  "+cell(DimStyle.Render("cache"), 10)+cacheDir)
	lines = append(lines, "  "+cell(DimStyle.Render("log"), 10)+logPath)
	lines = append(lines, "")

	boxW := min(width-4, 72)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 2).
		Width(boxW)

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box.Render(strings.Join(lines, "\n")))
}

func RenderFilter(query string) string {
	if query == "" {
		return ""
	}
	return fmt.Sprintf(" %s %s", DimStyle.Render("/"), HeaderStyle.Render(query))
}

// RenderCycleFilter shows the active quick-cycle filter pill.
func RenderCycleFilter(field, value string, pos, total int) string {
	return "  " + CycleStyle.Render(fmt.Sprintf("◉ %s: %s", field, value)) +
		DimStyle.Render(fmt.Sprintf(" %d/%d", pos, total))
}

// RenderSortIndicator shows the active sort direction.
func RenderSortIndicator(field string, asc bool) string {
	if asc {
		return "  " + SortAscStyle.Render("↑ "+field)
	}
	return "  " + SortDscStyle.Render("↓ "+field)
}

// ── Help overlay ──────────────────────────────────────────────────────────

// helpPages holds the two pages of key-binding reference shown by the ? overlay.
// Page 0 — daily-use navigation and actions.
// Page 1 — filters, sort, column reference, CLI.
var helpPages = [2][]struct {
	title string
	rows  [][2]string
}{
	// ── Page 1: Navigation & Actions ─────────────────────────────────────────
	{
		{"Navigation", [][2]string{
			{"j / k", "move down / up"},
			{"h / l", "previous / next tab"},
			{"H / L", "previous / next profile"},
			{"gg / G", "first / last item"},
			{"tab / shift+tab", "page jump  (accelerates: 5 → 10 → 20 → 25)"},
			{"{ / }", "jump between groups"},
			{"[ / ]", "jump between repo blocks  (tab 1: dirty/behind)"},
			{"( / )", "jump between subject blocks  (tab 1: CI status)"},
			{"1–5", "switch to tab directly"},
		}},
		{"Actions", [][2]string{
			{"enter", "open detail (tabs 1–4) · open diff (tab 5)"},
			{"o", "open in browser"},
			{"space", "diffnav · gh-dash · lazygit · workflow (per tab)"},
			{"e", "open $EDITOR / nvim at repo root"},
			{"p", "git pull current repo"},
			{"r", "refresh current tab"},
			{"R", "toggle auto-refresh"},
			{"ctrl+f", "git fetch all repos"},
			{"g (single)", "toggle grouped / flat view"},
			{",", "config preview  (profile in main views, repo in detail)"},
			{"!", "about / paths"},
			{"?", "this help"},
			{"q / ctrl+c", "quit"},
		}},
	},
	// ── Page 2: Filters, Sort & Reference ────────────────────────────────────
	{
		{"Filters & sort", [][2]string{
			{"/", "open text filter  ·  esc clear"},
			{"d / D", "cycle by author  ·  sort ↑↓ by author"},
			{"s / S", "cycle by subject  ·  sort ↑↓ by name/title"},
			{"a / A", "cycle by repository  ·  sort ↑↓ by repository  (tabs 2–5)"},
			{"f / F", "cycle by date  ·  sort ↑↓ by date/updated"},
			{"x / X", "cycle · sort  PR count / review status  (tabs 1 2 4)"},
			{"c / C", "cycle · sort  branch count / merged  (tabs 1 3 4)"},
			{"v / V", "cycle · sort  CI status / checks result  (tabs 1 2 3)"},
		}},
		{"Column reference", [][2]string{
			{"PR / Br  (tab 1)", "open PR count · branch count  (heat: blue→yellow→red)"},
			{"CI  (tab 1)", "latest run: ✓ success · ✗ failure · ● running · — none"},
			{"Checks  (tab 2)", "PR status check rollup: ✓ pass · ✗ fail · ● pending · — none"},
			{"↑n / ↓n  (tab 1)", "commits ahead / behind remote"},
			{"dirty / clean  (tab 1)", "working tree has uncommitted changes"},
			{"✓  (tab 1)", "sync — local and remote are even"},
			{"●  (tab 4)", "branch has an open PR"},
			{"∈  (tab 4)", "branch merged into the default branch"},
		}},
		{"CLI", [][2]string{
			{"grove clone", "clone missing org repos into base_paths"},
			{"grove clone <profile>", "clone for a specific profile only"},
			{"grove -v", "print version and config/cache/log paths"},
		}},
	},
}

// wrapHelpLine renders one help-row entry with a hanging indent so that
// description text that overflows the available width continues aligned
// under itself, not under the key label.
//
// contentW is the box content width (same value passed to lipgloss Width).
// keyW is the padded key column width.
func wrapHelpLine(key, desc string, keyW, contentW int) string {
	prefixW := 2 + keyW         // "  " + padded key column
	descW := contentW - prefixW // chars available for description
	prefix := "  " + cell(DimStyle.Render(key), keyW)
	if descW <= 0 {
		return prefix + desc
	}

	// All characters used in help descriptions are single display-column wide
	// (ASCII + ✓ ✗ · ↑ ↓ ● ∈ —), so rune count == visual width here.
	runes := []rune(desc)
	if len(runes) <= descW {
		return prefix + desc
	}

	indent := strings.Repeat(" ", prefixW)
	var segments []string
	start := 0
	for start < len(runes) {
		end := start + descW
		if end >= len(runes) {
			segments = append(segments, string(runes[start:]))
			break
		}
		// Walk back from end to find the last space to break on.
		bp := -1
		for i := end; i > start; i-- {
			if runes[i] == ' ' {
				bp = i
				break
			}
		}
		if bp < 0 {
			bp = end // hard break — no space found
		}
		segments = append(segments, strings.TrimRight(string(runes[start:bp]), " "))
		// Skip any spaces at the break point so the next segment starts clean.
		start = bp
		for start < len(runes) && runes[start] == ' ' {
			start++
		}
	}

	if len(segments) == 0 {
		return prefix + desc
	}
	out := prefix + segments[0]
	for _, seg := range segments[1:] {
		out += "\n" + indent + seg
	}
	return out
}

// RenderHelp renders the keybinding reference as a centred bordered box.
// page selects which of the two pages to show (0 or 1).
func RenderHelp(width, page int, version string) string {
	page = page % 2
	sections := helpPages[page]

	boxW := min(width-4, 90)
	keyW := 22
	var lines []string
	for _, s := range sections {
		lines = append(lines, "")
		lines = append(lines, HeaderStyle.Render(s.title))
		for _, r := range s.rows {
			lines = append(lines, wrapHelpLine(r[0], r[1], keyW, boxW))
		}
	}
	lines = append(lines, "")
	lines = append(lines, DimStyle.Render(fmt.Sprintf("  grove  v%s", version))+
		"   "+DimStyle.Render(fmt.Sprintf("page %d / 2", page+1))+
		"   "+DimStyle.Render("h / l  flip page"))
	lines = append(lines, "")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 2).
		Width(boxW)

	inner := strings.Join(lines, "\n")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box.Render(inner))
}

// RenderConfigPreview renders a human-readable view of the active config.
func RenderConfigPreview(content string, width int) string {
	boxW := min(width-4, 100)
	if boxW < 40 {
		boxW = width
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "Config preview"):
			lines[i] = HeaderStyle.Render(line)
		case strings.HasPrefix(line, "Profile:"),
			strings.HasPrefix(line, "Resolved remotes"),
			strings.HasPrefix(line, "Default remotes"),
			strings.HasPrefix(line, "Groups"),
			strings.HasPrefix(line, "Repo overrides"),
			strings.HasPrefix(line, "Matching group"):
			lines[i] = HeaderStyle.Render(line)
		case strings.HasPrefix(line, "  - "):
			lines[i] = GroupHeaderStyle.Render(line)
		case strings.HasPrefix(line, "    "):
			lines[i] = DimStyle.Render(line)
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 2).
		Width(boxW)

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box.Render(strings.Join(lines, "\n")))
}

// RenderInfoBar renders the context-aware bottom info bar.
func RenderInfoBar(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return DimStyle.Render("  " + strings.Join(parts, "   ·   "))
}

// ── Diff view ─────────────────────────────────────────────────────────────

// RenderDiff returns the fully-rendered diff as a string; the caller is
// responsible for slicing it against a scroll offset to fit the viewport.
// When preColored is true the content is already ANSI-colored (e.g. via delta)
// and colorDiffLine is skipped.
func RenderDiff(repoName, hash, content string, preColored bool) string {
	var b strings.Builder
	b.WriteString(HeaderStyle.Render(fmt.Sprintf("  %s  %s", repoName, hash)))
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  " + strings.Repeat("─", 86)))
	b.WriteString("\n")

	for _, line := range strings.Split(content, "\n") {
		if preColored {
			b.WriteString(line)
		} else {
			b.WriteString(colorDiffLine(line))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func colorDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "diff --git") ||
		strings.HasPrefix(line, "new file") ||
		strings.HasPrefix(line, "deleted file") ||
		strings.HasPrefix(line, "rename "):
		return DiffFileStyle.Render(line)
	case strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "similarity "):
		return DimStyle.Render(line)
	case strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
		return DimStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return DiffHunkStyle.Render(line)
	case strings.HasPrefix(line, "+"):
		return DiffAddStyle.Render(line)
	case strings.HasPrefix(line, "-"):
		return DiffRemStyle.Render(line)
	case strings.HasPrefix(line, "commit ") || strings.HasPrefix(line, "Author:") ||
		strings.HasPrefix(line, "Date:") || strings.HasPrefix(line, "Merge:"):
		return DimStyle.Render(line)
	default:
		return line
	}
}
