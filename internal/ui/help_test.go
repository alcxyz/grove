package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestWrapHelpLineWrapsLongDescriptions(t *testing.T) {
	desc := "cycle · sort  branch count / branch / merged / due  (tabs 1 3 4 7)"
	out := stripANSI(wrapHelpLine("c / C", desc, 22, 62))
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected long help line to wrap, got: %q", out)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > 62 {
			t.Fatalf("wrapped line width = %d, want <= 62: %q", got, line)
		}
	}
	if !strings.HasPrefix(lines[1], strings.Repeat(" ", 24)) {
		t.Fatalf("continuation line is not hanging-indented: %q", lines[1])
	}
}

func TestRenderHelpDoesNotOverflowTerminalWidth(t *testing.T) {
	for _, width := range []int{60, 80, 100} {
		out := RenderHelp(width, 1, "test")
		for _, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("RenderHelp(%d) line width = %d, want <= %d: %q", width, got, width, stripANSI(line))
			}
		}
	}
}
