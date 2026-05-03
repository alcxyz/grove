package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/model"
)

// newTestModel returns a Model with sensible defaults for testing.
func newTestModel() Model {
	return New(Options{
		Cfg: config.Config{
			Profiles:    []config.Profile{{Name: "test", Owner: "org", BasePaths: []string{"/tmp"}}},
			RefreshSecs: 300,
		},
		Version:   "0.1.0",
		StatusMsg: "testing",
		LogPath:   "/tmp/grove.log",
		CacheDir:  "/tmp/grove-cache",
		CacheKey:  "test-key",
	})
}

// sendKey feeds a key press through Update and returns the resulting model.
func sendKey(m Model, key string) Model {
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return result.(Model)
}

// sendSpecialKey feeds a special key (esc, enter, ctrl+c, etc.) through Update.
func sendSpecialKey(m Model, kt tea.KeyType) (Model, tea.Cmd) {
	result, cmd := m.Update(tea.KeyMsg{Type: kt})
	return result.(Model), cmd
}

// ── New constructor ──────────────────────────────────────────────────────

func TestNewDefaults(t *testing.T) {
	m := newTestModel()

	if !m.loading {
		t.Error("new model should be loading")
	}
	if !m.grouped {
		t.Error("new model should default to grouped view")
	}
	if !m.autoRefresh {
		t.Error("new model should default to auto-refresh on")
	}
	if m.cycleIdx != -1 {
		t.Errorf("cycleIdx should be -1, got %d", m.cycleIdx)
	}
	if m.version != "0.1.0" {
		t.Errorf("version should be 0.1.0, got %q", m.version)
	}
	if m.activeTab != tabDashboard {
		t.Error("new model should start on dashboard tab")
	}
}

// ── Tab switching ────────────────────────────────────────────────────────

func TestUpdateTabSwitchByNumber(t *testing.T) {
	m := newTestModel()
	m.loading = false

	tests := []struct {
		key  string
		want tab
	}{
		{"2", tabPRs},
		{"3", tabCI},
		{"4", tabBranches},
		{"5", tabActivity},
		{"1", tabDashboard},
	}
	for _, tt := range tests {
		m = sendKey(m, tt.key)
		if m.activeTab != tt.want {
			t.Errorf("key %q: activeTab = %d, want %d", tt.key, m.activeTab, tt.want)
		}
		if m.cursor != 0 {
			t.Errorf("key %q: cursor should reset to 0, got %d", tt.key, m.cursor)
		}
	}
}

func TestUpdateTabSwitchHL(t *testing.T) {
	m := newTestModel()
	m.loading = false

	// l moves forward
	m = sendKey(m, "l")
	if m.activeTab != tabPRs {
		t.Errorf("l from dashboard: activeTab = %d, want %d", m.activeTab, tabPRs)
	}

	// h moves backward
	m = sendKey(m, "h")
	if m.activeTab != tabDashboard {
		t.Errorf("h from PRs: activeTab = %d, want %d", m.activeTab, tabDashboard)
	}

	// h wraps around
	m = sendKey(m, "h")
	if m.activeTab != tabIssues {
		t.Errorf("h from dashboard: activeTab = %d, want %d (should wrap)", m.activeTab, tabIssues)
	}
}

// ── Cursor movement ─────────────────────────────────────────────────────

func TestUpdateCursorMovement(t *testing.T) {
	m := newTestModel()
	m.loading = false
	m.height = 40
	m.repos = make([]model.Repo, 10)
	for i := range m.repos {
		m.repos[i] = model.Repo{Name: "repo", Path: "/tmp/repo", Profile: "test"}
	}

	// j moves down
	m = sendKey(m, "j")
	if m.cursor != 1 {
		t.Errorf("j: cursor = %d, want 1", m.cursor)
	}

	// k moves up
	m = sendKey(m, "k")
	if m.cursor != 0 {
		t.Errorf("k: cursor = %d, want 0", m.cursor)
	}

	// k at top stays at 0
	m = sendKey(m, "k")
	if m.cursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0", m.cursor)
	}

	// G goes to last item
	m = sendKey(m, "G")
	if m.cursor != 9 {
		t.Errorf("G: cursor = %d, want 9", m.cursor)
	}

	// j at bottom clamps to last item
	m = sendKey(m, "j")
	if m.cursor != 9 {
		t.Errorf("j at bottom: cursor = %d, want 9", m.cursor)
	}
}

// ── Filter mode ─────────────────────────────────────────────────────────

func TestUpdateFilterMode(t *testing.T) {
	m := newTestModel()
	m.loading = false

	// / enters filter mode
	m = sendKey(m, "/")
	if !m.filtering {
		t.Error("/ should enter filter mode")
	}
	if m.filterQuery != "" {
		t.Error("filter query should start empty")
	}

	// typing in filter mode appends to query
	m = sendKey(m, "f")
	m = sendKey(m, "o")
	m = sendKey(m, "o")
	if m.filterQuery != "foo" {
		t.Errorf("filter query = %q, want %q", m.filterQuery, "foo")
	}
	if !m.filtering {
		t.Error("should still be in filter mode")
	}

	// enter confirms filter
	m, _ = sendSpecialKey(m, tea.KeyEnter)
	if m.filtering {
		t.Error("enter should exit filter mode")
	}
	if m.filterQuery != "foo" {
		t.Errorf("filter query should be preserved after enter, got %q", m.filterQuery)
	}

	// esc clears filter
	m, _ = sendSpecialKey(m, tea.KeyEscape)
	if m.filterQuery != "" {
		t.Errorf("esc should clear filter query, got %q", m.filterQuery)
	}
}

func TestUpdateFilterBackspace(t *testing.T) {
	m := newTestModel()
	m.loading = false

	m = sendKey(m, "/")
	m = sendKey(m, "a")
	m = sendKey(m, "b")

	// backspace removes last character
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = result.(Model)

	if m.filterQuery != "a" {
		t.Errorf("backspace: query = %q, want %q", m.filterQuery, "a")
	}
}

func TestUpdateFilterEscInFilterMode(t *testing.T) {
	m := newTestModel()
	m.loading = false

	m = sendKey(m, "/")
	m = sendKey(m, "x")

	// esc in filter mode clears and exits
	m, _ = sendSpecialKey(m, tea.KeyEscape)
	if m.filtering {
		t.Error("esc should exit filter mode")
	}
	if m.filterQuery != "" {
		t.Errorf("esc in filter mode should clear query, got %q", m.filterQuery)
	}
}

// ── Quit ─────────────────────────────────────────────────────────────────

func TestUpdateQuit(t *testing.T) {
	m := newTestModel()
	m.loading = false

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a command")
	}
	// tea.Quit returns a special message; verify the command is non-nil.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c should produce QuitMsg, got %T", msg)
	}
}

func TestUpdateQuitQ(t *testing.T) {
	m := newTestModel()
	m.loading = false

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q should produce QuitMsg, got %T", msg)
	}
}

// ── Message handling ────────────────────────────────────────────────────

func TestUpdateReposLoaded(t *testing.T) {
	m := newTestModel()

	repos := []model.Repo{
		{Name: "alpha", Path: "/tmp/alpha"},
		{Name: "bravo", Path: "/tmp/bravo"},
	}
	result, _ := m.Update(reposLoadedMsg{repos: repos})
	m = result.(Model)

	if len(m.repos) != 2 {
		t.Errorf("repos count = %d, want 2", len(m.repos))
	}
	if m.loading {
		t.Error("loading should be false after repos loaded")
	}
}

func TestUpdatePRsLoaded(t *testing.T) {
	m := newTestModel()

	prs := []model.PR{
		{Repo: "org/alpha", Title: "Fix bug", Author: "dev"},
		{Repo: "org/bravo", Title: "Add feature", Author: "dev"},
		{Repo: "org/bravo", Title: "Update docs", Author: "dev"},
	}
	result, _ := m.Update(prsLoadedMsg{prs: prs})
	m = result.(Model)

	if len(m.prs) != 3 {
		t.Errorf("prs count = %d, want 3", len(m.prs))
	}
	if m.loading {
		t.Error("loading should be false after PRs loaded")
	}
	if m.prsLoadedAt.IsZero() {
		t.Error("prsLoadedAt should be set")
	}
}

func TestUpdateBranchesLoadedWithErrors(t *testing.T) {
	m := newTestModel()

	branches := []model.BranchInfo{{Repo: "org/repo", Name: "main"}}
	errs := []string{"repo-x: connection refused"}
	result, _ := m.Update(branchesLoadedMsg{branches: branches, errors: errs})
	m = result.(Model)

	if len(m.branches) != 1 {
		t.Errorf("branches count = %d, want 1", len(m.branches))
	}
	if len(m.errLog) != 1 {
		t.Errorf("errLog count = %d, want 1", len(m.errLog))
	}
}

// ── Window resize ───────────────────────────────────────────────────────

func TestUpdateWindowResize(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = result.(Model)

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

// ── Grouped toggle ──────────────────────────────────────────────────────

func TestUpdateGroupedToggle(t *testing.T) {
	m := newTestModel()
	m.loading = false

	if !m.grouped {
		t.Fatal("should start grouped")
	}

	// Simulate the gTimeoutMsg that fires after a single 'g' press
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = result.(Model)
	// Simulate timeout firing
	result, _ = m.Update(gTimeoutMsg{})
	m = result.(Model)

	if m.grouped {
		t.Error("single g + timeout should toggle grouped off")
	}
}

// ── Auto-refresh toggle ─────────────────────────────────────────────────

func TestUpdateAutoRefreshToggle(t *testing.T) {
	m := newTestModel()
	m.loading = false

	if !m.autoRefresh {
		t.Fatal("should start with auto-refresh on")
	}

	m = sendKey(m, "R")
	if m.autoRefresh {
		t.Error("R should toggle auto-refresh off")
	}

	m = sendKey(m, "R")
	if !m.autoRefresh {
		t.Error("R should toggle auto-refresh back on")
	}
}

// ── Sort cycling ─────────────────────────────────────────────────────────

func TestUpdateSortCycle(t *testing.T) {
	m := newTestModel()
	m.loading = false

	// F cycles date sort: off → asc → desc → off
	m = sendKey(m, "F")
	ts := m.tabSort[m.activeTab]
	if ts.Field != "date" || ts.Order != sortAsc {
		t.Errorf("first F: field=%q order=%d, want date/asc", ts.Field, ts.Order)
	}

	m = sendKey(m, "F")
	ts = m.tabSort[m.activeTab]
	if ts.Field != "date" || ts.Order != sortDesc {
		t.Errorf("second F: field=%q order=%d, want date/desc", ts.Field, ts.Order)
	}

	m = sendKey(m, "F")
	ts = m.tabSort[m.activeTab]
	if ts.Field != "" {
		t.Errorf("third F: field=%q, want empty (cleared)", ts.Field)
	}
}

// ── Help overlay ─────────────────────────────────────────────────────────

func TestUpdateHelpOverlay(t *testing.T) {
	m := newTestModel()
	m.loading = false

	m = sendKey(m, "?")
	if !m.showHelp {
		t.Error("? should open help")
	}

	m = sendKey(m, "?")
	if m.showHelp {
		t.Error("? again should close help")
	}
}

func TestUpdateConfigPreviewOverlay(t *testing.T) {
	m := newTestModel()
	m.loading = false

	m = sendKey(m, ",")
	if !m.showConfigPreview {
		t.Error(", should open config preview")
	}
	if !strings.Contains(m.configPreview, "Profile: test") {
		t.Errorf("config preview should include active profile, got %q", m.configPreview)
	}

	m = sendKey(m, ",")
	if m.showConfigPreview {
		t.Error(", should close config preview")
	}
}

func TestUpdateDetailConfigPreviewUsesRepo(t *testing.T) {
	m := newTestModel()
	m.loading = false
	m.showDetail = true
	m.detailRepo = model.Repo{Name: "repo", Path: "/tmp/repo", Profile: "test"}

	m = sendKey(m, ",")
	if !m.showConfigPreview {
		t.Error(", should open config preview from detail")
	}
	if !strings.Contains(m.configPreview, "Config preview: repo") {
		t.Errorf("detail config preview should include repo name, got %q", m.configPreview)
	}
	if !strings.Contains(m.configPreview, "Resolved remotes") {
		t.Errorf("detail config preview should include resolved remotes, got %q", m.configPreview)
	}
}

// ── Splash overlay ───────────────────────────────────────────────────────

func TestUpdateSplashOverlay(t *testing.T) {
	m := newTestModel()
	m.loading = false

	m = sendKey(m, "!")
	if !m.showSplash {
		t.Error("! should open splash")
	}

	// Any key dismisses splash
	m = sendKey(m, "x")
	if m.showSplash {
		t.Error("key should dismiss splash")
	}
}

// ── Screensaver dismissal ────────────────────────────────────────────────

func TestUpdateScreensaverDismiss(t *testing.T) {
	m := newTestModel()
	m.ssActive = true

	m = sendKey(m, "j")
	if m.ssActive {
		t.Error("any key should dismiss screensaver")
	}
}

// ── Idle check triggers screensaver ──────────────────────────────────────

func TestUpdateIdleCheckTriggersScreensaver(t *testing.T) {
	m := newTestModel()
	m.cfg.ScreensaverSecs = 60
	m.width = 120
	m.height = 40
	m.lastActivity = time.Now().Add(-2 * time.Minute) // idle for 2 min

	result, _ := m.Update(idleCheckMsg{})
	m = result.(Model)

	if !m.ssActive {
		t.Error("idle check should activate screensaver when idle > threshold")
	}
}

// ── Version check ────────────────────────────────────────────────────────

func TestUpdateVersionCheck(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(versionCheckMsg{latest: "v0.2.0"})
	m = result.(Model)

	if m.latestVersion != "v0.2.0" {
		t.Errorf("latestVersion = %q, want %q", m.latestVersion, "v0.2.0")
	}
}
