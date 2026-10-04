package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

// Timeouts for git invocations. Network operations get longer because large
// fetches and pushes are legitimately slow. Variables so tests can shorten them.
var (
	localTimeout   = time.Minute
	networkTimeout = 5 * time.Minute
)

func run(dir string, args ...string) (string, error) {
	return runWithTimeout(localTimeout, dir, args...)
}

// runWithTimeout runs git without access to the terminal: grove owns it while
// the TUI is active, so prompts would hang the command or corrupt the screen.
// Errors include git's own explanation from stderr.
func runWithTimeout(timeout time.Duration, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	detachFromTerminal(cmd)
	// Stop waiting on output pipes shortly after cancellation even if a
	// helper process survived the kill.
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		cmdLine := strings.Join(args, " ")
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("git %s: timed out after %s", cmdLine, timeout)
		}
		if msg := stderrSummary(stderr.String()); msg != "" {
			return "", fmt.Errorf("git %s: %s", cmdLine, msg)
		}
		return "", fmt.Errorf("git %s: %w", cmdLine, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// stderrSummary picks the most useful line of git's stderr: the first
// "fatal:" or "error:" line, otherwise the first line that is not a hint.
func stderrSummary(stderr string) string {
	first := ""
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "hint:") {
			continue
		}
		if strings.HasPrefix(line, "fatal:") || strings.HasPrefix(line, "error:") {
			return line
		}
		if first == "" {
			first = line
		}
	}
	return first
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

// CommitDiff returns the full patch for a single commit (git show), no color.
func CommitDiff(repoPath, hash string) (string, error) {
	return run(repoPath, "show", "--stat", "--patch", "--no-color", hash)
}

// diffPager returns the user's configured diff pager.
// Priority: GIT_PAGER env → git config core.pager → PAGER env → ""
func diffPager(repoPath string) string {
	if p := os.Getenv("GIT_PAGER"); p != "" {
		return p
	}
	if out, err := run(repoPath, "config", "core.pager"); err == nil && out != "" {
		return out
	}
	if p := os.Getenv("PAGER"); p != "" {
		return p
	}
	return ""
}

// runThroughDelta pipes git output through delta (or bat) with no paging.
// Unknown pagers fall through and the original input is returned.
func runThroughDelta(pagerCmd, input string, width int) (string, error) {
	base := filepath.Base(strings.Fields(pagerCmd)[0])
	var cmd *exec.Cmd
	switch base {
	case "delta":
		cmd = exec.Command("delta", "--paging=never", fmt.Sprintf("--width=%d", width))
	case "bat":
		cmd = exec.Command("bat", "--paging=never", "--color=always", "--plain")
	default:
		return input, nil
	}
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		return input, nil // pager failed; fall back to original
	}
	return string(out), nil
}

// CommitDiffFormatted returns the patch for a commit, optionally processed
// through the user's configured diff pager.
// Returns (content, preColored, error). preColored=true means the content
// already contains ANSI escape sequences and should not be re-colored.
func CommitDiffFormatted(repoPath, hash string, width int) (string, bool, error) {
	pager := diffPager(repoPath)
	if pager == "" {
		content, err := CommitDiff(repoPath, hash)
		return content, false, err
	}
	// Produce colored git output for the pager
	cmd := exec.Command("git", "show", "--stat", "--patch", "--color=always", hash)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		// Fallback to plain diff
		content, err2 := CommitDiff(repoPath, hash)
		return content, false, err2
	}
	result, _ := runThroughDelta(pager, string(out), width)
	return result, true, nil
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

// CurrentBranch returns the current branch name (or "HEAD" if detached). It
// reads the full ref, because rev-parse --abbrev-ref prints "heads/<name>"
// when a tag has the same name as the branch.
func CurrentBranch(path string) (string, error) {
	if ref, err := run(path, "symbolic-ref", "-q", "HEAD"); err == nil {
		return strings.TrimPrefix(ref, "refs/heads/"), nil
	}
	return run(path, "rev-parse", "--abbrev-ref", "HEAD")
}

// HeadCommit returns the full hash of the commit HEAD points to.
func HeadCommit(path string) (string, error) {
	return run(path, "rev-parse", "HEAD")
}

// Checkout switches to the named branch. It gets the network timeout because
// hooks and LFS smudge filters may download content.
func Checkout(path, branch string) error {
	_, err := runWithTimeout(networkTimeout, path, "checkout", branch)
	return err
}

func Fetch(path string) error {
	_, err := runWithTimeout(networkTimeout, path, "fetch", "--quiet")
	return err
}

func Pull(path string) (string, error) {
	return runWithTimeout(networkTimeout, path, "pull", "--ff-only")
}

func Push(path string) (string, error) {
	return runWithTimeout(networkTimeout, path, "push")
}
