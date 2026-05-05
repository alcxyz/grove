package forge

import (
	"errors"
	"fmt"

	"github.com/alcxyz/grove/internal/model"
)

// ErrNotAuthenticated is returned when a provider reports an auth failure.
var ErrNotAuthenticated = errors.New("not authenticated")

// Provider abstracts forge-specific API calls.
type Provider interface {
	ListPRs(repoFullName string) ([]model.PR, error)
	ListIssues(repoFullName string) ([]model.Issue, error)
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
	Forge       string // "github", "forgejo"
	InstanceURL string // base URL for non-GitHub forges
	TokenFile   string // path to file containing API token
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
	default:
		return nil, fmt.Errorf("unknown forge: %q", cfg.Forge)
	}
}
