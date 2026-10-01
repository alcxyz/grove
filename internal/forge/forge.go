package forge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

// ErrNotAuthenticated is returned when a provider reports an auth failure.
var ErrNotAuthenticated = errors.New("not authenticated")

// ErrRateLimited is returned when a provider rejects a request because an API
// rate limit has been exhausted.
var ErrRateLimited = errors.New("rate limited")

// Provider abstracts forge-specific API calls.
type Provider interface {
	ListPRs(repoFullName string) ([]model.PR, error)
	ListIssues(repoFullName string) ([]model.Issue, error)
	ListMilestones(repoFullName string) ([]model.Milestone, error)
	ListBranches(repoFullName string) ([]model.BranchInfo, error)
	ListWorkflowRuns(repoFullName string) ([]model.WorkflowRun, error)

	ListRepos(owner string, prefixes []string) ([]string, error)
	CloneRepo(owner, name, targetDir string) error

	RepoURL(owner, repo string) string
	CommitURL(owner, repo, hash string) string
	BranchURL(owner, repo, branch string) string
}

// ProviderConfig holds the fields needed to construct a Provider.
type ProviderConfig struct {
	Forge       string // "github", "forgejo", "azuredevops"
	InstanceURL string // base URL for non-GitHub forges
	Project     string // Azure DevOps project for azuredevops
	TokenFile   string // path to file containing API token
	AuthMode    string // provider auth mode: "token", "gh" for GitHub, or "tea" for Forgejo
	CloneProto  string // "https" or "ssh"
	SSHHost     string // optional SSH clone host when it differs from InstanceURL host
}

// NewProvider returns the Provider implementation for the given config.
// An empty Forge defaults to "github".
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Forge {
	case "github", "":
		return NewGitHubProvider(cfg), nil
	case "forgejo":
		return NewForgejoProvider(cfg)
	case "azuredevops":
		return NewAzureDevOpsProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown forge: %q", cfg.Forge)
	}
}

// httpClient bounds every forge API request so a stalled connection cannot
// hang a refresh indefinitely.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// cliTimeout bounds forge CLI invocations (gh, tea, az) used for API reads.
const cliTimeout = 2 * time.Minute

// runCLI runs a forge CLI command bounded by cliTimeout and returns its
// stdout and stderr separately.
func runCLI(name string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	// Wrapper scripts (az is one) can leave a child holding the output pipes
	// after the timeout kills the wrapper; stop waiting for them shortly after.
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, nil, fmt.Errorf("%s timed out after %s", name, cliTimeout)
	}
	return out, stderr.Bytes(), err
}

// unavailableProvider stands in for a remote whose provider could not be
// built, so each repo that resolves to it reports the reason.
type unavailableProvider struct {
	err error
}

// Unavailable returns a Provider whose calls all fail with err.
func Unavailable(err error) Provider {
	return unavailableProvider{err: err}
}

func (u unavailableProvider) ListPRs(string) ([]model.PR, error)       { return nil, u.err }
func (u unavailableProvider) ListIssues(string) ([]model.Issue, error) { return nil, u.err }
func (u unavailableProvider) ListMilestones(string) ([]model.Milestone, error) {
	return nil, u.err
}
func (u unavailableProvider) ListBranches(string) ([]model.BranchInfo, error) { return nil, u.err }
func (u unavailableProvider) ListWorkflowRuns(string) ([]model.WorkflowRun, error) {
	return nil, u.err
}
func (u unavailableProvider) ListRepos(string, []string) ([]string, error) { return nil, u.err }
func (u unavailableProvider) CloneRepo(string, string, string) error       { return u.err }
func (u unavailableProvider) RepoURL(string, string) string                { return "" }
func (u unavailableProvider) CommitURL(string, string, string) string      { return "" }
func (u unavailableProvider) BranchURL(string, string, string) string      { return "" }
