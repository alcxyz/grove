package ui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette
const (
	catMauve    = "#cba6f7"
	catRed      = "#f38ba8"
	catPeach    = "#fab387"
	catYellow   = "#f9e2af"
	catGreen    = "#a6e3a1"
	catTeal     = "#94e2d5"
	catSky      = "#89dceb"
	catSapphire = "#74c7ec"
	catBlue     = "#89b4fa"
	catText     = "#cdd6f4"
	catSubtext1 = "#bac2de"
	catOverlay1 = "#7f849c"
	catOverlay0 = "#6c7086"
	catSurface1 = "#45475a"
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(catMauve)).
			PaddingLeft(1)

	TabStyle = lipgloss.NewStyle().
			Padding(0, 2)

	ActiveTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color(catMauve)).
			Underline(true)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catOverlay0)).
			PaddingLeft(1)

	DirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(catRed))
	CleanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(catGreen))
	AheadStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(catPeach))
	BehindStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(catBlue))
	HeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(catText))
	DimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(catOverlay1))
	SelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(catMauve))
	PassStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(catGreen))
	FailStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(catRed))
	PendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(catYellow))
	ApprovedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catGreen))
	ChangesStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(catRed))
	ReviewStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(catBlue))

	// Full-row selection: text on a raised surface
	SelectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(catText)).
				Background(lipgloss.Color(catSurface1))

	// Group header separator
	GroupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(catMauve)).
				PaddingLeft(1)

	// Diff view
	DiffAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(catGreen))
	DiffRemStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(catRed))
	DiffHunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catSapphire))
	DiffFileStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(catMauve))

	// Cycle filter / sort indicators
	CycleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(catPeach))
	SortAscStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catSky))
	SortDscStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(catTeal))

	// Block-jump highlight: applied to the column value that defines the current jump block
	BlockMatchStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(catTeal))
)
