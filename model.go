package main

import (
	"time"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/model"
)

type tab int

const (
	tabDashboard tab = iota
	tabPRs
	tabBranches
	tabActivity
)

var tabNames = []string{"Dashboard [1]", "Pull Requests [2]", "Branches [3]", "Activity [4]"}

type sortOrder int

const (
	sortDefault sortOrder = iota
	sortAsc
	sortDesc
)

// tabSortState holds the active sort for one tab: which field and which direction.
// Field "" means no sort override (use native order).
type tabSortState struct {
	Field string    // "date" | "author" | "subject" | ""
	Order sortOrder // sortAsc | sortDesc (sortDefault == no sort, Field should also be "")
}

type appModel struct {
	cfg      config.Config
	repos    []model.Repo
	prs      []model.PR
	branches []model.BranchInfo
	activity []model.Commit

	activeTab tab
	cursor    int
	width     int
	height    int
	loading   bool
	statusMsg string

	// Vim-style navigation
	prevKey      string
	scrollOffset map[tab]int

	// Cache timestamps — avoid unnecessary refetches on tab switch
	prsLoadedAt      time.Time
	branchesLoadedAt time.Time
	activityLoadedAt time.Time

	// Load errors — shown in view when a tab has no data
	errLog  []string
	authErr bool   // true when any gh call returned ErrNotLoggedIn
	logPath string // path of the runtime log file, shown in the help bar

	// Filter
	filtering   bool
	filterQuery string

	// Cycle quick-filter (d=author, s=subject, a=repo, f=date)
	cycleField  string   // active field; "" = none
	cycleValues []string // sorted unique values for current field
	cycleIdx    int      // index into cycleValues; -1 = none active

	// Per-tab sort: field ("date"|"author"|"subject"|"") + direction
	tabSort map[tab]tabSortState

	// Detail pane
	showDetail     bool
	detailCommits  []model.Commit
	detailPRs      []model.PR
	detailBranches []string
	detailStats    model.RepoStats

	// Diff view (tab 4 enter)
	showDiff    bool
	diffContent string
	diffRepo    string
	diffHash    string

	// Help overlay
	showHelp bool

	// Splash/about overlay (! key)
	showSplash bool

	// Grouping
	grouped bool

	// Block-jump highlight: "repo" or "subject", set by [ ] ( ) keys
	highlightField string

	// Mouse state
	lastClickY  int
	lastClickAt time.Time

	// Screensaver
	lastActivity time.Time
	ssActive     bool
	ssX, ssY     int
	ssDX, ssDY   int
	ssColor      int

	// Auto-refresh (R toggles)
	autoRefresh bool

	// Cache directory and config-derived key for invalidation
	cacheDir string
	cacheKey string
}

// Messages
type reposLoadedMsg struct{ repos []model.Repo }
type prsLoadedMsg struct {
	prs    []model.PR
	errors []string
}
type branchesLoadedMsg struct {
	branches []model.BranchInfo
	errors   []string
}
type activityLoadedMsg struct{ commits []model.Commit }
type fetchDoneMsg struct{ msg string }
type statusMsg string
type tickMsg time.Time

type detailLoadedMsg struct {
	commits  []model.Commit
	prs      []model.PR
	branches []string
	stats    model.RepoStats
}

type diffLoadedMsg struct {
	content string
	repo    string
	hash    string
}

type gTimeoutMsg struct{}
type ssTickMsg struct{}   // screensaver animation frame
type idleCheckMsg struct{} // periodic idle-time check
