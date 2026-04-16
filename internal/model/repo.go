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
}

type Commit struct {
	Repo     string
	RepoPath string // absolute local path — required for diff loading
	Hash     string
	Subject  string
	Author   string
	Date     time.Time
}

type RepoStats struct {
	CommitCount  int
	Contributors int
}
