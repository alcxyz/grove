package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/grove/internal/cache"
	"github.com/alcxyz/grove/internal/gh"
	gitpkg "github.com/alcxyz/grove/internal/git"
	"github.com/alcxyz/grove/internal/ui"
)

// saveState persists minimal UI state (active profile) for next launch.
func (m appModel) saveState() {
	_ = cache.SaveState(m.cacheDir, cache.UIState{ActiveProfile: m.activeProfile})
}

// containsAuthErr returns true if any error string matches the gh auth sentinel.
func containsAuthErr(errs []string) bool {
	needle := gh.ErrNotLoggedIn.Error()
	for _, e := range errs {
		if strings.Contains(e, needle) {
			return true
		}
	}
	return false
}

func (m appModel) Init() tea.Cmd {
	ttl := time.Duration(m.cfg.RefreshSecs) * time.Second
	cmds := []tea.Cmd{loadRepos(m.cfg.Profiles), checkLatestVersion(), splashBlinkCmd(0)}

	// Background-refresh any cached data that is stale
	if len(m.prs) == 0 || time.Since(m.prsLoadedAt) > ttl {
		cmds = append(cmds, loadPRs(m.cfg.Profiles))
	}
	if len(m.branches) == 0 || time.Since(m.branchesLoadedAt) > ttl {
		cmds = append(cmds, loadBranches(m.cfg.Profiles))
	}
	if len(m.activity) == 0 || time.Since(m.activityLoadedAt) > ttl {
		cmds = append(cmds, loadActivity(m.cfg.Profiles))
	}
	if len(m.runs) == 0 || time.Since(m.runsLoadedAt) > ttl {
		cmds = append(cmds, loadRuns(m.cfg.Profiles))
	}

	if m.autoRefresh {
		cmds = append(cmds, tickCmd(ttl))
	}
	if m.cfg.ScreensaverSecs > 0 {
		cmds = append(cmds, idleCheckCmd())
	}
	return tea.Batch(cmds...)
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		key := msg.String()

		// ctrl+c always quits, regardless of mode/overlay.
		if key == "ctrl+c" {
			m.saveState()
			return m, tea.Quit
		}

		// Any key dismisses screensaver
		if m.ssActive {
			m.ssActive = false
			m.lastActivity = time.Now()
			return m, nil
		}
		m.lastActivity = time.Now()

		// Splash overlay
		if m.showSplash {
			if key == "tab" {
				// tab in the about screen launches the screensaver
				m.showSplash = false
				m.ssActive = true
				m.ssX = max(0, (m.width-ui.SplashArtWidth)/2)
				m.ssY = max(0, (m.height-ui.SplashArtHeight)/2)
				m.ssDX, m.ssDY = 1, 1
				m.ssColor = 0
				return m, tea.Batch(idleCheckCmd(), ssTickCmd())
			}
			m.showSplash = false
			return m, nil
		}
		if key == "!" && !m.filtering {
			m.showSplash = true
			return m, nil
		}

		// Help overlay — tab/shift+tab cycle pages, anything else closes
		if m.showHelp {
			switch key {
			case "tab":
				m.helpPage = (m.helpPage + 1) % 2
			case "shift+tab":
				m.helpPage = (m.helpPage + 1) % 2 // only 2 pages so same as +1
			case "?", "esc", "q":
				m.showHelp = false
				m.helpPage = 0
			}
			return m, nil
		}
		if key == "?" && !m.filtering {
			m.showHelp = true
			m.helpPage = 0
			return m, nil
		}

		// Filter mode input
		if m.filtering {
			switch key {
			case "esc":
				m.filtering = false
				m.filterQuery = ""
				m.cursor = 0
			case "enter":
				m.filtering = false
			case "backspace":
				if len(m.filterQuery) > 0 {
					m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
					m.cursor = 0
				}
			default:
				if len(key) == 1 {
					m.filterQuery += key
					m.cursor = 0
				}
			}
			m.clampCursor()
			return m, nil
		}

		// Diff view input
		if m.showDiff {
			isTabNav := key == "tab" || key == "shift+tab" || key == "1" || key == "2" || key == "3" || key == "4" || key == "5"
			if !isTabNav {
				// loadCommitAt fetches the diff for cursor c (grouped-order index)
				// and resets diffScroll so the new diff starts at the top.
				loadCommitAt := func(c int) (appModel, tea.Cmd) {
					m.cursor = c
					if cm, ok := m.commitAtCursor(); ok {
						m.diffScroll = 0
						m.loading = true
						m.statusMsg = fmt.Sprintf("Loading diff %s…", cm.Hash)
						return m, loadDiff(cm.RepoPath, cm.Repo, cm.Hash, m.width)
					}
					return m, nil
				}
				switch key {
				case "esc", "backspace", "q":
					m.showDiff = false
				case "o":
					if c, ok := m.commitAtCursor(); ok {
						if r, ok := m.repoByName(c.Repo); ok && r.Owner != "" {
							_ = ui.OpenURL(fmt.Sprintf("https://github.com/%s/%s/commit/%s", r.Owner, r.Name, c.Hash))
						}
					}
				case "e":
					if c, ok := m.commitAtCursor(); ok {
						return m, launchNvim(c.RepoPath)
					}
				case " ":
					if c, ok := m.commitAtCursor(); ok {
						return m, launchDiffnav(c.RepoPath, c.Hash)
					}
				case "j", "down":
					m.diffScroll++
					m.clampDiffScroll()
				case "k", "up":
					if m.diffScroll > 0 {
						m.diffScroll--
					}
				case "G":
					m.diffScroll = max(0, m.diffTotalLines()-m.contentHeight())
				case "g":
					if m.prevKey == "g" {
						m.prevKey = ""
						m.diffScroll = 0 // gg = scroll to top
					} else {
						m.prevKey = "g"
						return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
							return gTimeoutMsg{}
						})
					}
				case "[":
					if m.cursor > 0 {
						return loadCommitAt(m.cursor - 1)
					}
				case "]":
					if m.cursor < m.listLen()-1 {
						return loadCommitAt(m.cursor + 1)
					}
				case "{":
					// Jump to previous file header.
					starts := m.diffFileStarts()
					target := 0
					for i := len(starts) - 1; i >= 0; i-- {
						if starts[i] < m.diffScroll {
							target = starts[i]
							break
						}
					}
					m.diffScroll = target
				case "}":
					// Jump to next file header.
					for _, s := range m.diffFileStarts() {
						if s > m.diffScroll {
							m.diffScroll = s
							m.clampDiffScroll()
							break
						}
					}
				}
				return m, nil
			}
			m.showDiff = false
		}

		// Detail pane input
		if m.showDetail {
			isTabNav := key == "tab" || key == "shift+tab" || key == "1" || key == "2" || key == "3" || key == "4" || key == "5"
			if !isTabNav {
				// loadAt navigates to a different repo from within the detail pane.
				// Walks the grouped structure so that [/] respects the current grouping.
				loadAt := func(c int) (appModel, tea.Cmd) {
					flat := 0
					for _, g := range m.groupedRepos() {
						for _, r := range g.Repos {
							if flat == c {
								m.cursor = c
								m.detailScroll = 0
								m.loading = true
								m.statusMsg = fmt.Sprintf("Loading %s…", r.Name)
								return m, loadDetail(r)
							}
							flat++
						}
					}
					return m, nil
				}
				switch key {
				case "esc", "backspace", "q":
					m.showDetail = false
				case "o":
					// Contextual open-in-browser based on selected item.
					if m.detailCursor >= 0 && m.detailCursor < len(m.detailItems) {
						item := m.detailItems[m.detailCursor]
						repo, _ := m.repoAtCursor()
						switch item.Section {
						case detailRemoteBranch:
							branches := m.detailRemoteBranches()
							if item.Index < len(branches) && repo.Owner != "" {
								_ = ui.OpenURL(fmt.Sprintf("https://github.com/%s/%s/tree/%s", repo.Owner, repo.Name, branches[item.Index].Name))
							}
						case detailPR:
							if item.Index < len(m.detailPRs) {
								_ = ui.OpenURL(m.detailPRs[item.Index].URL)
							}
						case detailCIRun:
							runs := m.detailCIRuns()
							if item.Index < len(runs) {
								_ = ui.OpenURL(runs[item.Index].URL)
							}
						case detailCommit:
							if item.Index < len(m.detailCommits) && repo.Owner != "" {
								c := m.detailCommits[item.Index]
								_ = ui.OpenURL(fmt.Sprintf("https://github.com/%s/%s/commit/%s", repo.Owner, repo.Name, c.Hash))
							}
						default:
							// Local branches / fallback: open repo on GitHub.
							if repo.Owner != "" {
								_ = ui.OpenURL(fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name))
							}
						}
					}
				case "e":
					if path := m.repoPathAtCursor(); path != "" {
						return m, launchNvim(path)
					}
				case " ":
					// Contextual external tool based on selected item.
					if m.detailCursor >= 0 && m.detailCursor < len(m.detailItems) {
						item := m.detailItems[m.detailCursor]
						switch item.Section {
						case detailCommit:
							if item.Index < len(m.detailCommits) {
								c := m.detailCommits[item.Index]
								return m, launchDiffnav(c.RepoPath, c.Hash)
							}
						default:
							if path := m.repoPathAtCursor(); path != "" {
								return m, launchLazygit(path)
							}
						}
					}
				case "j", "down":
					if m.detailCursor < len(m.detailItems)-1 {
						m.detailCursor++
						m.adjustDetailScroll()
					}
				case "k", "up":
					if m.detailCursor > 0 {
						m.detailCursor--
						m.adjustDetailScroll()
					}
				case "G":
					if len(m.detailItems) > 0 {
						m.detailCursor = len(m.detailItems) - 1
						m.adjustDetailScroll()
					}
				case "g":
					if m.prevKey == "g" {
						m.prevKey = ""
						m.detailCursor = 0
						m.adjustDetailScroll()
					} else {
						m.prevKey = "g"
						return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
							return gTimeoutMsg{}
						})
					}
				case "[":
					if m.cursor > 0 {
						m.cursor--
					}
					return loadAt(m.cursor)
				case "]":
					if m.cursor < m.listLen()-1 {
						m.cursor++
					}
					return loadAt(m.cursor)
				case "{":
					m.detailJumpSection(-1)
				case "}":
					m.detailJumpSection(+1)
				}
				return m, nil
			}
			m.showDetail = false
		}

		// g / gg / G
		// Single g (after 400 ms timeout) → toggle grouping.
		// Double gg (second g before timeout) → go to first item.
		// G → go to last item.
		if key == "g" {
			if m.prevKey == "g" {
				// gg: go to top
				m.cursor = 0
				m.prevKey = ""
				m.adjustScroll()
				return m, nil
			}
			m.prevKey = "g"
			return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
				return gTimeoutMsg{}
			})
		}
		if key == "G" {
			m.cursor = max(0, m.listLen()-1)
			m.prevKey = ""
			m.adjustScroll()
			return m, nil
		}
		m.prevKey = "" // reset for any non-g key

		// Normal mode
		switch key {
		case "q":
			m.saveState()
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterQuery = ""
			return m, nil
		case "esc":
			if m.cycleField != "" {
				m.clearCycleFilter()
				m.cursor = 0
				m.adjustScroll()
			} else if m.filterQuery != "" {
				m.filterQuery = ""
				m.cursor = 0
				m.adjustScroll()
			} else {
				m.highlightField = ""
			}
		case "tab":
			m.activeTab = (m.activeTab + 1) % 5
			m.cursor = 0
			m.filterQuery = ""
			m.clearCycleFilter()
			m.scrollOffset[m.activeTab] = 0
			return m, m.loadTabIfNeeded()
		case "shift+tab":
			m.activeTab = (m.activeTab + 4) % 5
			m.cursor = 0
			m.filterQuery = ""
			m.clearCycleFilter()
			m.scrollOffset[m.activeTab] = 0
			return m, m.loadTabIfNeeded()
		case "1":
			m.activeTab = tabDashboard
			m.cursor = 0
			m.filterQuery = ""
			m.scrollOffset[tabDashboard] = 0
			m.clearCycleFilter()
		case "2":
			m.activeTab = tabPRs
			m.cursor = 0
			m.filterQuery = ""
			m.clearCycleFilter()
			ttl := time.Duration(m.cfg.RefreshSecs) * time.Second
			if len(m.prs) == 0 || time.Since(m.prsLoadedAt) > ttl {
				m.loading = true
				m.statusMsg = "Loading PRs..."
				return m, loadPRs(m.cfg.Profiles)
			}
		case "3":
			m.activeTab = tabCI
			m.cursor = 0
			m.filterQuery = ""
			m.clearCycleFilter()
			ttl := time.Duration(m.cfg.RefreshSecs) * time.Second
			if len(m.runs) == 0 || time.Since(m.runsLoadedAt) > ttl {
				m.loading = true
				m.statusMsg = "Loading CI runs..."
				return m, loadRuns(m.cfg.Profiles)
			}
		case "4":
			m.activeTab = tabBranches
			m.cursor = 0
			m.filterQuery = ""
			m.clearCycleFilter()
			ttl := time.Duration(m.cfg.RefreshSecs) * time.Second
			if len(m.branches) == 0 || time.Since(m.branchesLoadedAt) > ttl {
				m.loading = true
				m.statusMsg = "Loading branches..."
				return m, loadBranches(m.cfg.Profiles)
			}
		case "5":
			m.activeTab = tabActivity
			m.cursor = 0
			m.filterQuery = ""
			m.clearCycleFilter()
			ttl := time.Duration(m.cfg.RefreshSecs) * time.Second
			if len(m.activity) == 0 || time.Since(m.activityLoadedAt) > ttl {
				m.loading = true
				m.statusMsg = "Loading activity..."
				return m, loadActivity(m.cfg.Profiles)
			}
		case "enter":
			// enter = open in-app view: detail pane (tabs 1-4), diff (tab 5).
			switch m.activeTab {
			case tabDashboard:
				if r, ok := m.repoAtCursor(); ok {
					m.showDetail = true
					m.detailScroll = 0
					m.loading = true
					m.statusMsg = fmt.Sprintf("Loading %s details…", r.Name)
					return m, loadDetail(r)
				}
			case tabPRs:
				if pr, ok := m.prAtCursor(); ok {
					if repo, ok := m.repoByName(repoBaseName(pr.Repo)); ok {
						m.showDetail = true
						m.detailScroll = 0
						m.loading = true
						m.statusMsg = fmt.Sprintf("Loading %s details…", repo.Name)
						return m, loadDetail(repo)
					}
				}
			case tabBranches:
				if br, ok := m.branchAtCursor(); ok {
					if repo, ok := m.repoByName(repoBaseName(br.Repo)); ok {
						m.showDetail = true
						m.detailScroll = 0
						m.loading = true
						m.statusMsg = fmt.Sprintf("Loading %s details…", repo.Name)
						return m, loadDetail(repo)
					}
				}
			case tabActivity:
				if c, ok := m.commitAtCursor(); ok {
					m.showDiff = true
					m.diffContent = ""
					m.diffScroll = 0
					m.loading = true
					m.statusMsg = fmt.Sprintf("Loading diff %s…", c.Hash)
					return m, loadDiff(c.RepoPath, c.Repo, c.Hash, m.width)
				}
			case tabCI:
				if r, ok := m.runAtCursor(); ok {
					if repo, ok := m.repoByName(repoBaseName(r.Repo)); ok {
						m.showDetail = true
						m.detailScroll = 0
						m.loading = true
						m.statusMsg = fmt.Sprintf("Loading %s details…", repo.Name)
						return m, loadDetail(repo)
					}
				}
			}
		case "r":
			m.loading = true
			m.statusMsg = "Refreshing..."
			switch m.activeTab {
			case tabDashboard:
				return m, loadRepos(m.cfg.Profiles)
			case tabPRs:
				return m, loadPRs(m.cfg.Profiles)
			case tabBranches:
				return m, loadBranches(m.cfg.Profiles)
			case tabActivity:
				return m, loadActivity(m.cfg.Profiles)
			case tabCI:
				return m, loadRuns(m.cfg.Profiles)
			}
		case "R":
			m.autoRefresh = !m.autoRefresh
			if m.autoRefresh {
				m.statusMsg = "Auto-refresh on"
				return m, tickCmd(time.Duration(m.cfg.RefreshSecs) * time.Second)
			}
			m.statusMsg = "Auto-refresh off"
		// Sort cycle keys: F=date, S=subject/name, D=author, A=repo
		case "F":
			m.cycleSortField("date")
		case "S":
			m.cycleSortField("subject")
		case "D":
			m.cycleSortField("author")
		case "A":
			m.cycleSortField("repo")
		case "ctrl+f":
			m.loading = true
			m.statusMsg = "Fetching all repos..."
			return m, fetchAll(m.cfg.Profiles)
		case "<":
			if len(m.cfg.Profiles) > 1 {
				if m.activeProfile == 0 {
					m.activeProfile = -1
				} else if m.activeProfile == -1 {
					m.activeProfile = len(m.cfg.Profiles) - 1
				} else {
					m.activeProfile--
				}
				m.cursor = 0
				m.filterQuery = ""
				m.clearCycleFilter()
				for k := range m.scrollOffset {
					m.scrollOffset[k] = 0
				}
				go m.saveState()
			}
		case ">":
			if len(m.cfg.Profiles) > 1 {
				if m.activeProfile == len(m.cfg.Profiles)-1 {
					m.activeProfile = -1
				} else if m.activeProfile == -1 {
					m.activeProfile = 0
				} else {
					m.activeProfile++
				}
				m.cursor = 0
				m.filterQuery = ""
				m.clearCycleFilter()
				for k := range m.scrollOffset {
					m.scrollOffset[k] = 0
				}
				go m.saveState()
			}
		// Cycle quick filters
		case "d":
			m.doCycleFilter("author")
		case "s":
			m.doCycleFilter("subject")
		case "a":
			m.doCycleFilter("repo")
		case "f":
			m.doCycleFilter("date")
		// x/X — PR column: PR count (tab 1) · ReviewDecision (tab 2) · has-PR (tab 3)
		case "x":
			switch m.activeTab {
			case tabDashboard:
				m.doCycleFilter("prcount")
			case tabPRs:
				m.doCycleFilter("review")
			case tabBranches:
				m.doCycleFilter("prcount")
			}
		case "X":
			switch m.activeTab {
			case tabDashboard:
				m.cycleSortField("prcount")
			case tabPRs:
				m.cycleSortField("review")
			case tabBranches:
				m.cycleSortField("prcount")
			}
		// c/C — Br column: branch count (tab 1) · merged (tab 3) · branch prefix (tab 5)
		case "c":
			switch m.activeTab {
			case tabDashboard:
				m.doCycleFilter("brcount")
			case tabBranches:
				m.doCycleFilter("merged")
			case tabCI:
				m.doCycleFilter("branch")
			}
		case "C":
			switch m.activeTab {
			case tabDashboard:
				m.cycleSortField("brcount")
			case tabBranches:
				m.cycleSortField("merged")
			case tabCI:
				m.cycleSortField("branch")
			}
		// v/V — CI column: CI conclusion (tabs 1 5) · PR checks (tab 2)
		case "v":
			switch m.activeTab {
			case tabDashboard:
				m.doCycleFilter("ci")
			case tabPRs:
				m.doCycleFilter("checks")
			case tabCI:
				m.doCycleFilter("ci")
			}
		case "V":
			switch m.activeTab {
			case tabDashboard:
				m.cycleSortField("ci")
			case tabPRs:
				m.cycleSortField("checks")
			case tabCI:
				m.cycleSortField("ci")
			}
		case " ":
			// Diff/commit-centric tabs → diffnav; repo-centric tabs → lazygit.
			switch m.activeTab {
			case tabActivity:
				if c, ok := m.commitAtCursor(); ok {
					return m, launchDiffnav(c.RepoPath, c.Hash)
				}
			default:
				if path := m.repoPathAtCursor(); path != "" {
					return m, launchLazygit(path)
				}
			}
		case "e":
			// e = open editor (nvim / $EDITOR) at the repo root.
			if path := m.repoPathAtCursor(); path != "" {
				return m, launchNvim(path)
			}
		case "p":
			// p = git pull the repo for the selected item (all tabs).
			if path := m.repoPathAtCursor(); path != "" {
				name := repoBaseName(path)
				m.loading = true
				m.statusMsg = fmt.Sprintf("Pulling %s...", name)
				return m, func() tea.Msg {
					_, err := gitpkg.Pull(path)
					if err != nil {
						return statusMsg(fmt.Sprintf("Pull %s failed: %v", name, err))
					}
					return statusMsg(fmt.Sprintf("Pulled %s", name))
				}
			}
		case "o":
			// o = open on GitHub in browser.
			switch m.activeTab {
			case tabDashboard:
				if r, ok := m.repoAtCursor(); ok {
					if r.Owner != "" {
						_ = ui.OpenURL(fmt.Sprintf("https://github.com/%s/%s", r.Owner, r.Name))
					}
				}
			case tabPRs:
				if pr, ok := m.prAtCursor(); ok {
					_ = ui.OpenURL(pr.URL)
				}
			case tabBranches:
				if br, ok := m.branchAtCursor(); ok {
					_ = ui.OpenURL(fmt.Sprintf("https://github.com/%s/tree/%s", br.Repo, br.Name))
				}
			case tabActivity:
				if c, ok := m.commitAtCursor(); ok {
					if r, ok := m.repoByName(c.Repo); ok && r.Owner != "" {
						_ = ui.OpenURL(fmt.Sprintf("https://github.com/%s/%s/commit/%s", r.Owner, r.Name, c.Hash))
					}
				}
			case tabCI:
				if r, ok := m.runAtCursor(); ok {
					_ = ui.OpenURL(r.URL)
				}
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
		case "down", "j":
			m.cursor++
			m.clampCursor() // clampCursor calls adjustScroll
		case "{":
			m.jumpGroup(-1)
		case "}":
			m.jumpGroup(+1)
		case "[":
			m.highlightField = "repo"
			m.jumpRepo(-1)
		case "]":
			m.highlightField = "repo"
			m.jumpRepo(+1)
		case "(":
			m.highlightField = "subject"
			m.jumpSubject(-1)
		case ")":
			m.highlightField = "subject"
			m.jumpSubject(+1)
		}

	case tea.MouseMsg:
		m.lastActivity = time.Now()
		if m.ssActive {
			m.ssActive = false
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.showDiff {
				m.diffScroll = max(0, m.diffScroll-3)
			} else if m.showDetail {
				m.detailScroll = max(0, m.detailScroll-3)
			} else {
				so := m.scrollOffset[m.activeTab]
				if so >= 3 {
					so -= 3
				} else {
					so = 0
				}
				m.scrollOffset[m.activeTab] = so
			}
		case tea.MouseButtonWheelDown:
			if m.showDiff {
				m.diffScroll += 3
				m.clampDiffScroll()
			} else if m.showDetail {
				m.detailScroll += 3
				m.clampDetailScroll()
			} else {
				so := m.scrollOffset[m.activeTab] + 3
				if mso := m.maxScrollOffset(); so > mso {
					so = mso
				}
				m.scrollOffset[m.activeTab] = so
			}
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionPress {
				// Tab bar / profile bar click handling
				if m.showProfileBar() {
					if msg.Y == 2 {
						// Profile tab click
						if idx := m.profileAtX(msg.X); idx >= -1 {
							m.activeProfile = idx
							m.cursor = 0
							m.filterQuery = ""
							m.clearCycleFilter()
							for k := range m.scrollOffset {
								m.scrollOffset[k] = 0
							}
							go m.saveState()
							return m, nil
						}
					} else if msg.Y == 3 || msg.Y == 4 {
						// Content tab click (shifted down by 1 row due to profile bar)
						if t := tabAtX(msg.X); t >= 0 {
							m.showDiff = false
							m.showDetail = false
							m.showHelp = false
							m.showSplash = false
							m.activeTab = tab(t)
							m.cursor = 0
							m.filterQuery = ""
							m.clearCycleFilter()
							m.scrollOffset[m.activeTab] = 0
							return m, m.loadTabIfNeeded()
						}
					} else if !m.showDiff && !m.showDetail && !m.showHelp && !m.showSplash {
						if idx := m.termRowToCursor(msg.Y); idx >= 0 && idx < m.listLen() {
							m.cursor = idx
							m.adjustScroll()
						}
					}
				} else {
					if msg.Y == 2 || msg.Y == 3 {
						if t := tabAtX(msg.X); t >= 0 {
							m.showDiff = false
							m.showDetail = false
							m.showHelp = false
							m.showSplash = false
							m.activeTab = tab(t)
							m.cursor = 0
							m.filterQuery = ""
							m.clearCycleFilter()
							m.scrollOffset[m.activeTab] = 0
							return m, m.loadTabIfNeeded()
						}
					} else if !m.showDiff && !m.showDetail && !m.showHelp && !m.showSplash {
						if idx := m.termRowToCursor(msg.Y); idx >= 0 && idx < m.listLen() {
							m.cursor = idx
							m.adjustScroll()
						}
					}
				}
			}
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft &&
				!m.showDiff && !m.showDetail && !m.showHelp && !m.showSplash {
				// double-click detection: same row as last click within 500ms
				now := time.Now()
				if msg.Y == m.lastClickY && now.Sub(m.lastClickAt) < 500*time.Millisecond {
					m.lastClickAt = time.Time{}
					// synthesise enter — same semantics as the keyboard handler
					switch m.activeTab {
					case tabDashboard:
						if r, ok := m.repoAtCursor(); ok {
							m.showDetail = true
							m.detailScroll = 0
							m.loading = true
							m.statusMsg = fmt.Sprintf("Loading %s details…", r.Name)
							return m, loadDetail(r)
						}
					case tabPRs:
						if pr, ok := m.prAtCursor(); ok {
							if repo, ok := m.repoByName(repoBaseName(pr.Repo)); ok {
								m.showDetail = true
								m.detailScroll = 0
								m.loading = true
								m.statusMsg = fmt.Sprintf("Loading %s details…", repo.Name)
								return m, loadDetail(repo)
							}
						}
					case tabBranches:
						if br, ok := m.branchAtCursor(); ok {
							if repo, ok := m.repoByName(repoBaseName(br.Repo)); ok {
								m.showDetail = true
								m.detailScroll = 0
								m.loading = true
								m.statusMsg = fmt.Sprintf("Loading %s details…", repo.Name)
								return m, loadDetail(repo)
							}
						}
					case tabActivity:
						if c, ok := m.commitAtCursor(); ok {
							m.showDiff = true
							m.diffContent = ""
							m.diffScroll = 0
							m.loading = true
							m.statusMsg = fmt.Sprintf("Loading diff %s…", c.Hash)
							return m, loadDiff(c.RepoPath, c.Repo, c.Hash, m.width)
						}
					case tabCI:
						if r, ok := m.runAtCursor(); ok {
							if repo, ok := m.repoByName(repoBaseName(r.Repo)); ok {
								m.showDetail = true
								m.detailScroll = 0
								m.loading = true
								m.statusMsg = fmt.Sprintf("Loading %s details…", repo.Name)
								return m, loadDetail(repo)
							}
						}
					}
				} else {
					m.lastClickY = msg.Y
					m.lastClickAt = now
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustScroll()

	case reposLoadedMsg:
		m.repos = msg.repos
		m.loading = false
		m.statusMsg = fmt.Sprintf("%d repositories loaded", len(msg.repos))

	case prsLoadedMsg:
		m.prs = msg.prs
		m.errLog = msg.errors
		m.authErr = containsAuthErr(msg.errors)
		m.prsLoadedAt = time.Now()
		m.loading = false
		if len(msg.errors) > 0 {
			m.statusMsg = fmt.Sprintf("%d open PRs (%d repos failed)", len(msg.prs), len(msg.errors))
		} else {
			m.statusMsg = fmt.Sprintf("%d open PRs", len(msg.prs))
		}
		go cache.SavePRs(m.cacheDir, m.cacheKey, msg.prs) //nolint:errcheck

	case branchesLoadedMsg:
		m.branches = msg.branches
		m.errLog = msg.errors
		m.authErr = containsAuthErr(msg.errors)
		m.branchesLoadedAt = time.Now()
		m.loading = false
		if len(msg.errors) > 0 {
			m.statusMsg = fmt.Sprintf("%d branches (%d repos failed)", len(msg.branches), len(msg.errors))
		} else {
			m.statusMsg = fmt.Sprintf("%d branches", len(msg.branches))
		}
		go cache.SaveBranches(m.cacheDir, m.cacheKey, msg.branches) //nolint:errcheck

	case activityLoadedMsg:
		m.activity = msg.commits
		m.activityLoadedAt = time.Now()
		m.loading = false
		m.statusMsg = fmt.Sprintf("%d recent commits", len(msg.commits))
		go cache.SaveActivity(m.cacheDir, m.cacheKey, msg.commits) //nolint:errcheck

	case runsLoadedMsg:
		m.runs = msg.runs
		m.errLog = msg.errors
		m.authErr = containsAuthErr(msg.errors)
		m.runsLoadedAt = time.Now()
		m.loading = false
		if len(msg.errors) > 0 {
			m.statusMsg = fmt.Sprintf("%d CI runs (%d repos failed)", len(msg.runs), len(msg.errors))
		} else {
			m.statusMsg = fmt.Sprintf("%d CI runs", len(msg.runs))
		}
		go cache.SaveRuns(m.cacheDir, m.cacheKey, msg.runs) //nolint:errcheck

	case detailLoadedMsg:
		m.detailCommits = msg.commits
		m.detailPRs = msg.prs
		m.detailBranches = msg.branches
		m.detailStats = msg.stats
		m.buildDetailItems()
		m.detailCursor = 0
		m.detailScroll = 0
		m.adjustDetailScroll()
		m.loading = false
		m.statusMsg = "Detail loaded"

	case diffLoadedMsg:
		m.diffContent = msg.content
		m.diffRepo = msg.repo
		m.diffHash = msg.hash
		m.diffPreColored = msg.preColored
		m.loading = false
		m.statusMsg = fmt.Sprintf("%s/%s", msg.repo, msg.hash)

	case versionCheckMsg:
		m.latestVersion = msg.latest

	case fetchDoneMsg:
		m.statusMsg = msg.msg
		m.loading = false
		return m, loadRepos(m.cfg.Profiles)

	case statusMsg:
		m.statusMsg = string(msg)
		m.loading = false
		return m, loadRepos(m.cfg.Profiles)

	case tickMsg:
		if !m.autoRefresh {
			return m, nil
		}
		return m, tea.Batch(
			loadRepos(m.cfg.Profiles),
			tickCmd(time.Duration(m.cfg.RefreshSecs)*time.Second),
		)

	case gTimeoutMsg:
		if m.prevKey == "g" {
			m.prevKey = ""
			// In a detail or diff pane a single g is a no-op (gg was the intent).
			if !m.showDetail && !m.showDiff {
				m.grouped = !m.grouped
				m.cursor = 0
				m.adjustScroll()
			}
		}

	case idleCheckMsg:
		if m.cfg.ScreensaverSecs > 0 {
			if !m.ssActive && time.Since(m.lastActivity) >= time.Duration(m.cfg.ScreensaverSecs)*time.Second {
				m.ssActive = true
				// initialise bounce position to centre
				m.ssX = max(0, (m.width-ui.SplashArtWidth)/2)
				m.ssY = max(0, (m.height-ui.SplashArtHeight)/2)
				m.ssDX, m.ssDY = 1, 1
				m.ssColor = 0
				return m, tea.Batch(idleCheckCmd(), ssTickCmd())
			}
		}
		return m, idleCheckCmd()

	case splashBlinkMsg:
		m.splashBlink = msg.next
		return m, splashBlinkCmd(msg.next)

	case ssTickMsg:
		if !m.ssActive {
			return m, nil
		}
		artW, artH := ui.SplashArtWidth, ui.SplashArtHeight
		m.ssX += m.ssDX
		m.ssY += m.ssDY
		if m.ssX <= 0 || m.ssX+artW >= m.width {
			m.ssDX = -m.ssDX
			m.ssColor++
		}
		if m.ssY <= 0 || m.ssY+artH >= m.height {
			m.ssDY = -m.ssDY
			m.ssColor++
		}
		m.ssX = max(0, min(m.ssX, m.width-artW))
		m.ssY = max(0, min(m.ssY, m.height-artH))
		return m, ssTickCmd()
	}

	return m, nil
}
