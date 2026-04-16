package gh

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

// ErrNotLoggedIn is returned when gh reports an authentication failure.
var ErrNotLoggedIn = errors.New("not logged in — run: gh auth login")

// sem limits concurrent gh CLI invocations to avoid hitting GitHub rate limits.
var sem = make(chan struct{}, 5)

func acquire() { sem <- struct{}{} }
func release() { <-sem }

// wrapErr inspects the stderr of a failed gh command and returns a friendly
// ErrNotLoggedIn sentinel when it looks like an auth problem.
func wrapErr(repo string, err error, stderr []byte) error {
	s := strings.ToLower(string(stderr))
	if strings.Contains(s, "not logged in") ||
		strings.Contains(s, "auth login") ||
		strings.Contains(s, "authentication required") ||
		strings.Contains(s, "401") ||
		strings.Contains(s, "saml") {
		log.Printf("gh auth error for %s: %s", repo, strings.TrimSpace(string(stderr)))
		return fmt.Errorf("%s: %w", repo, ErrNotLoggedIn)
	}
	log.Printf("gh error for %s: %v — %s", repo, err, strings.TrimSpace(string(stderr)))
	return fmt.Errorf("%s: %w", repo, err)
}

type ghAuthor struct {
	Login string `json:"login"`
}

func ListPRs(repoFullName string) ([]model.PR, error) {
	acquire()
	defer release()

	cmd := exec.Command("gh", "pr", "list",
		"--repo", repoFullName,
		"--state", "open",
		"--json", "number,title,author,headRefName,state,updatedAt,url,reviewDecision",
		"--limit", "50",
	)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		var stderr []byte
		if errors.As(err, &ee) {
			stderr = ee.Stderr
		}
		return nil, wrapErr(repoFullName, err, stderr)
	}

	var raw []struct {
		Number         int       `json:"number"`
		Title          string    `json:"title"`
		Author         ghAuthor  `json:"author"`
		HeadRef        string    `json:"headRefName"`
		State          string    `json:"state"`
		UpdatedAt      time.Time `json:"updatedAt"`
		URL            string    `json:"url"`
		ReviewDecision string    `json:"reviewDecision"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh pr list %s: parse: %w", repoFullName, err)
	}

	prs := make([]model.PR, len(raw))
	for i, r := range raw {
		prs[i] = model.PR{
			Repo:           repoFullName,
			Number:         r.Number,
			Title:          r.Title,
			Author:         r.Author.Login,
			Branch:         r.HeadRef,
			State:          r.State,
			ReviewDecision: r.ReviewDecision,
			UpdatedAt:      r.UpdatedAt,
			URL:            r.URL,
		}
	}
	return prs, nil
}

// branchQuery fetches branch names, last commit date/author, and the default
// branch in a single GraphQL call — far more efficient than N REST calls.
const branchQuery = `query($owner:String!,$name:String!){repository(owner:$owner,name:$name){defaultBranchRef{name}refs(refPrefix:"refs/heads/",first:100,orderBy:{field:TAG_COMMIT_DATE,direction:DESC}){nodes{name target{... on Commit{committedDate author{name}}}}}}}`

func ListBranches(repoFullName string) ([]model.BranchInfo, error) {
	acquire()
	defer release()

	owner, name, _ := strings.Cut(repoFullName, "/")
	cmd := exec.Command("gh", "api", "graphql",
		"-f", "query="+branchQuery,
		"-f", "owner="+owner,
		"-f", "name="+name,
	)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		var stderr []byte
		if errors.As(err, &ee) {
			stderr = ee.Stderr
		}
		return nil, wrapErr(repoFullName, err, stderr)
	}

	var result struct {
		Data struct {
			Repository struct {
				DefaultBranchRef *struct {
					Name string `json:"name"`
				} `json:"defaultBranchRef"`
				Refs struct {
					Nodes []struct {
						Name   string `json:"name"`
						Target struct {
							CommittedDate time.Time `json:"committedDate"`
							Author        struct {
								Name string `json:"name"`
							} `json:"author"`
						} `json:"target"`
					} `json:"nodes"`
				} `json:"refs"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("gh graphql branches %s: parse: %w", repoFullName, err)
	}

	defaultBranch := ""
	if result.Data.Repository.DefaultBranchRef != nil {
		defaultBranch = result.Data.Repository.DefaultBranchRef.Name
	}

	nodes := result.Data.Repository.Refs.Nodes
	branches := make([]model.BranchInfo, len(nodes))
	for i, n := range nodes {
		branches[i] = model.BranchInfo{
			Repo:       repoFullName,
			Name:       n.Name,
			IsDefault:  n.Name == defaultBranch,
			LastCommit: n.Target.CommittedDate,
			Author:     n.Target.Author.Name,
		}
	}
	return branches, nil
}
