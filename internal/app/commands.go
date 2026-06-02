package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/forge"
	gitpkg "github.com/alcxyz/grove/internal/git"
	"github.com/alcxyz/grove/internal/model"
)

func discoverRepoPaths(profile config.Profile) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(full string) {
		name := filepath.Base(full)
		if profile.ExcludesRepo(name, full) {
			return
		}
		if _, err := os.Stat(filepath.Join(full, ".git")); err != nil {
			return
		}
		if _, dup := seen[full]; dup {
			return
		}
		seen[full] = struct{}{}
		paths = append(paths, full)
	}
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
			add(full)
		}
	}
	for _, path := range profile.RepoPaths {
		add(path)
	}
	sort.Strings(paths)
	return paths
}

func profileByName(profiles []config.Profile, name string) (config.Profile, bool) {
	for _, p := range profiles {
		if p.Name == name {
			return p, true
		}
	}
	return config.Profile{}, false
}

func providerForRemote(providers map[string]forge.Provider, remote config.Remote) forge.Provider {
	return providers[remote.Key()]
}

func remoteLabel(remote config.Remote) string {
	forgeName := remote.EffectiveForge()
	if forgeName == "forgejo" && remote.InstanceURL != "" {
		return forgeName + " " + remote.InstanceURL
	}
	return forgeName
}

func formatRemoteError(repoName string, remote config.Remote, err error) string {
	remoteName := remote.RepoName(repoName)
	if remoteName != repoName {
		return fmt.Sprintf("%s -> %s/%s [%s]: %v", repoName, remote.Owner, remoteName, remoteLabel(remote), err)
	}
	return fmt.Sprintf("%s [%s]: %v", repoName, remoteLabel(remote), err)
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
				r.Owner = pp.profile.CodeRemote(r.Name, pp.path).Owner
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

func loadPRs(profiles []config.Profile, providers map[string]forge.Provider) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allPRs []model.PR
		var errs []string

		for _, p := range profiles {
			paths := discoverRepoPaths(p)
			for _, path := range paths {
				wg.Add(1)
				go func(path string, profile config.Profile) {
					defer wg.Done()
					name := filepath.Base(path)
					remote := profile.SocialRemote(name, path)
					if remote.Owner == "" {
						return
					}
					prov := providerForRemote(providers, remote)
					if prov == nil {
						return
					}
					repoFull := remote.FullName(name)
					prs, err := prov.ListPRs(repoFull)
					mu.Lock()
					if err != nil {
						errs = append(errs, formatRemoteError(name, remote, err))
					} else {
						localRepoFull := remote.Owner + "/" + name
						for i := range prs {
							prs[i].Profile = profile.Name
							prs[i].Repo = localRepoFull
							prs[i].RepoPath = path
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

func loadBranches(profiles []config.Profile, providers map[string]forge.Provider) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allBranches []model.BranchInfo
		var errs []string

		for _, p := range profiles {
			paths := discoverRepoPaths(p)
			for _, path := range paths {
				wg.Add(1)
				go func(path string, profile config.Profile) {
					defer wg.Done()
					name := filepath.Base(path)
					remote := profile.CodeRemote(name, path)
					if remote.Owner == "" {
						return
					}
					prov := providerForRemote(providers, remote)
					if prov == nil {
						return
					}
					repoFull := remote.FullName(name)
					branches, err := prov.ListBranches(repoFull)
					if err != nil {
						mu.Lock()
						errs = append(errs, formatRemoteError(name, remote, err))
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
					localRepoFull := remote.Owner + "/" + name
					for i := range branches {
						branches[i].Profile = profile.Name
						branches[i].Repo = localRepoFull
						branches[i].RepoPath = path
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

func loadRuns(profiles []config.Profile, providers map[string]forge.Provider) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allRuns []model.WorkflowRun
		var errs []string

		for _, p := range profiles {
			paths := discoverRepoPaths(p)
			for _, path := range paths {
				wg.Add(1)
				go func(path string, profile config.Profile) {
					defer wg.Done()
					name := filepath.Base(path)
					remote := profile.CIRemote(name, path)
					if remote.Owner == "" {
						return
					}
					prov := providerForRemote(providers, remote)
					if prov == nil {
						return
					}
					repoFull := remote.FullName(name)
					runs, err := prov.ListWorkflowRuns(repoFull)
					mu.Lock()
					if err != nil {
						errs = append(errs, formatRemoteError(name, remote, err))
					} else {
						localRepoFull := remote.Owner + "/" + name
						for i := range runs {
							runs[i].Profile = profile.Name
							runs[i].Repo = localRepoFull
							runs[i].RepoPath = path
						}
						allRuns = append(allRuns, runs...)
					}
					mu.Unlock()
				}(path, p)
			}
		}
		wg.Wait()

		sort.Slice(allRuns, func(i, j int) bool {
			return allRuns[i].UpdatedAt.After(allRuns[j].UpdatedAt)
		})
		return runsLoadedMsg{runs: allRuns, errors: errs}
	}
}

func loadIssues(profiles []config.Profile, providers map[string]forge.Provider) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allIssues []model.Issue
		var errs []string

		for _, p := range profiles {
			paths := discoverRepoPaths(p)
			for _, path := range paths {
				wg.Add(1)
				go func(path string, profile config.Profile) {
					defer wg.Done()
					name := filepath.Base(path)
					remote := profile.SocialRemote(name, path)
					if remote.Owner == "" {
						return
					}
					prov := providerForRemote(providers, remote)
					if prov == nil {
						return
					}
					repoFull := remote.FullName(name)
					issues, err := prov.ListIssues(repoFull)
					mu.Lock()
					if err != nil {
						errs = append(errs, formatRemoteError(name, remote, err))
					} else {
						localRepoFull := remote.Owner + "/" + name
						for i := range issues {
							issues[i].Profile = profile.Name
							issues[i].Repo = localRepoFull
							issues[i].RepoPath = path
						}
						allIssues = append(allIssues, issues...)
					}
					mu.Unlock()
				}(path, p)
			}
		}
		wg.Wait()

		sort.Slice(allIssues, func(i, j int) bool {
			return allIssues[i].UpdatedAt.After(allIssues[j].UpdatedAt)
		})
		return issuesLoadedMsg{issues: allIssues, errors: errs}
	}
}

func loadDetail(repo model.Repo, profile config.Profile, providers map[string]forge.Provider) tea.Cmd {
	return func() tea.Msg {
		var wg sync.WaitGroup
		var commits []model.Commit
		var prs []model.PR
		var issues []model.Issue
		var branches []string
		var stats model.RepoStats

		wg.Add(6)
		go func() {
			defer wg.Done()
			commits, _ = gitpkg.RecentCommits(repo.Path, 15)
			for i := range commits {
				commits[i].RepoPath = repo.Path
				commits[i].Repo = repo.Name
			}
		}()
		go func() {
			defer wg.Done()
			remote := profile.SocialRemote(repo.Name, repo.Path)
			if remote.Owner != "" {
				if provider := providerForRemote(providers, remote); provider != nil {
					prs, _ = provider.ListPRs(remote.FullName(repo.Name))
					localRepoFull := remote.Owner + "/" + repo.Name
					for i := range prs {
						prs[i].Profile = profile.Name
						prs[i].Repo = localRepoFull
						prs[i].RepoPath = repo.Path
					}
				}
			}
		}()
		go func() {
			defer wg.Done()
			remote := profile.SocialRemote(repo.Name, repo.Path)
			if remote.Owner != "" {
				if provider := providerForRemote(providers, remote); provider != nil {
					issues, _ = provider.ListIssues(remote.FullName(repo.Name))
					localRepoFull := remote.Owner + "/" + repo.Name
					for i := range issues {
						issues[i].Profile = profile.Name
						issues[i].Repo = localRepoFull
						issues[i].RepoPath = repo.Path
					}
				}
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

		return detailLoadedMsg{commits: commits, prs: prs, issues: issues, branches: branches, stats: stats}
	}
}

// launchLazygit suspends grove and opens lazygit in the given repo directory.
// When the user exits lazygit the terminal is handed back to grove. If lazygit
// isn't on PATH or the repo path is missing, the error surfaces as a status
// message rather than silently returning to the list.
func launchLazygit(path string) tea.Cmd {
	if path == "" {
		return func() tea.Msg { return statusMsg("no repo selected") }
	}
	if _, err := exec.LookPath("lazygit"); err != nil {
		return func() tea.Msg { return statusMsg("lazygit not found on PATH") }
	}
	c := exec.Command("lazygit", "-p", path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return statusMsg(fmt.Sprintf("lazygit exited: %v", err))
		}
		return statusMsg("back in grove")
	})
}

// launchNvim suspends grove and opens nvim at the given repo directory.
func launchNvim(path string) tea.Cmd {
	if path == "" {
		return func() tea.Msg { return statusMsg("no repo selected") }
	}
	bin := os.Getenv("EDITOR")
	if bin == "" {
		bin = "nvim"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return func() tea.Msg { return statusMsg(fmt.Sprintf("%s not found on PATH", bin)) }
	}
	c := exec.Command(bin, ".")
	c.Dir = path
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return statusMsg(fmt.Sprintf("%s exited: %v", bin, err))
		}
		return statusMsg("back in grove")
	})
}

// launchGhDash suspends grove and opens gh-dash in the given repo directory.
func launchGhDash(path string) tea.Cmd {
	if path == "" {
		return func() tea.Msg { return statusMsg("no repo selected") }
	}
	if _, err := exec.LookPath("gh-dash"); err != nil {
		return func() tea.Msg { return statusMsg("gh-dash not found on PATH") }
	}
	c := exec.Command("gh-dash")
	c.Dir = path
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return statusMsg(fmt.Sprintf("gh-dash exited: %v", err))
		}
		return statusMsg("back in grove")
	})
}

// launchLazygitOnBranch checks out the target branch, opens lazygit, and
// restores the original branch when lazygit exits.
func launchLazygitOnBranch(repoPath, targetBranch string) tea.Cmd {
	if repoPath == "" {
		return func() tea.Msg { return statusMsg("no repo selected") }
	}
	if _, err := exec.LookPath("lazygit"); err != nil {
		return func() tea.Msg { return statusMsg("lazygit not found on PATH") }
	}
	origBranch, err := gitpkg.CurrentBranch(repoPath)
	if err != nil {
		return func() tea.Msg { return statusMsg(fmt.Sprintf("could not get current branch: %v", err)) }
	}
	if err := gitpkg.Checkout(repoPath, targetBranch); err != nil {
		return func() tea.Msg { return statusMsg(fmt.Sprintf("checkout %s failed: %v", targetBranch, err)) }
	}
	c := exec.Command("lazygit", "-p", repoPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		// Always restore the original branch, even if lazygit errored.
		_ = gitpkg.Checkout(repoPath, origBranch)
		if err != nil {
			return statusMsg(fmt.Sprintf("lazygit exited: %v", err))
		}
		return statusMsg(fmt.Sprintf("back in grove (restored %s)", origBranch))
	})
}

// launchEditorAt opens $EDITOR/nvim at a specific file path.
func launchEditorAt(filePath string) tea.Cmd {
	if filePath == "" {
		return func() tea.Msg { return statusMsg("no file to open") }
	}
	bin := os.Getenv("EDITOR")
	if bin == "" {
		bin = "nvim"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return func() tea.Msg { return statusMsg(fmt.Sprintf("%s not found on PATH", bin)) }
	}
	c := exec.Command(bin, filePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return statusMsg(fmt.Sprintf("%s exited: %v", bin, err))
		}
		return statusMsg("back in grove")
	})
}

// workflowFilePath returns the absolute path to the workflow file for a CI run.
// Prefers the API-provided WorkflowFile path; falls back to scanning the local
// .github/workflows/ directory for a matching name: field or filename.
func workflowFilePath(repoPath string, run model.WorkflowRun) string {
	if run.WorkflowFile != "" {
		full := filepath.Join(repoPath, run.WorkflowFile)
		if _, err := os.Stat(full); err == nil {
			return full
		}
	}
	// Fallback: scan local workflow files.
	dir := filepath.Join(repoPath, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		full := filepath.Join(dir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "name:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				val = strings.Trim(val, "\"'")
				if strings.EqualFold(val, run.WorkflowName) {
					return full
				}
				break
			}
		}
	}
	return ""
}

// launchDiffnav suspends grove and opens diffnav for a specific commit.
func launchDiffnav(repoPath, hash string) tea.Cmd {
	if repoPath == "" {
		return func() tea.Msg { return statusMsg("no repo selected") }
	}
	if _, err := exec.LookPath("diffnav"); err != nil {
		return func() tea.Msg { return statusMsg("diffnav not found on PATH") }
	}
	// pipe git show into diffnav
	c := exec.Command("bash", "-c", fmt.Sprintf("cd %q && git show %s | diffnav", repoPath, hash))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return statusMsg(fmt.Sprintf("diffnav exited: %v", err))
		}
		return statusMsg("back in grove")
	})
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

type releaseVersion struct {
	major int
	minor int
	patch int
}

func parseReleaseVersion(v string) (releaseVersion, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return releaseVersion{}, false
	}
	var parsed [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return releaseVersion{}, false
		}
		parsed[i] = n
	}
	return releaseVersion{major: parsed[0], minor: parsed[1], patch: parsed[2]}, true
}

func IsReleaseVersion(v string) bool {
	_, ok := parseReleaseVersion(v)
	return ok
}

func compareReleaseVersion(a, b releaseVersion) int {
	if a.major != b.major {
		return a.major - b.major
	}
	if a.minor != b.minor {
		return a.minor - b.minor
	}
	return a.patch - b.patch
}

func isNewerRelease(latest, current string) bool {
	lv, ok := parseReleaseVersion(latest)
	if !ok {
		return false
	}
	cv, ok := parseReleaseVersion(current)
	if !ok {
		return false
	}
	return compareReleaseVersion(lv, cv) > 0
}

// LatestVersion fetches the latest GitHub release tag and returns it only when
// it is semantically newer than the current version. It silently no-ops for dev
// builds, hash builds, or when the network is unavailable.
func LatestVersion(version string) string {
	current, ok := parseReleaseVersion(version)
	if !ok {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/alcxyz/grove/releases?per_page=20", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "grove/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var payload []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	var latestTag string
	var latest releaseVersion
	for _, rel := range payload {
		if rel.Draft || rel.Prerelease {
			continue
		}
		rv, ok := parseReleaseVersion(rel.TagName)
		if !ok {
			continue
		}
		if latestTag == "" || compareReleaseVersion(rv, latest) > 0 {
			latestTag = rel.TagName
			latest = rv
		}
	}
	if latestTag == "" || compareReleaseVersion(latest, current) <= 0 {
		return ""
	}
	return latestTag
}

// checkLatestVersion fetches the latest GitHub release tag in the background
// and returns a versionCheckMsg if a newer version is available.
func checkLatestVersion(version string) tea.Cmd {
	return func() tea.Msg {
		return versionCheckMsg{latest: LatestVersion(version)}
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

// splashBlinkCmd schedules the next eye-blink frame for the ! splash overlay.
//
//   - When current==0 (eyes open): wait 1.5–4 s, then randomly blink one eye
//     or both (states 1, 2, 3).
//   - When current!=0 (eyes closed): wait 80–150 ms, then reopen (state 0).
func splashBlinkCmd(current int) tea.Cmd {
	var d time.Duration
	var next int
	if current == 0 {
		d = time.Duration(1500+rand.Intn(2500)) * time.Millisecond
		next = rand.Intn(3) + 1 // 1=left, 2=right, 3=both
	} else {
		d = time.Duration(80+rand.Intn(70)) * time.Millisecond
		next = 0
	}
	return tea.Tick(d, func(time.Time) tea.Msg { return splashBlinkMsg{next: next} })
}

// loadTabIfNeeded returns a load command when the active tab's data is stale
// or missing.  Returns nil if no fetch is required.
func (m *Model) loadTabIfNeeded() tea.Cmd {
	ttl := time.Duration(m.cfg.RefreshSecs) * time.Second
	switch m.activeTab {
	case tabPRs:
		if len(m.prs) == 0 || time.Since(m.prsLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading PRs..."
			return loadPRs(m.cfg.Profiles, m.providers)
		}
	case tabBranches:
		if len(m.branches) == 0 || time.Since(m.branchesLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading branches..."
			return loadBranches(m.cfg.Profiles, m.providers)
		}
	case tabActivity:
		if len(m.activity) == 0 || time.Since(m.activityLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading activity..."
			return loadActivity(m.cfg.Profiles)
		}
	case tabCI:
		if len(m.runs) == 0 || time.Since(m.runsLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading CI runs..."
			return loadRuns(m.cfg.Profiles, m.providers)
		}
	case tabIssues:
		if len(m.issues) == 0 || time.Since(m.issuesLoadedAt) > ttl {
			m.loading = true
			m.statusMsg = "Loading issues..."
			return loadIssues(m.cfg.Profiles, m.providers)
		}
	}
	return nil
}
