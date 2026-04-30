package forge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/alcxyz/grove/internal/gh"
	"github.com/alcxyz/grove/internal/model"
)

type GitHubProvider struct{}

func NewGitHubProvider() *GitHubProvider { return &GitHubProvider{} }

func (g *GitHubProvider) ListPRs(repoFullName string) ([]model.PR, error) {
	prs, err := gh.ListPRs(repoFullName)
	if err != nil && errors.Is(err, gh.ErrNotLoggedIn) {
		return nil, fmt.Errorf("%w: %v", ErrNotAuthenticated, err)
	}
	return prs, err
}

func (g *GitHubProvider) ListIssues(repoFullName string) ([]model.Issue, error) {
	issues, err := gh.ListIssues(repoFullName)
	if err != nil && errors.Is(err, gh.ErrNotLoggedIn) {
		return nil, fmt.Errorf("%w: %v", ErrNotAuthenticated, err)
	}
	return issues, err
}

func (g *GitHubProvider) ListBranches(repoFullName string) ([]model.BranchInfo, error) {
	branches, err := gh.ListBranches(repoFullName)
	if err != nil && errors.Is(err, gh.ErrNotLoggedIn) {
		return nil, fmt.Errorf("%w: %v", ErrNotAuthenticated, err)
	}
	return branches, err
}

func (g *GitHubProvider) ListWorkflowRuns(repoFullName string) ([]model.WorkflowRun, error) {
	runs, err := gh.ListWorkflowRuns(repoFullName)
	if err != nil && errors.Is(err, gh.ErrNotLoggedIn) {
		return nil, fmt.Errorf("%w: %v", ErrNotAuthenticated, err)
	}
	return runs, err
}

func (g *GitHubProvider) ListRepos(owner string, prefixes []string) ([]string, error) {
	names, err := ghAPIListRepos("orgs/" + owner + "/repos")
	if err != nil {
		names, err = ghAPIListRepos("users/" + owner + "/repos")
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

func (g *GitHubProvider) CloneRepo(owner, name, targetDir string) error {
	cmd := exec.Command("gh", "repo", "clone", owner+"/"+name, targetDir, "--", "--quiet")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

func (g *GitHubProvider) RepoURL(owner, repo string) string {
	return fmt.Sprintf("https://github.com/%s/%s", owner, repo)
}

func (g *GitHubProvider) CommitURL(owner, repo, hash string) string {
	return fmt.Sprintf("https://github.com/%s/%s/commit/%s", owner, repo, hash)
}

func (g *GitHubProvider) BranchURL(owner, repo, branch string) string {
	return fmt.Sprintf("https://github.com/%s/%s/tree/%s", owner, repo, branch)
}

// ghAPIListRepos paginates through a GitHub repos API endpoint and returns all
// repo names.
func ghAPIListRepos(endpoint string) ([]string, error) {
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
