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

type ForgejoProvider struct {
	baseURL    string // e.g. "https://git.alc.xyz"
	tokenFile  string
	cloneProto string // "https" or "ssh"

	tokenOnce sync.Once
	token     string
}

func NewForgejoProvider(cfg ProviderConfig) (*ForgejoProvider, error) {
	if cfg.InstanceURL == "" {
		return nil, fmt.Errorf("forgejo provider requires instance_url")
	}
	proto := cfg.CloneProto
	if proto == "" {
		proto = "https"
	}
	return &ForgejoProvider{
		baseURL:    strings.TrimRight(cfg.InstanceURL, "/"),
		tokenFile:  cfg.TokenFile,
		cloneProto: proto,
	}, nil
}

func (f *ForgejoProvider) loadToken() string {
	f.tokenOnce.Do(func() {
		if f.tokenFile == "" {
			return
		}
		data, err := os.ReadFile(f.tokenFile)
		if err != nil {
			return
		}
		f.token = strings.TrimSpace(string(data))
	})
	return f.token
}

func (f *ForgejoProvider) apiGet(path string) ([]byte, error) {
	url := f.baseURL + "/api/v1" + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if tok := f.loadToken(); tok != "" {
		req.Header.Set("Authorization", "token "+tok)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: %s returned %d", ErrNotAuthenticated, f.baseURL, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (f *ForgejoProvider) apiGetPaginated(pathFmt string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	for page := 1; ; page++ {
		path := fmt.Sprintf("%s?page=%d&limit=50", pathFmt, page)
		data, err := f.apiGet(path)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("parse page %d: %w", page, err)
		}
		all = append(all, batch...)
		if len(batch) < 50 {
			break
		}
	}
	return all, nil
}

func (f *ForgejoProvider) ListPRs(repoFullName string) ([]model.PR, error) {
	path := fmt.Sprintf("/repos/%s/pulls?state=open", repoFullName)
	data, err := f.apiGet(path)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		User      struct{ Login string } `json:"user"`
		Head      struct{ Ref string } `json:"head"`
		State     string    `json:"state"`
		UpdatedAt time.Time `json:"updated_at"`
		HTMLURL   string    `json:"html_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse PRs for %s: %w", repoFullName, err)
	}

	prs := make([]model.PR, len(raw))
	for i, r := range raw {
		prs[i] = model.PR{
			Repo:      repoFullName,
			Number:    r.Number,
			Title:     r.Title,
			Author:    r.User.Login,
			Branch:    r.Head.Ref,
			State:     r.State,
			UpdatedAt: r.UpdatedAt,
			URL:       r.HTMLURL,
		}
	}
	return prs, nil
}

func (f *ForgejoProvider) ListIssues(repoFullName string) ([]model.Issue, error) {
	path := fmt.Sprintf("/repos/%s/issues?state=open&type=issues", repoFullName)
	data, err := f.apiGet(path)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		User      struct{ Login string } `json:"user"`
		State     string    `json:"state"`
		Labels    []struct{ Name string } `json:"labels"`
		Assignees []struct{ Login string } `json:"assignees"`
		Milestone *struct{ Title string } `json:"milestone"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		HTMLURL   string    `json:"html_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse issues for %s: %w", repoFullName, err)
	}

	issues := make([]model.Issue, len(raw))
	for i, r := range raw {
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		assignees := make([]string, len(r.Assignees))
		for j, a := range r.Assignees {
			assignees[j] = a.Login
		}
		milestone := ""
		if r.Milestone != nil {
			milestone = r.Milestone.Title
		}
		issues[i] = model.Issue{
			Repo:      repoFullName,
			Number:    r.Number,
			Title:     r.Title,
			Author:    r.User.Login,
			State:     r.State,
			Labels:    labels,
			Assignees: assignees,
			Milestone: milestone,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			URL:       r.HTMLURL,
		}
	}
	return issues, nil
}

func (f *ForgejoProvider) ListBranches(repoFullName string) ([]model.BranchInfo, error) {
	// Get default branch from repo metadata.
	repoData, err := f.apiGet(fmt.Sprintf("/repos/%s", repoFullName))
	if err != nil {
		return nil, err
	}
	var repoMeta struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(repoData, &repoMeta); err != nil {
		return nil, fmt.Errorf("parse repo metadata for %s: %w", repoFullName, err)
	}

	// Get branches.
	items, err := f.apiGetPaginated(fmt.Sprintf("/repos/%s/branches", repoFullName))
	if err != nil {
		return nil, err
	}

	branches := make([]model.BranchInfo, len(items))
	for i, item := range items {
		var b struct {
			Name   string `json:"name"`
			Commit struct {
				Timestamp time.Time `json:"timestamp"`
				Author    *struct {
					Name string `json:"name"`
				} `json:"author"`
				Message string `json:"message"`
			} `json:"commit"`
		}
		if err := json.Unmarshal(item, &b); err != nil {
			continue
		}
		author := ""
		if b.Commit.Author != nil {
			author = b.Commit.Author.Name
		}
		branches[i] = model.BranchInfo{
			Repo:       repoFullName,
			Name:       b.Name,
			IsDefault:  b.Name == repoMeta.DefaultBranch,
			LastCommit: b.Commit.Timestamp,
			Author:     author,
		}
	}
	return branches, nil
}

func (f *ForgejoProvider) ListWorkflowRuns(repoFullName string) ([]model.WorkflowRun, error) {
	path := fmt.Sprintf("/repos/%s/actions/runs", repoFullName)
	data, err := f.apiGet(path)
	if err != nil {
		// Actions may not be enabled — return nil, not error.
		return nil, nil
	}

	var resp struct {
		WorkflowRuns []struct {
			ID         int64     `json:"id"`
			Status     string    `json:"status"`
			Conclusion string    `json:"conclusion"`
			HeadBranch string    `json:"head_branch"`
			Event      string    `json:"event"`
			Name       string    `json:"name"`
			RunNumber  int       `json:"run_number"`
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			HTMLURL    string    `json:"html_url"`
			Path       string    `json:"path"`
		} `json:"workflow_runs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		// Might be an older Forgejo without Actions — not an error.
		return nil, nil
	}

	runs := make([]model.WorkflowRun, len(resp.WorkflowRuns))
	for i, r := range resp.WorkflowRuns {
		runs[i] = model.WorkflowRun{
			Repo:         repoFullName,
			WorkflowName: r.Name,
			WorkflowFile: r.Path,
			Branch:       r.HeadBranch,
			Event:        r.Event,
			Status:       r.Status,
			Conclusion:   r.Conclusion,
			RunID:        r.ID,
			Number:       r.RunNumber,
			StartedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
			URL:          r.HTMLURL,
		}
	}
	return runs, nil
}

func (f *ForgejoProvider) ListRepos(owner string, prefixes []string) ([]string, error) {
	// Try org endpoint first, fall back to user.
	items, err := f.apiGetPaginated(fmt.Sprintf("/orgs/%s/repos", owner))
	if err != nil {
		items, err = f.apiGetPaginated(fmt.Sprintf("/users/%s/repos", owner))
		if err != nil {
			return nil, fmt.Errorf("listing repos for %s: %w", owner, err)
		}
	}

	var names []string
	for _, item := range items {
		var r struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(item, &r) == nil {
			names = append(names, r.Name)
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

func (f *ForgejoProvider) CloneRepo(owner, name, targetDir string) error {
	var cloneURL string
	switch f.cloneProto {
	case "ssh":
		host := strings.TrimPrefix(strings.TrimPrefix(f.baseURL, "https://"), "http://")
		cloneURL = fmt.Sprintf("git@%s:%s/%s.git", host, owner, name)
	default:
		cloneURL = fmt.Sprintf("%s/%s/%s.git", f.baseURL, owner, name)
	}
	cmd := exec.Command("git", "clone", "--quiet", cloneURL, targetDir)
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

func (f *ForgejoProvider) RepoURL(owner, repo string) string {
	return fmt.Sprintf("%s/%s/%s", f.baseURL, owner, repo)
}

func (f *ForgejoProvider) CommitURL(owner, repo, hash string) string {
	return fmt.Sprintf("%s/%s/%s/commit/%s", f.baseURL, owner, repo, hash)
}

func (f *ForgejoProvider) BranchURL(owner, repo, branch string) string {
	return fmt.Sprintf("%s/%s/%s/src/branch/%s", f.baseURL, owner, repo, branch)
}
