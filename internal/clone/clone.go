package clone

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/alcxyz/grove/internal/config"
)

// Run enumerates org repos for each matching profile and clones any that
// are missing locally.
func Run(cfg config.Config) {
	// Optional profile names from args: grove clone [profile1 profile2 ...]
	filter := parseProfileFilter(cfg, os.Args[2:])

	ran := false
	for _, profile := range cfg.Profiles {
		if filter != nil && !filter[profile.Name] {
			continue
		}
		if profile.Owner == "" || len(profile.BasePaths) == 0 {
			fmt.Fprintf(os.Stderr, "profile %q: missing owner or base_path, skipping\n", profile.Name)
			continue
		}
		ran = true
		cloneProfile(profile)
	}

	if !ran {
		fmt.Fprintln(os.Stderr, "No matching profiles with owner and base_path configured.")
		os.Exit(1)
	}
}

func cloneProfile(profile config.Profile) {
	fmt.Printf("Profile %q  owner=%s\n", profile.Name, profile.Owner)

	names, err := listOrgRepos(profile.Owner, profile.Prefixes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		return
	}
	fmt.Printf("  %d matching repos\n", len(names))

	searchPaths := allSearchPaths(profile)

	var toClone []string
	for _, name := range names {
		if !repoExistsInAny(searchPaths, name) {
			toClone = append(toClone, name)
		}
	}

	alreadyPresent := len(names) - len(toClone)
	if len(toClone) == 0 {
		fmt.Printf("  all %d repos already present\n", alreadyPresent)
		return
	}
	fmt.Printf("  %d already present, cloning %d\n", alreadyPresent, len(toClone))

	type cloneResult struct {
		name string
		dest string
		err  error
	}
	results := make(chan cloneResult, len(toClone))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for _, name := range toClone {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			dest := cloneDestFor(profile, n)
			target := filepath.Join(dest, n)
			cmd := exec.Command("gh", "repo", "clone", profile.Owner+"/"+n, target, "--", "--quiet")
			out, err := cmd.CombinedOutput()
			results <- cloneResult{name: n, dest: dest, err: wrapCloneErr(err, out)}
		}(name)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var cloned, failed int
	for r := range results {
		if r.err != nil {
			fmt.Printf("  ✗ %s: %v\n", r.name, r.err)
			failed++
		} else {
			fmt.Printf("  ✓ %s  →  %s\n", r.name, r.dest)
			cloned++
		}
	}
	fmt.Printf("  done — cloned: %d, failed: %d\n", cloned, failed)
}

// parseProfileFilter validates the requested profile names and returns a set,
// or nil when no filter is requested (meaning all profiles).
func parseProfileFilter(cfg config.Config, names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	known := make(map[string]bool, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		known[p.Name] = true
	}
	var unknown []string
	for _, n := range names {
		if !known[n] {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		var avail []string
		for _, p := range cfg.Profiles {
			avail = append(avail, fmt.Sprintf("%q", p.Name))
		}
		fmt.Fprintf(os.Stderr, "unknown profile(s): %s\navailable: %s\n",
			strings.Join(unknown, ", "), strings.Join(avail, ", "))
		os.Exit(1)
	}
	filter := make(map[string]bool, len(names))
	for _, n := range names {
		filter[n] = true
	}
	return filter
}

// cloneDestFor returns the destination base directory for a repo. It checks
// groups in order and uses the first one whose match prefix fits the repo name
// and has a base_path set. Falls back to profile.BasePaths[0].
func cloneDestFor(profile config.Profile, repoName string) string {
	for _, g := range profile.Groups {
		if g.BasePath == "" {
			continue
		}
		if g.Match == "" || strings.HasPrefix(repoName, g.Match) {
			return g.BasePath
		}
	}
	return profile.BasePaths[0]
}

// allSearchPaths returns all unique paths to check when deciding whether a
// repo already exists: the profile's base_paths plus any group base_paths.
func allSearchPaths(profile config.Profile) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, p := range profile.BasePaths {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, g := range profile.Groups {
		if g.BasePath != "" && !seen[g.BasePath] {
			seen[g.BasePath] = true
			paths = append(paths, g.BasePath)
		}
	}
	return paths
}

// listOrgRepos lists all repos in owner that start with any of the given
// prefixes. An empty prefix list matches all repos.
func listOrgRepos(owner string, prefixes []string) ([]string, error) {
	names, err := ghListRepos("orgs/" + owner + "/repos")
	if err != nil {
		names, err = ghListRepos("users/" + owner + "/repos")
		if err != nil {
			return nil, fmt.Errorf("listing repos for %s: %w", owner, err)
		}
	}

	if len(prefixes) == 0 {
		sort.Strings(names)
		return names, nil
	}

	var filtered []string
	for _, name := range names {
		for _, p := range prefixes {
			if strings.HasPrefix(name, p) {
				filtered = append(filtered, name)
				break
			}
		}
	}
	sort.Strings(filtered)
	return filtered, nil
}

// ghListRepos paginates through a GitHub repos API endpoint and returns all
// repo names.
func ghListRepos(endpoint string) ([]string, error) {
	var all []string
	for page := 1; ; page++ {
		url := fmt.Sprintf("%s?per_page=100&page=%d", endpoint, page)
		cmd := exec.Command("gh", "api", url)
		out, err := cmd.Output()
		if err != nil {
			if page == 1 {
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					return nil, fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
				}
				return nil, err
			}
			break
		}

		var repos []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(out, &repos); err != nil {
			return nil, fmt.Errorf("parse response page %d: %w", page, err)
		}
		for _, r := range repos {
			all = append(all, r.Name)
		}
		if len(repos) < 100 {
			break
		}
	}
	return all, nil
}

// repoExistsInAny returns true if a .git directory exists at name/ under any
// of the given base paths.
func repoExistsInAny(basePaths []string, name string) bool {
	for _, base := range basePaths {
		if _, err := os.Stat(filepath.Join(base, name, ".git")); err == nil {
			return true
		}
	}
	return false
}

func wrapCloneErr(err error, output []byte) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(output))
	if msg == "" {
		return err
	}
	return fmt.Errorf("%s", msg)
}
