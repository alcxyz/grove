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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, nil, fmt.Errorf("%s timed out after %s", name, cliTimeout)
	}
	return out, stderr.Bytes(), err
}
