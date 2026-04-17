package app

import (
	"strings"
	"testing"

	"github.com/alcxyz/grove/internal/model"
)

// TestViewEmptyModel verifies that View() does not panic with a fresh model.
func TestViewEmptyModel(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40

	out := m.View()
	if out == "" {
		t.Error("View() should produce output")
	}
	if !strings.Contains(out, "grove") {
		t.Error("View() should contain the title")
	}
}

// TestViewWithRepos verifies View() renders the dashboard with repo data.
func TestViewWithRepos(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.loading = false
	m.repos = []model.Repo{
		{Name: "alpha", Path: "/tmp/alpha", Branch: "main", Profile: "test"},
		{Name: "bravo", Path: "/tmp/bravo", Branch: "develop", Dirty: true, Profile: "test"},
	}

	out := m.View()
	if !strings.Contains(out, "alpha") {
		t.Error("View() should contain repo name 'alpha'")
	}
}

// TestViewAllTabs verifies that View() works on every tab without panicking.
func TestViewAllTabs(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.loading = false
	m.repos = []model.Repo{{Name: "repo", Path: "/tmp/repo", Branch: "main", Owner: "org"}}
	m.prs = []model.PR{{Repo: "org/repo", Title: "Fix", Author: "dev"}}
	m.branches = []model.BranchInfo{{Repo: "org/repo", Name: "feat", Author: "dev"}}
	m.activity = []model.Commit{{Repo: "repo", Subject: "init", Author: "dev"}}
	m.runs = []model.WorkflowRun{{Repo: "org/repo", WorkflowName: "CI", Status: "completed", Conclusion: "success"}}

	tabs := []tab{tabDashboard, tabPRs, tabCI, tabBranches, tabActivity}
	for _, tb := range tabs {
		m.activeTab = tb
		out := m.View()
		if out == "" {
			t.Errorf("View() on tab %d should produce output", tb)
		}
	}
}

// TestViewWithFilter verifies View() renders the filter indicator.
func TestViewWithFilter(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.loading = false
	m.filterQuery = "test"

	out := m.View()
	if !strings.Contains(out, "test") {
		t.Error("View() should show the active filter query")
	}
}

// TestViewHelpOverlay verifies the help overlay renders.
func TestViewHelpOverlay(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.showHelp = true

	out := m.View()
	if !strings.Contains(out, "help") || !strings.Contains(out, "close") {
		t.Error("View() with showHelp should render help content")
	}
}

// TestViewSplashOverlay verifies the splash overlay renders.
func TestViewSplashOverlay(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.showSplash = true

	out := m.View()
	if out == "" {
		t.Error("View() with showSplash should produce output")
	}
}

// TestViewScreensaver verifies the screensaver renders.
func TestViewScreensaver(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.ssActive = true
	m.ssX = 10
	m.ssY = 5

	out := m.View()
	if out == "" {
		t.Error("View() with screensaver should produce output")
	}
}

// TestViewVersionInFooter verifies the version appears in the footer.
func TestViewVersionInFooter(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.loading = false

	out := m.View()
	if !strings.Contains(out, "v0.1.0") {
		t.Error("View() should show version in footer")
	}
}

// TestViewUpdateAvailable verifies the update notification appears.
func TestViewUpdateAvailable(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 40
	m.loading = false
	m.latestVersion = "v0.2.0"

	out := m.View()
	if !strings.Contains(out, "v0.2.0") {
		t.Error("View() should show available update version")
	}
}
