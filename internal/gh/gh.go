// Package gh wraps the gh CLI to fetch GitHub data.
//
// All outbound calls go through a shared semaphore (sem) that caps concurrent
// gh invocations at 5.  This keeps grove well within GitHub's REST API rate
// limit of 5 000 authenticated requests per hour: with a semaphore of 5 and
// typical round-trip times of 200–500 ms, the theoretical maximum is around
// 36 000–90 000 requests per hour, but in practice grove only fetches on
// startup and on explicit refresh, so the actual call volume is far lower.
//
// The cap of 5 is intentionally conservative so that grove does not monopolise
// a user's rate-limit budget when they also have other tools (GitHub CLI, CI
// systems) running concurrently.
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

// sem is a counting semaphore that limits concurrent gh CLI invocations to 5.
// See the package comment for the reasoning behind this value.
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

// summarizeChecks maps a PR's statusCheckRollup array to a single "pass" /
// "fail" / "pending" / "" value.  Each element may be a CheckRun (has
// status/conclusion) or a legacy StatusContext (has state).
func summarizeChecks(rollup []struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}) string {
	if len(rollup) == 0 {
		return ""
	}
	hasPending := false
	for _, c := range rollup {
		// CheckRun: use status/conclusion pair
		if c.Status != "" && strings.ToUpper(c.Status) != "COMPLETED" {
			hasPending = true
			continue
		}
		conc := c.Conclusion
		if conc == "" {
			conc = c.State // StatusContext fallback
		}
		switch strings.ToUpper(conc) {
		case "FAILURE", "TIMED_OUT", "ERROR", "STARTUP_FAILURE":
			return "fail"
		case "PENDING", "IN_PROGRESS", "EXPECTED", "QUEUED", "WAITING":
			hasPending = true
		}
	}
	if hasPending {
		return "pending"
	}
	return "pass"
}

func ListPRs(repoFullName string) ([]model.PR, error) {
	acquire()
	defer release()

	cmd := exec.Command("gh", "pr", "list",
		"--repo", repoFullName,
		"--state", "open",
		"--json", "number,title,author,headRefName,state,updatedAt,url,reviewDecision,statusCheckRollup",
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
		StatusCheckRollup []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			State      string `json:"state"`
		} `json:"statusCheckRollup"`
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
			Checks:         summarizeChecks(r.StatusCheckRollup),
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

func ListWorkflowRuns(repoFullName string) ([]model.WorkflowRun, error) {
	acquire()
	defer release()

	cmd := exec.Command("gh", "run", "list",
		"--repo", repoFullName,
		"--limit", "20",
		"--json", "databaseId,number,status,conclusion,workflowName,headBranch,event,startedAt,updatedAt,url",
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
		DatabaseID   int64     `json:"databaseId"`
		Number       int       `json:"number"`
		Status       string    `json:"status"`
		Conclusion   string    `json:"conclusion"`
		WorkflowName string    `json:"workflowName"`
		HeadBranch   string    `json:"headBranch"`
		Event        string    `json:"event"`
		StartedAt    time.Time `json:"startedAt"`
		UpdatedAt    time.Time `json:"updatedAt"`
		URL          string    `json:"url"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh run list %s: parse: %w", repoFullName, err)
	}

	// Fetch workflow name→path mapping (one call per repo, no extra semaphore needed).
	wfPaths := map[string]string{}
	wfCmd := exec.Command("gh", "workflow", "list", "--repo", repoFullName, "--json", "name,path", "--limit", "50")
	if wfOut, err := wfCmd.Output(); err == nil {
		var wfs []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if json.Unmarshal(wfOut, &wfs) == nil {
			for _, wf := range wfs {
				wfPaths[wf.Name] = wf.Path
			}
		}
	}

	runs := make([]model.WorkflowRun, len(raw))
	for i, r := range raw {
		runs[i] = model.WorkflowRun{
			Repo:         repoFullName,
			WorkflowName: r.WorkflowName,
			WorkflowFile: wfPaths[r.WorkflowName],
			Branch:       r.HeadBranch,
			Event:        r.Event,
			Status:       r.Status,
			Conclusion:   r.Conclusion,
			RunID:        r.DatabaseID,
			Number:       r.Number,
			StartedAt:    r.StartedAt,
			UpdatedAt:    r.UpdatedAt,
			URL:          r.URL,
		}
	}
	return runs, nil
}

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
