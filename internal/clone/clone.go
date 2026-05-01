package clone

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/forge"
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
		prov, err := forge.NewProvider(forge.ProviderConfig{
			Forge:       profile.Forge,
			InstanceURL: profile.InstanceURL,
			TokenFile:   profile.TokenFile,
			CloneProto:  profile.CloneProto,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "profile %q: %v\n", profile.Name, err)
			continue
		}
		ran = true
		cloneProfile(profile, prov)
	}

	if !ran {
		fmt.Fprintln(os.Stderr, "No matching profiles with owner and base_path configured.")
		os.Exit(1)
	}
}

func cloneProfile(profile config.Profile, provider forge.Provider) {
	fmt.Printf("Profile %q  owner=%s\n", profile.Name, profile.Owner)

	names, err := provider.ListRepos(profile.Owner, profile.Prefixes)
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
			results <- cloneResult{name: n, dest: dest, err: provider.CloneRepo(profile.Owner, n, target)}
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
