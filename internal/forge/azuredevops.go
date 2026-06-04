package forge

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

type AzureDevOpsProvider struct {
	instanceURL string
	project     string
	cloneProto  string
}

func NewAzureDevOpsProvider(cfg ProviderConfig) (*AzureDevOpsProvider, error) {
	if cfg.Project == "" {
		return nil, fmt.Errorf("azuredevops provider requires project")
	}
	proto := cfg.CloneProto
	if proto == "" {
		proto = "https"
	}
	return &AzureDevOpsProvider{
		instanceURL: strings.TrimRight(cfg.InstanceURL, "/"),
		project:     cfg.Project,
		cloneProto:  proto,
	}, nil
}

func (a *AzureDevOpsProvider) ListPRs(repoFullName string) ([]model.PR, error) {
	owner, repo := splitRepoFullName(repoFullName)
	data, err := a.azJSON(
		"repos", "pr", "list",
		"--organization", a.organizationURL(owner),
		"--project", a.project,
		"--repository", repo,
		"--status", "active",
		"--top", "100",
		"--include-links",
	)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		PullRequestID int       `json:"pullRequestId"`
		Title         string    `json:"title"`
		Status        string    `json:"status"`
		SourceRefName string    `json:"sourceRefName"`
		CreationDate  time.Time `json:"creationDate"`
		CreatedBy     struct {
			DisplayName string `json:"displayName"`
			UniqueName  string `json:"uniqueName"`
		} `json:"createdBy"`
		Links struct {
			Web struct {
				Href string `json:"href"`
			} `json:"web"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse Azure DevOps PRs for %s: %w", repoFullName, err)
	}

	prs := make([]model.PR, 0, len(raw))
	for _, r := range raw {
		author := r.CreatedBy.DisplayName
		if author == "" {
			author = r.CreatedBy.UniqueName
		}
		prs = append(prs, model.PR{
			Repo:      repoFullName,
			Number:    r.PullRequestID,
			Title:     r.Title,
			Author:    author,
			Branch:    strings.TrimPrefix(r.SourceRefName, "refs/heads/"),
			State:     r.Status,
			UpdatedAt: r.CreationDate,
			URL:       r.Links.Web.Href,
		})
	}
	return prs, nil
}

func (a *AzureDevOpsProvider) ListIssues(repoFullName string) ([]model.Issue, error) {
	// Azure Boards work items are not a direct GitHub/Forgejo issue equivalent.
	// Keep this conservative until grove has an explicit work-item model.
	return nil, nil
}

func (a *AzureDevOpsProvider) ListMilestones(repoFullName string) ([]model.Milestone, error) {
	// Azure DevOps iterations are project-scoped Azure Boards concepts, not
	// repository milestones. Keep this empty until grove models work items.
	return nil, nil
}

func (a *AzureDevOpsProvider) ListBranches(repoFullName string) ([]model.BranchInfo, error) {
	owner, repo := splitRepoFullName(repoFullName)
	repoMeta, err := a.repoMetadata(owner, repo)
	if err != nil {
		return nil, err
	}
	data, err := a.azJSON(
		"repos", "ref", "list",
		"--organization", a.organizationURL(owner),
		"--project", a.project,
		"--repository", repo,
		"--filter", "heads/",
	)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse Azure DevOps branches for %s: %w", repoFullName, err)
	}

	defaultBranch := strings.TrimPrefix(repoMeta.DefaultBranch, "refs/heads/")
	branches := make([]model.BranchInfo, 0, len(raw))
	for _, r := range raw {
		name := strings.TrimPrefix(r.Name, "refs/heads/")
		if name == "" {
			continue
		}
		branches = append(branches, model.BranchInfo{
			Repo:      repoFullName,
			Name:      name,
			IsDefault: name == defaultBranch,
		})
	}
	return branches, nil
}

func (a *AzureDevOpsProvider) ListWorkflowRuns(repoFullName string) ([]model.WorkflowRun, error) {
	// Azure Pipelines run listing is project-scoped in the CLI. Returning all
	// project runs for every repo would duplicate unrelated CI data.
	return nil, nil
}

func (a *AzureDevOpsProvider) ListRepos(owner string, prefixes []string) ([]string, error) {
	data, err := a.azJSON(
		"repos", "list",
		"--organization", a.organizationURL(owner),
		"--project", a.project,
	)
	if err != nil {
		return nil, fmt.Errorf("listing Azure DevOps repos for %s/%s: %w", owner, a.project, err)
	}

	var raw []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse Azure DevOps repos for %s/%s: %w", owner, a.project, err)
	}

	var names []string
	for _, r := range raw {
		if r.Name != "" {
			names = append(names, r.Name)
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

func (a *AzureDevOpsProvider) CloneRepo(owner, name, targetDir string) error {
	meta, err := a.repoMetadata(owner, name)
	if err != nil {
		return err
	}
	cloneURL := meta.RemoteURL
	if a.cloneProto == "ssh" && meta.SSHURL != "" {
		cloneURL = meta.SSHURL
	}
	if cloneURL == "" {
		cloneURL = a.RepoURL(owner, name)
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

func (a *AzureDevOpsProvider) RepoURL(owner, repo string) string {
	return fmt.Sprintf("%s/%s/_git/%s", a.organizationURL(owner), pathEscape(a.project), pathEscape(repo))
}

func (a *AzureDevOpsProvider) CommitURL(owner, repo, hash string) string {
	return fmt.Sprintf("%s/commit/%s", a.RepoURL(owner, repo), pathEscape(hash))
}

func (a *AzureDevOpsProvider) BranchURL(owner, repo, branch string) string {
	return fmt.Sprintf("%s?version=GB%s", a.RepoURL(owner, repo), url.QueryEscape(branch))
}

type azureDevOpsRepoMeta struct {
	Name          string `json:"name"`
	RemoteURL     string `json:"remoteUrl"`
	SSHURL        string `json:"sshUrl"`
	WebURL        string `json:"webUrl"`
	DefaultBranch string `json:"defaultBranch"`
}

func (a *AzureDevOpsProvider) repoMetadata(owner, repo string) (azureDevOpsRepoMeta, error) {
	data, err := a.azJSON(
		"repos", "show",
		"--organization", a.organizationURL(owner),
		"--project", a.project,
		"--repository", repo,
	)
	if err != nil {
		return azureDevOpsRepoMeta{}, err
	}
	var meta azureDevOpsRepoMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return azureDevOpsRepoMeta{}, fmt.Errorf("parse Azure DevOps repo %s/%s/%s: %w", owner, a.project, repo, err)
	}
	return meta, nil
}

func (a *AzureDevOpsProvider) azJSON(args ...string) ([]byte, error) {
	fullArgs := append([]string{}, args...)
	fullArgs = append(fullArgs, "--output", "json", "--only-show-errors")
	cmd := exec.Command("az", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return nil, err
		}
		return nil, azureDevOpsCLIError(msg)
	}
	return out, nil
}

func (a *AzureDevOpsProvider) organizationURL(owner string) string {
	if a.instanceURL != "" {
		return a.instanceURL
	}
	return "https://dev.azure.com/" + owner
}

func splitRepoFullName(repoFullName string) (string, string) {
	parts := strings.Split(repoFullName, "/")
	if len(parts) < 2 {
		return "", repoFullName
	}
	return parts[0], parts[len(parts)-1]
}

func azureDevOpsCLIError(msg string) error {
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "az login") ||
		strings.Contains(lower, "devops login") ||
		strings.Contains(lower, "sign in") ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "unauthorized") {
		return fmt.Errorf("%w: azure devops CLI: %s", ErrNotAuthenticated, msg)
	}
	return fmt.Errorf("azure devops CLI: %s", msg)
}

func pathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "+", "%20")
}
