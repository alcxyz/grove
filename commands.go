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

func discoverRepoPaths(cfg config.Config) []string {
	entries, err := os.ReadDir(cfg.BasePath)
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		matched := false
		for _, prefix := range cfg.Prefixes {
			if strings.HasPrefix(name, prefix) {
				matched = true
				break
			}
		}
		if matched {
			full := filepath.Join(cfg.BasePath, name)
			if _, err := os.Stat(filepath.Join(full, ".git")); err == nil {
				paths = append(paths, full)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func loadRepos(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		paths := discoverRepoPaths(cfg)
		var mu sync.Mutex
		var wg sync.WaitGroup
		repos := make([]model.Repo, 0, len(paths))

		for _, p := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				r, err := gitpkg.GetRepoStatus(path)
				if err != nil {
					r = model.Repo{Name: filepath.Base(path), Path: path}
				}
				mu.Lock()
				repos = append(repos, r)
				mu.Unlock()
			}(p)
		}
		wg.Wait()

		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Name < repos[j].Name
		})
		return reposLoadedMsg{repos}
	}
}

func loadPRs(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		paths := discoverRepoPaths(cfg)
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allPRs []model.PR
		var errs []string

		for _, p := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				name := filepath.Base(path)
				repoFull := cfg.Org + "/" + name
				prs, err := gh.ListPRs(repoFull)
				mu.Lock()
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", name, err))
				} else {
					allPRs = append(allPRs, prs...)
				}
				mu.Unlock()
			}(p)
		}
		wg.Wait()

		sort.Slice(allPRs, func(i, j int) bool {
			return allPRs[i].UpdatedAt.After(allPRs[j].UpdatedAt)
		})
		return prsLoadedMsg{prs: allPRs, errors: errs}
	}
}

func loadBranches(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		paths := discoverRepoPaths(cfg)
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allBranches []model.BranchInfo
		var errs []string

		for _, p := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				name := filepath.Base(path)
				repoFull := cfg.Org + "/" + name
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
				mu.Lock()
				allBranches = append(allBranches, branches...)
				mu.Unlock()
			}(p)
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

func loadActivity(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		paths := discoverRepoPaths(cfg)
		var mu sync.Mutex
		var wg sync.WaitGroup
		var all []model.Commit

		for _, p := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				repoName := filepath.Base(path)
				commits, err := gitpkg.RecentCommits(path, 3)
				if err != nil {
					return
				}
				for i := range commits {
					commits[i].Repo = repoName
					commits[i].RepoPath = path
				}
				mu.Lock()
				all = append(all, commits...)
				mu.Unlock()
			}(p)
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

func loadDetail(cfg config.Config, repo model.Repo) tea.Cmd {
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
			prs, _ = gh.ListPRs(cfg.Org + "/" + repo.Name)
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

func fetchAll(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		paths := discoverRepoPaths(cfg)
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
			return loadPRs(m.cfg)
		}
	case tabBranches:
		if len(m.branches) == 0 || time.Since(m.branchesLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading branches..."
			return loadBranches(m.cfg)
		}
	case tabActivity:
		if len(m.activity) == 0 || time.Since(m.activityLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading activity..."
			return loadActivity(m.cfg)
		}
	}
	return nil
}
