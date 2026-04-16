package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func GetRepoStatus(path string) (model.Repo, error) {
	name := filepath.Base(path)
	r := model.Repo{Name: name, Path: path}

	branch, err := run(path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return r, err
	}
	r.Branch = branch

	status, err := run(path, "status", "--porcelain")
	if err != nil {
		return r, err
	}
	r.Dirty = status != ""

	// ahead/behind
	revList, _ := run(path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if revList != "" {
		parts := strings.Fields(revList)
		if len(parts) == 2 {
			r.Ahead, _ = strconv.Atoi(parts[0])
			r.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	// last commit
	logLine, _ := run(path, "log", "-1", "--format=%aI\t%an")
	if logLine != "" {
		parts := strings.SplitN(logLine, "\t", 2)
		if len(parts) == 2 {
			r.LastCommit, _ = time.Parse(time.RFC3339, parts[0])
			r.LastAuthor = parts[1]
		}
	}

	return r, nil
}

func RecentCommits(path string, n int) ([]model.Commit, error) {
	out, err := run(path, "log", fmt.Sprintf("-%d", n), "--format=%h\t%s\t%an\t%aI")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	commits := make([]model.Commit, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		t, _ := time.Parse(time.RFC3339, parts[3])
		commits = append(commits, model.Commit{
			Hash:    parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    t,
		})
	}
	return commits, nil
}

func LocalBranches(path string) ([]string, error) {
	out, err := run(path, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// CommitDiff returns the full patch for a single commit (git show).
func CommitDiff(repoPath, hash string) (string, error) {
	return run(repoPath, "show", "--stat", "--patch", "--no-color", hash)
}

// CommitCount returns the total number of commits reachable from HEAD.
func CommitCount(path string) (int, error) {
	out, err := run(path, "rev-list", "--count", "HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// Contributors returns the number of unique commit authors (excluding merges).
func Contributors(path string) (int, error) {
	out, err := run(path, "shortlog", "-sn", "--no-merges", "HEAD")
	if err != nil {
		return 0, err
	}
	if out == "" {
		return 0, nil
	}
	return len(strings.Split(strings.TrimSpace(out), "\n")), nil
}

// MergedRemoteBranches returns a set of remote branch names that have been
// merged into origin/<defaultBranch>.  Uses local git objects so it is fast
// and works offline after a fetch.
func MergedRemoteBranches(path, defaultBranch string) (map[string]bool, error) {
	out, err := run(path, "branch", "-r", "--merged", "origin/"+defaultBranch)
	if err != nil {
		return nil, err
	}
	merged := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		// strip "origin/" prefix; skip HEAD pseudo-ref
		if after, ok := strings.CutPrefix(name, "origin/"); ok && after != "HEAD" {
			merged[after] = true
		}
	}
	return merged, nil
}

func Fetch(path string) error {
	_, err := run(path, "fetch", "--quiet")
	return err
}

func Pull(path string) (string, error) {
	return run(path, "pull", "--ff-only")
}
