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

// NewProvider returns the Provider implementation for the named forge.
// An empty forgeName defaults to "github".
func NewProvider(forgeName string) (Provider, error) {
	switch forgeName {
	case "github", "":
		return NewGitHubProvider(), nil
	default:
		return nil, fmt.Errorf("unknown forge: %q", forgeName)
	}
}
