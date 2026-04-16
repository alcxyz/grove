package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/gh"
	gitpkg "github.com/alcxyz/grove/internal/git"
	"github.com/alcxyz/grove/internal/model"
)

func discoverRepoPaths(profile config.Profile) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, base := range profile.BasePaths {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			// Empty prefixes = match all repos in this base_path.
			matched := len(profile.Prefixes) == 0
			for _, prefix := range profile.Prefixes {
				if strings.HasPrefix(name, prefix) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			full := filepath.Join(base, name)
			if _, err := os.Stat(filepath.Join(full, ".git")); err != nil {
				continue
			}
			if _, dup := seen[full]; dup {
				continue
			}
			seen[full] = struct{}{}
			paths = append(paths, full)
		}
	}
	sort.Strings(paths)
	return paths
}

func loadRepos(profiles []config.Profile) tea.Cmd {
	return func() tea.Msg {
		// Collect all (path, profile) pairs, deduplicated by path.
		type pathProfile struct {
			path    string
			profile config.Profile
		}
		seen := map[string]struct{}{}
		var pairs []pathProfile
		for _, p := range profiles {
			for _, path := range discoverRepoPaths(p) {
				if _, dup := seen[path]; dup {
					continue
				}
				seen[path] = struct{}{}
				pairs = append(pairs, pathProfile{path: path, profile: p})
			}
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		repos := make([]model.Repo, 0, len(pairs))

		for _, pp := range pairs {
			wg.Add(1)
			go func(pp pathProfile) {
				defer wg.Done()
				r, err := gitpkg.GetRepoStatus(pp.path)
				if err != nil {
					r = model.Repo{Name: filepath.Base(pp.path), Path: pp.path}
				}
				r.Owner = pp.profile.Owner
				r.Profile = pp.profile.Name
				mu.Lock()
				repos = append(repos, r)
				mu.Unlock()
			}(pp)
		}
		wg.Wait()

		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Name < repos[j].Name
		})
		return reposLoadedMsg{repos}
	}
}

func loadPRs(profiles []config.Profile) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allPRs []model.PR
		var errs []string

		for _, p := range profiles {
			if p.Owner == "" {
				continue
			}
			paths := discoverRepoPaths(p)
			for _, path := range paths {
				wg.Add(1)
				go func(path string, profile config.Profile) {
					defer wg.Done()
					name := filepath.Base(path)
					repoFull := profile.Owner + "/" + name
					prs, err := gh.ListPRs(repoFull)
					mu.Lock()
					if err != nil {
						errs = append(errs, fmt.Sprintf("%s: %v", name, err))
					} else {
						for i := range prs {
							prs[i].Profile = profile.Name
						}
						allPRs = append(allPRs, prs...)
					}
					mu.Unlock()
				}(path, p)
			}
		}
		wg.Wait()

		sort.Slice(allPRs, func(i, j int) bool {
			return allPRs[i].UpdatedAt.After(allPRs[j].UpdatedAt)
		})
		return prsLoadedMsg{prs: allPRs, errors: errs}
	}
}

func loadBranches(profiles []config.Profile) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allBranches []model.BranchInfo
		var errs []string

		for _, p := range profiles {
			if p.Owner == "" {
				continue
			}
			paths := discoverRepoPaths(p)
			for _, path := range paths {
				wg.Add(1)
				go func(path string, profile config.Profile) {
					defer wg.Done()
					name := filepath.Base(path)
					repoFull := profile.Owner + "/" + name
					branches, err := gh.ListBranches(repoFull)
					if err != nil {
						mu.Lock()
						errs = append(errs, fmt.Sprintf("%s: %v", name, err))
						mu.Unlock()
						return
					}
					// Populate IsMerged using local git objects (fast, no API call)
					defaultBranch := ""
					for _, br := range branches {
						if br.IsDefault {
							defaultBranch = br.Name
							break
						}
					}
					if defaultBranch != "" {
						if merged, _ := gitpkg.MergedRemoteBranches(path, defaultBranch); merged != nil {
							for i := range branches {
								// Don't mark the default branch itself as merged
								branches[i].IsMerged = merged[branches[i].Name] && !branches[i].IsDefault
							}
						}
					}
					for i := range branches {
						branches[i].Profile = profile.Name
					}
					mu.Lock()
					allBranches = append(allBranches, branches...)
					mu.Unlock()
				}(path, p)
			}
		}
		wg.Wait()

		sort.Slice(allBranches, func(i, j int) bool {
			if allBranches[i].Repo != allBranches[j].Repo {
				return allBranches[i].Repo < allBranches[j].Repo
			}
			return allBranches[i].Name < allBranches[j].Name
		})
		return branchesLoadedMsg{branches: allBranches, errors: errs}
	}
}

func loadActivity(profiles []config.Profile) tea.Cmd {
	return func() tea.Msg {
		// Collect all (path, profile) pairs, deduplicated by path.
		type pathProfile struct {
			path    string
			profile config.Profile
		}
		seen := map[string]struct{}{}
		var pairs []pathProfile
		for _, p := range profiles {
			for _, path := range discoverRepoPaths(p) {
				if _, dup := seen[path]; dup {
					continue
				}
				seen[path] = struct{}{}
				pairs = append(pairs, pathProfile{path: path, profile: p})
			}
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		var all []model.Commit

		for _, pp := range pairs {
			wg.Add(1)
			go func(pp pathProfile) {
				defer wg.Done()
				repoName := filepath.Base(pp.path)
				commits, err := gitpkg.RecentCommits(pp.path, 3)
				if err != nil {
					return
				}
				for i := range commits {
					commits[i].Repo = repoName
					commits[i].RepoPath = pp.path
					commits[i].Profile = pp.profile.Name
				}
				mu.Lock()
				all = append(all, commits...)
				mu.Unlock()
			}(pp)
		}
		wg.Wait()

		sort.Slice(all, func(i, j int) bool {
			return all[i].Date.After(all[j].Date)
		})

		// Cap at 30 most recent
		if len(all) > 30 {
			all = all[:30]
		}
		return activityLoadedMsg{all}
	}
}

func loadDetail(repo model.Repo) tea.Cmd {
	return func() tea.Msg {
		var wg sync.WaitGroup
		var commits []model.Commit
		var prs []model.PR
		var branches []string
		var stats model.RepoStats

		wg.Add(5)
		go func() {
			defer wg.Done()
			commits, _ = gitpkg.RecentCommits(repo.Path, 15)
		}()
		go func() {
			defer wg.Done()
			if repo.Owner != "" {
				prs, _ = gh.ListPRs(repo.Owner + "/" + repo.Name)
			}
		}()
		go func() {
			defer wg.Done()
			branches, _ = gitpkg.LocalBranches(repo.Path)
		}()
		go func() {
			defer wg.Done()
			stats.CommitCount, _ = gitpkg.CommitCount(repo.Path)
		}()
		go func() {
			defer wg.Done()
			stats.Contributors, _ = gitpkg.Contributors(repo.Path)
		}()
		wg.Wait()

		return detailLoadedMsg{commits: commits, prs: prs, branches: branches, stats: stats}
	}
}

func loadDiff(repoPath, repoName, hash string, width int) tea.Cmd {
	return func() tea.Msg {
		content, preColored, err := gitpkg.CommitDiffFormatted(repoPath, hash, width)
		if err != nil {
			content = fmt.Sprintf("error running git show: %v", err)
		}
		return diffLoadedMsg{content: content, repo: repoName, hash: hash, preColored: preColored}
	}
}

func fetchAll(profiles []config.Profile) tea.Cmd {
	return func() tea.Msg {
		// Collect all paths, deduplicated.
		seen := map[string]struct{}{}
		var paths []string
		for _, p := range profiles {
			for _, path := range discoverRepoPaths(p) {
				if _, dup := seen[path]; dup {
					continue
				}
				seen[path] = struct{}{}
				paths = append(paths, path)
			}
		}

		var wg sync.WaitGroup
		var errCount int
		var mu sync.Mutex

		for _, p := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				if err := gitpkg.Fetch(path); err != nil {
					mu.Lock()
					errCount++
					mu.Unlock()
				}
			}(p)
		}
		wg.Wait()

		msg := fmt.Sprintf("Fetched %d repos", len(paths)-errCount)
		if errCount > 0 {
			msg += fmt.Sprintf(" (%d failed)", errCount)
		}
		return fetchDoneMsg{msg}
	}
}

// checkLatestVersion fetches the latest GitHub release tag in the background
// and returns a versionCheckMsg if a newer version is available.
// Silently no-ops for dev builds or when the network is unavailable.
func checkLatestVersion() tea.Cmd {
	return func() tea.Msg {
		if version == "dev" {
			return versionCheckMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.github.com/repos/alcxyz/grove/releases/latest", nil)
		if err != nil {
			return versionCheckMsg{}
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "grove/"+version)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return versionCheckMsg{}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return versionCheckMsg{}
		}
		var payload struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return versionCheckMsg{}
		}
		// tag_name is "v0.2.0"; version is "0.2.0" (injected by goreleaser without v-prefix)
		if payload.TagName == "v"+version || payload.TagName == version {
			return versionCheckMsg{}
		}
		return versionCheckMsg{latest: payload.TagName}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func ssTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return ssTickMsg{} })
}

func idleCheckCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg { return idleCheckMsg{} })
}

// loadTabIfNeeded returns a load command when the active tab's data is stale
// or missing.  Returns nil if no fetch is required.
func (m *appModel) loadTabIfNeeded() tea.Cmd {
	ttl := time.Duration(m.cfg.RefreshSecs) * time.Second
	switch m.activeTab {
	case tabPRs:
		if len(m.prs) == 0 || time.Since(m.prsLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading PRs..."
			return loadPRs(m.cfg.Profiles)
		}
	case tabBranches:
		if len(m.branches) == 0 || time.Since(m.branchesLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading branches..."
			return loadBranches(m.cfg.Profiles)
		}
	case tabActivity:
		if len(m.activity) == 0 || time.Since(m.activityLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading activity..."
			return loadActivity(m.cfg.Profiles)
		}
	}
	return nil
}
