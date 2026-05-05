package forge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

var githubAPISem = make(chan struct{}, 5)

type GitHubProvider struct {
	apiURL     string
	tokenFile  string
	cloneProto string
	sshHost    string

	tokenOnce sync.Once
	token     string
	tokenErr  error
}

func NewGitHubProvider(cfg ProviderConfig) *GitHubProvider {
	proto := cfg.CloneProto
	if proto == "" {
		proto = "https"
	}
	return &GitHubProvider{
		apiURL:     "https://api.github.com",
		tokenFile:  cfg.TokenFile,
		cloneProto: proto,
		sshHost:    cfg.SSHHost,
	}
}

func (g *GitHubProvider) loadToken() (string, error) {
	g.tokenOnce.Do(func() {
		if g.tokenFile != "" {
			data, err := os.ReadFile(g.tokenFile)
			if err != nil {
				g.tokenErr = err
				return
			}
			g.token = strings.TrimSpace(string(data))
			return
		}
		if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
			g.token = token
			return
		}
		g.token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	})
	return g.token, g.tokenErr
}

func (g *GitHubProvider) apiGet(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, g.apiURL+path, nil)
	if err != nil {
		return nil, err
	}
	return g.apiDo(req)
}

func (g *GitHubProvider) apiDo(req *http.Request) ([]byte, error) {
	if token, err := g.loadToken(); err != nil {
		return nil, fmt.Errorf("%w: github token_file %s: %v", ErrNotAuthenticated, g.tokenFile, err)
	} else if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "grove")

	githubAPISem <- struct{}{}
	resp, err := http.DefaultClient.Do(req)
	<-githubAPISem
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: github API returned %d%s", ErrNotAuthenticated, resp.StatusCode, githubAPIMessage(body))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github API %s: %d%s", req.URL.Path, resp.StatusCode, githubAPIMessage(body))
	}
	return body, nil
}

func githubAPIMessage(body []byte) string {
	var msg struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &msg); err != nil || msg.Message == "" {
		return ""
	}
	return ": " + msg.Message
}

func (g *GitHubProvider) apiGetPaginated(path string, perPage int) ([]json.RawMessage, error) {
	var all []json.RawMessage
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for page := 1; ; page++ {
		data, err := g.apiGet(fmt.Sprintf("%s%sper_page=%d&page=%d", path, sep, perPage, page))
		if err != nil {
			return nil, err
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("parse page %d: %w", page, err)
		}
		all = append(all, batch...)
		if len(batch) < perPage {
			break
		}
	}
	return all, nil
}

func (g *GitHubProvider) ListPRs(repoFullName string) ([]model.PR, error) {
	items, err := g.apiGetPaginated(fmt.Sprintf("/repos/%s/pulls?state=open", repoFullName), 50)
	if err != nil {
		return nil, err
	}

	prs := make([]model.PR, 0, len(items))
	for _, item := range items {
		var raw struct {
			Number int                    `json:"number"`
			Title  string                 `json:"title"`
			User   struct{ Login string } `json:"user"`
			Head   struct {
				Ref string `json:"ref"`
				SHA string `json:"sha"`
			} `json:"head"`
			State     string    `json:"state"`
			UpdatedAt time.Time `json:"updated_at"`
			HTMLURL   string    `json:"html_url"`
		}
		if err := json.Unmarshal(item, &raw); err != nil {
			return nil, fmt.Errorf("parse PRs for %s: %w", repoFullName, err)
		}
		prs = append(prs, model.PR{
			Repo:           repoFullName,
			Number:         raw.Number,
			Title:          raw.Title,
			Author:         raw.User.Login,
			Branch:         raw.Head.Ref,
			State:          raw.State,
			Checks:         g.checkSummary(repoFullName, raw.Head.SHA),
			ReviewDecision: g.reviewDecision(repoFullName, raw.Number),
			UpdatedAt:      raw.UpdatedAt,
			URL:            raw.HTMLURL,
		})
	}
	return prs, nil
}

func (g *GitHubProvider) reviewDecision(repoFullName string, number int) string {
	items, err := g.apiGetPaginated(fmt.Sprintf("/repos/%s/pulls/%d/reviews", repoFullName, number), 100)
	if err != nil {
		return ""
	}
	latestByUser := map[string]string{}
	for _, item := range items {
		var review struct {
			State string `json:"state"`
			User  struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		if json.Unmarshal(item, &review) != nil || review.User.Login == "" {
			continue
		}
		latestByUser[review.User.Login] = strings.ToUpper(review.State)
	}
	hasApproval := false
	for _, state := range latestByUser {
		switch state {
		case "CHANGES_REQUESTED":
			return "CHANGES_REQUESTED"
		case "APPROVED":
			hasApproval = true
		}
	}
	if hasApproval {
		return "APPROVED"
	}
	return ""
}

func (g *GitHubProvider) checkSummary(repoFullName, sha string) string {
	if sha == "" {
		return ""
	}
	hasSignal := false
	hasPending := false

	if data, err := g.apiGet(fmt.Sprintf("/repos/%s/commits/%s/status", repoFullName, sha)); err == nil {
		var status struct {
			State string `json:"state"`
		}
		if json.Unmarshal(data, &status) == nil && status.State != "" {
			hasSignal = true
			switch strings.ToLower(status.State) {
			case "failure", "error":
				return "fail"
			case "pending":
				hasPending = true
			}
		}
	}

	if data, err := g.apiGet(fmt.Sprintf("/repos/%s/commits/%s/check-runs?per_page=100", repoFullName, sha)); err == nil {
		var resp struct {
			TotalCount int `json:"total_count"`
			CheckRuns  []struct {
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
			} `json:"check_runs"`
		}
		if json.Unmarshal(data, &resp) == nil && resp.TotalCount > 0 {
			hasSignal = true
			for _, run := range resp.CheckRuns {
				if strings.ToLower(run.Status) != "completed" {
					hasPending = true
					continue
				}
				switch strings.ToLower(run.Conclusion) {
				case "failure", "timed_out", "cancelled", "action_required", "startup_failure":
					return "fail"
				case "", "neutral", "success", "skipped":
				default:
					hasPending = true
				}
			}
		}
	}

	if hasPending {
		return "pending"
	}
	if hasSignal {
		return "pass"
	}
	return ""
}

func (g *GitHubProvider) ListIssues(repoFullName string) ([]model.Issue, error) {
	items, err := g.apiGetPaginated(fmt.Sprintf("/repos/%s/issues?state=open", repoFullName), 50)
	if err != nil {
		return nil, err
	}

	issues := make([]model.Issue, 0, len(items))
	for _, item := range items {
		var raw struct {
			Number int                    `json:"number"`
			Title  string                 `json:"title"`
			User   struct{ Login string } `json:"user"`
			State  string                 `json:"state"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
			Assignees []struct {
				Login string `json:"login"`
			} `json:"assignees"`
			Milestone *struct {
				Title string `json:"title"`
			} `json:"milestone"`
			PullRequest *struct{} `json:"pull_request"`
			CreatedAt   time.Time `json:"created_at"`
			UpdatedAt   time.Time `json:"updated_at"`
			HTMLURL     string    `json:"html_url"`
		}
		if err := json.Unmarshal(item, &raw); err != nil {
			return nil, fmt.Errorf("parse issues for %s: %w", repoFullName, err)
		}
		if raw.PullRequest != nil {
			continue
		}
		labels := make([]string, len(raw.Labels))
		for i, label := range raw.Labels {
			labels[i] = label.Name
		}
		assignees := make([]string, len(raw.Assignees))
		for i, assignee := range raw.Assignees {
			assignees[i] = assignee.Login
		}
		milestone := ""
		if raw.Milestone != nil {
			milestone = raw.Milestone.Title
		}
		issues = append(issues, model.Issue{
			Repo:      repoFullName,
			Number:    raw.Number,
			Title:     raw.Title,
			Author:    raw.User.Login,
			State:     raw.State,
			Labels:    labels,
			Assignees: assignees,
			Milestone: milestone,
			CreatedAt: raw.CreatedAt,
			UpdatedAt: raw.UpdatedAt,
			URL:       raw.HTMLURL,
		})
	}
	return issues, nil
}

func (g *GitHubProvider) ListBranches(repoFullName string) ([]model.BranchInfo, error) {
	data, err := g.apiGet(fmt.Sprintf("/repos/%s", repoFullName))
	if err != nil {
		return nil, err
	}
	var repoMeta struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(data, &repoMeta); err != nil {
		return nil, fmt.Errorf("parse repo metadata for %s: %w", repoFullName, err)
	}

	items, err := g.apiGetPaginated(fmt.Sprintf("/repos/%s/branches", repoFullName), 100)
	if err != nil {
		return nil, err
	}

	branches := make([]model.BranchInfo, 0, len(items))
	for _, item := range items {
		var raw struct {
			Name   string `json:"name"`
			Commit struct {
				SHA string `json:"sha"`
			} `json:"commit"`
		}
		if err := json.Unmarshal(item, &raw); err != nil {
			return nil, fmt.Errorf("parse branches for %s: %w", repoFullName, err)
		}
		branch := model.BranchInfo{
			Repo:      repoFullName,
			Name:      raw.Name,
			IsDefault: raw.Name == repoMeta.DefaultBranch,
		}
		if raw.Commit.SHA != "" {
			if commit := g.commitInfo(repoFullName, raw.Commit.SHA); commit != nil {
				branch.LastCommit = commit.date
				branch.Author = commit.author
			}
		}
		branches = append(branches, branch)
	}
	return branches, nil
}

type githubCommitInfo struct {
	date   time.Time
	author string
}

func (g *GitHubProvider) commitInfo(repoFullName, sha string) *githubCommitInfo {
	data, err := g.apiGet(fmt.Sprintf("/repos/%s/commits/%s", repoFullName, sha))
	if err != nil {
		return nil
	}
	var raw struct {
		Commit struct {
			Author struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
			Committer struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	info := githubCommitInfo{date: raw.Commit.Author.Date, author: raw.Commit.Author.Name}
	if info.date.IsZero() {
		info.date = raw.Commit.Committer.Date
	}
	if info.author == "" {
		info.author = raw.Commit.Committer.Name
	}
	return &info
}

func (g *GitHubProvider) ListWorkflowRuns(repoFullName string) ([]model.WorkflowRun, error) {
	workflowPaths := g.workflowPaths(repoFullName)
	data, err := g.apiGet(fmt.Sprintf("/repos/%s/actions/runs?per_page=20", repoFullName))
	if err != nil {
		return nil, err
	}

	var resp struct {
		WorkflowRuns []struct {
			ID           int64     `json:"id"`
			RunNumber    int       `json:"run_number"`
			Status       string    `json:"status"`
			Conclusion   string    `json:"conclusion"`
			Name         string    `json:"name"`
			WorkflowID   int64     `json:"workflow_id"`
			HeadBranch   string    `json:"head_branch"`
			Event        string    `json:"event"`
			CreatedAt    time.Time `json:"created_at"`
			UpdatedAt    time.Time `json:"updated_at"`
			HTMLURL      string    `json:"html_url"`
			Path         string    `json:"path"`
			DisplayTitle string    `json:"display_title"`
		} `json:"workflow_runs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse workflow runs for %s: %w", repoFullName, err)
	}

	runs := make([]model.WorkflowRun, len(resp.WorkflowRuns))
	for i, raw := range resp.WorkflowRuns {
		name := raw.Name
		if name == "" {
			name = raw.DisplayTitle
		}
		path := raw.Path
		if path == "" {
			path = workflowPaths[raw.WorkflowID]
		}
		runs[i] = model.WorkflowRun{
			Repo:         repoFullName,
			WorkflowName: name,
			WorkflowFile: path,
			Branch:       raw.HeadBranch,
			Event:        raw.Event,
			Status:       raw.Status,
			Conclusion:   raw.Conclusion,
			RunID:        raw.ID,
			Number:       raw.RunNumber,
			StartedAt:    raw.CreatedAt,
			UpdatedAt:    raw.UpdatedAt,
			URL:          raw.HTMLURL,
		}
	}
	return runs, nil
}

func (g *GitHubProvider) workflowPaths(repoFullName string) map[int64]string {
	data, err := g.apiGet(fmt.Sprintf("/repos/%s/actions/workflows?per_page=100", repoFullName))
	if err != nil {
		return nil
	}
	var resp struct {
		Workflows []struct {
			ID   int64  `json:"id"`
			Path string `json:"path"`
		} `json:"workflows"`
	}
	if json.Unmarshal(data, &resp) != nil {
		return nil
	}
	paths := make(map[int64]string, len(resp.Workflows))
	for _, workflow := range resp.Workflows {
		paths[workflow.ID] = workflow.Path
	}
	return paths
}

func (g *GitHubProvider) ListRepos(owner string, prefixes []string) ([]string, error) {
	names, err := g.listReposEndpoint("/orgs/" + owner + "/repos?type=all")
	if err != nil {
		names, err = g.listReposEndpoint("/users/" + owner + "/repos?type=owner")
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
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				filtered = append(filtered, name)
				break
			}
		}
	}
	sort.Strings(filtered)
	return filtered, nil
}

func (g *GitHubProvider) listReposEndpoint(path string) ([]string, error) {
	items, err := g.apiGetPaginated(path, 100)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		var raw struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(item, &raw) == nil && raw.Name != "" {
			names = append(names, raw.Name)
		}
	}
	return names, nil
}

func (g *GitHubProvider) CloneRepo(owner, name, targetDir string) error {
	cmd := exec.Command("git", "clone", "--quiet", g.cloneURL(owner, name), targetDir)
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

func (g *GitHubProvider) cloneURL(owner, name string) string {
	switch g.cloneProto {
	case "ssh":
		host := g.sshHost
		if host == "" {
			host = "github.com"
		}
		return fmt.Sprintf("git@%s:%s/%s.git", host, owner, name)
	default:
		return fmt.Sprintf("https://github.com/%s/%s.git", owner, name)
	}
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
