package app

import (
	"time"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/forge"
	"github.com/alcxyz/grove/internal/model"
)

type tab int

const (
	tabDashboard tab = iota
	tabPRs
	tabCI
	tabBranches
	tabActivity
	tabIssues
)

var tabNames = []string{"1 Dashboard", "2 Pull Requests", "3 CI Runs", "4 Branches", "5 Activity", "6 Issues"}

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

// Options holds the parameters needed to create a new Model.
type Options struct {
	Cfg              config.Config
	Version          string
	StatusMsg        string
	LogPath          string
	CacheDir         string
	CacheKey         string
	PRs              []model.PR
	PRsLoadedAt      time.Time
	Branches         []model.BranchInfo
	BranchesLoadedAt time.Time
	Activity         []model.Commit
	ActivityLoadedAt time.Time
	Runs             []model.WorkflowRun
	RunsLoadedAt     time.Time
	Issues           []model.Issue
	IssuesLoadedAt   time.Time
	ActiveProfile    int
	Providers        map[string]forge.Provider
}

// New creates a Model ready to be passed to tea.NewProgram.
func New(o Options) Model {
	return Model{
		cfg:              o.Cfg,
		version:          o.Version,
		statusMsg:        o.StatusMsg,
		loading:          true,
		scrollOffset:     map[tab]int{},
		tabSort:          map[tab]tabSortState{},
		cycleIdx:         -1,
		logPath:          o.LogPath,
		grouped:          true,
		autoRefresh:      true,
		lastActivity:     time.Now(),
		ssDX:             1,
		ssDY:             1,
		cacheDir:         o.CacheDir,
		cacheKey:         o.CacheKey,
		prs:              o.PRs,
		prsLoadedAt:      o.PRsLoadedAt,
		branches:         o.Branches,
		branchesLoadedAt: o.BranchesLoadedAt,
		activity:         o.Activity,
		activityLoadedAt: o.ActivityLoadedAt,
		runs:             o.Runs,
		runsLoadedAt:     o.RunsLoadedAt,
		issues:           o.Issues,
		issuesLoadedAt:   o.IssuesLoadedAt,
		activeProfile:    o.ActiveProfile,
		providers:        o.Providers,
	}
}

type Model struct {
	cfg      config.Config
	version  string
	repos    []model.Repo
	prs      []model.PR
	branches []model.BranchInfo
	activity []model.Commit
	runs     []model.WorkflowRun
	issues   []model.Issue

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
	runsLoadedAt     time.Time
	issuesLoadedAt   time.Time

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
	detailRepo     model.Repo // the repo currently shown in detail
	detailCommits  []model.Commit
	detailPRs      []model.PR
	detailIssues   []model.Issue
	detailBranches []string
	detailStats    model.RepoStats
	detailScroll   int          // scroll offset within the detail content
	detailCursor   int          // flat index into detailItems
	detailItems    []detailItem // selectable items with section/line info

	// Diff view (tab 4 enter)
	showDiff       bool
	diffContent    string
	diffRepo       string
	diffHash       string
	diffPreColored bool // true when content is already ANSI-colored (e.g. via delta)
	diffScroll     int  // scroll offset within the diff content

	// Help overlay (two pages, cycled with tab)
	showHelp bool
	helpPage int

	// Splash/about overlay (! key) with blink animation
	showSplash  bool
	splashBlink int // 0=both open 1=left closed 2=right closed 3=both closed

	// Profile switching: index into cfg.Profiles, or -1 for "All"
	activeProfile int

	// Grouping
	grouped bool

	// Block-jump highlight: "repo" or "subject", set by [ ] ( ) keys
	highlightField string

	// Mouse state
	lastClickY  int
	lastClickAt time.Time

	// Tab streak for exponential scroll (tab / shift+tab)
	lastTabAt time.Time
	tabStreak int // 0-3 → jump distances [5, 10, 20, 25]

	// Screensaver
	lastActivity time.Time
	ssActive     bool
	ssX, ssY     int
	ssDX, ssDY   int
	ssColor      int

	// Auto-refresh (R toggles)
	autoRefresh bool

	// Version update check
	latestVersion string // non-empty when a newer release is available

	// Cache directory and config-derived key for invalidation
	cacheDir string
	cacheKey string

	// Per-profile forge providers
	providers map[string]forge.Provider
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
	issues   []model.Issue
	branches []string
	stats    model.RepoStats
}

type diffLoadedMsg struct {
	content    string
	repo       string
	hash       string
	preColored bool
}

type runsLoadedMsg struct {
	runs   []model.WorkflowRun
	errors []string
}

type issuesLoadedMsg struct {
	issues []model.Issue
	errors []string
}

// detailSect identifies which section of the detail pane an item belongs to.
type detailSect int

const (
	detailLocalBranch detailSect = iota
	detailRemoteBranch
	detailPR
	detailIssue
	detailCIRun
	detailCommit
)

// detailItem maps a selectable item in the detail pane to its section, data
// index within that section's slice, and rendered line number.
type detailItem struct {
	Section detailSect
	Index   int // index into the relevant data slice
	Line    int // 0-indexed rendered line number
}

type versionCheckMsg struct{ latest string }
type gTimeoutMsg struct{}
type ssTickMsg struct{}                // screensaver animation frame
type idleCheckMsg struct{}             // periodic idle-time check
type splashBlinkMsg struct{ next int } // next blink state
