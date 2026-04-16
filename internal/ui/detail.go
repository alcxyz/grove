package ui

import (
	"fmt"
	"strings"

	"github.com/alcxyz/grove/internal/model"
	"github.com/charmbracelet/lipgloss"
)

func RenderRepoDetail(repo model.Repo, commits []model.Commit, prs []model.PR, branches []string, stats model.RepoStats, width int) string {
	var b strings.Builder

	// Repo header
	b.WriteString(HeaderStyle.Render(fmt.Sprintf("  %s", repo.Name)))
	b.WriteString("\n")
	b.WriteString(DimStyle.Render(strings.Repeat("─", min(width, 90))))
	b.WriteString("\n\n")

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

	b.WriteString(fmt.Sprintf("  Branch:  %s\n", HeaderStyle.Render(repo.Branch)))
	b.WriteString(fmt.Sprintf("  Status:  %s\n", status))
	b.WriteString(fmt.Sprintf("  Sync:    %s\n", syncInfo))
	b.WriteString(fmt.Sprintf("  Path:    %s\n", DimStyle.Render(repo.Path)))
	if stats.CommitCount > 0 || stats.Contributors > 0 {
		b.WriteString(fmt.Sprintf("  Commits: %s", HeaderStyle.Render(fmt.Sprintf("%d", stats.CommitCount))))
		if stats.Contributors > 0 {
			b.WriteString(fmt.Sprintf("   Contributors: %s", HeaderStyle.Render(fmt.Sprintf("%d", stats.Contributors))))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Local branches
	b.WriteString(HeaderStyle.Render("  Branches"))
	b.WriteString("\n")
	if len(branches) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
	} else {
		for _, br := range branches {
			marker := "  "
			if br == repo.Branch {
				marker = SelectedStyle.Render("* ")
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", marker, br))
		}
	}
	b.WriteString("\n")

	// Open PRs
	b.WriteString(HeaderStyle.Render(fmt.Sprintf("  Open PRs (%d)", len(prs))))
	b.WriteString("\n")
	if len(prs) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
	} else {
		for _, pr := range prs {
			title := truncate(pr.Title, 60)
			b.WriteString(fmt.Sprintf("  #%-4d %s  %s\n",
				pr.Number, title, DimStyle.Render(pr.Author)))
		}
	}
	b.WriteString("\n")

	// Recent commits
	b.WriteString(HeaderStyle.Render(fmt.Sprintf("  Recent Commits (%d)", len(commits))))
	b.WriteString("\n")
	if len(commits) == 0 {
		b.WriteString(DimStyle.Render("  (none)\n"))
	} else {
		for _, c := range commits {
			subject := truncate(c.Subject, 55)
			ago := timeAgo(c.Date)
			b.WriteString(fmt.Sprintf("  %s %s  %s  %s\n",
				DimStyle.Render(c.Hash), subject, DimStyle.Render(c.Author), DimStyle.Render(ago)))
		}
	}

	return b.String()
}

// splashArt is the ASCII art for the splash/about screen (! key) and screensaver.
// Generated with figlet standard font for "grove".
var splashArt = `   __ _ _ __ _____   _____
  / _` + "`" + ` | '__/ _ \ \ / / _ \
 | (_| | | | (_) \ V /  __/
  \__, |_|  \___/ \_/ \___|
     |_|`

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
		b.WriteString("\n")
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RenderSplash renders the about/splash overlay (! key).
func RenderSplash(configPath, cacheDir, logPath string, width int) string {
	var lines []string
	lines = append(lines, "")
	for _, l := range strings.Split(splashArt, "\n") {
		lines = append(lines, "  "+HeaderStyle.Render(l))
	}
	lines = append(lines, "")
	lines = append(lines, "  "+DimStyle.Render("git repository monitor  ·  press ! to close"))
	lines = append(lines, "")
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

// RenderHelp renders a full keybinding reference as a centred bordered box.
func RenderHelp(width int) string {
	sections := []struct {
		title string
		rows  [][2]string
	}{
		{"Navigation", [][2]string{
			{"j / k", "move down / up"},
			{"gg / G", "first / last item"},
			{"{ / }", "jump between groups"},
			{"[ / ]", "jump between repo blocks  (tab 1: dirty/behind repos)"},
			{"( / )", "jump between subject/branch/message blocks"},
			{"tab / shift+tab", "next / previous tab"},
			{"1 / 2 / 3 / 4", "switch to tab directly"},
		}},
		{"Filters & sort", [][2]string{
			{"/", "open text filter"},
			{"esc", "clear active filter / close pane"},
			{"d / D", "cycle by author  ·  sort ↑↓ by author  (all tabs)"},
			{"s / S", "cycle by subject prefix  ·  sort ↑↓ by name/title/branch/subject"},
			{"a / A", "cycle by repository  ·  sort ↑↓ by repository  (tabs 2 3 4)"},
			{"f / F", "cycle by date  ·  sort ↑↓ by date/updated  (all tabs)"},
		}},
		{"Dashboard columns", [][2]string{
			{"PR / Br", "open PR count · branch count  (heat: blue→yellow→red)"},
			{"tab 3  ●", "branch has an open PR"},
			{"tab 3  ∈", "branch merged into default branch"},
		}},
		{"Actions", [][2]string{
			{"enter", "open detail (tab 1) · open diff (tab 4)"},
			{"o", "open PR in browser  (tabs 2 4)"},
			{"r", "refresh current tab"},
			{"R", "toggle auto-refresh on / off"},
			{"ctrl+f", "git fetch all repos"},
			{"p", "git pull current repo  (tab 1)"},
			{"!", "about / paths"},
		}},
		{"Panes", [][2]string{
			{"j / k  (in detail/diff)", "navigate to next / previous item"},
			{"g  (single, 400 ms)", "toggle grouped / flat view"},
			{"?", "toggle this help"},
		}},
	}

	keyW := 26
	var lines []string
	for _, s := range sections {
		lines = append(lines, "")
		lines = append(lines, HeaderStyle.Render(s.title))
		for _, r := range s.rows {
			key := cell(DimStyle.Render(r[0]), keyW)
			lines = append(lines, "  "+key+r[1])
		}
	}
	lines = append(lines, "")

	boxW := min(width-4, 72)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 2).
		Width(boxW)

	inner := strings.Join(lines, "\n")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box.Render(inner))
}

// RenderInfoBar renders the context-aware bottom info bar.
func RenderInfoBar(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return DimStyle.Render("  " + strings.Join(parts, "   ·   "))
}

// ── Diff view ─────────────────────────────────────────────────────────────

// RenderDiff renders a coloured git-show patch, clipped to maxLines.
func RenderDiff(repoName, hash, content string, maxLines int) string {
	var b strings.Builder
	b.WriteString(HeaderStyle.Render(fmt.Sprintf("  %s  %s", repoName, hash)))
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  "+strings.Repeat("─", 86)))
	b.WriteString("\n")

	lines := strings.Split(content, "\n")
	shown := 0
	for _, line := range lines {
		if shown >= maxLines-2 { // -2 for the two header lines above
			b.WriteString(DimStyle.Render(fmt.Sprintf("  … %d more lines", len(lines)-shown)))
			b.WriteString("\n")
			break
		}
		b.WriteString(colorDiffLine(line))
		b.WriteString("\n")
		shown++
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
