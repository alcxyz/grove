package model

import "time"

type Repo struct {
	Name       string
	Path       string
	Branch     string
	Ahead      int
	Behind     int
	Dirty      bool
	LastCommit time.Time
	LastAuthor string
	Owner      string
	Profile    string
}

type PR struct {
	Repo           string
	Number         int
	Title          string
	Author         string
	Branch         string
	State          string
	Checks         string // "pass", "fail", "pending", ""
	ReviewDecision string // "APPROVED", "CHANGES_REQUESTED", "REVIEW_REQUIRED", ""
	UpdatedAt      time.Time
	URL            string
	Profile        string
}

type BranchInfo struct {
	Repo       string
	Name       string
	IsDefault  bool
	IsMerged   bool
	LastCommit time.Time
	Author     string
	Ahead      int
	Behind     int
	Profile    string
}

type Commit struct {
	Repo     string
	RepoPath string // absolute local path — required for diff loading
	Hash     string
	Subject  string
	Author   string
	Date     time.Time
	Profile  string
}

type RepoStats struct {
	CommitCount  int
	Contributors int
}

type WorkflowRun struct {
	Repo         string // "owner/name"
	WorkflowName string
	WorkflowFile string // relative path, e.g. ".github/workflows/ci.yml"
	Branch       string
	Event        string // "push", "pull_request", "schedule", "workflow_dispatch", etc.
	Status       string // "queued", "in_progress", "completed"
	Conclusion   string // "success", "failure", "cancelled", "skipped", "" when not completed
	RunID        int64
	Number       int
	StartedAt    time.Time
	UpdatedAt    time.Time
	URL          string
	Profile      string
}
