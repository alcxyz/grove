package ui

import (
	"sort"
	"strings"

	"github.com/alcxyz/grove/internal/model"
)

// RepoGroup is already defined in dashboard.go

type PRGroup struct {
	Name     string
	PRs      []model.PR
	StartIdx int
}

type BranchGroup struct {
	Name     string
	Branches []model.BranchInfo
	StartIdx int
}

type CommitGroup struct {
	Name     string
	Commits  []model.Commit
	StartIdx int
}

// repoShortName strips the org prefix from a full repo name like "my-org/my-repo".
func repoShortName(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}

func BuildPRGroups(prs []model.PR, groupFor func(string) string, groupOrder func(string) int) []PRGroup {
	var groups []PRGroup
	idx := map[string]int{}
	for _, pr := range prs {
		gname := groupFor(repoShortName(pr.Repo))
		gi, exists := idx[gname]
		if !exists {
			gi = len(groups)
			idx[gname] = gi
			groups = append(groups, PRGroup{Name: gname})
		}
		groups[gi].PRs = append(groups[gi].PRs, pr)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groupOrder(groups[i].Name) < groupOrder(groups[j].Name)
	})
	si := 0
	for i := range groups {
		groups[i].StartIdx = si
		si += len(groups[i].PRs)
	}
	return groups
}

func BuildBranchGroups(branches []model.BranchInfo, groupFor func(string) string, groupOrder func(string) int) []BranchGroup {
	var groups []BranchGroup
	idx := map[string]int{}
	for _, br := range branches {
		gname := groupFor(repoShortName(br.Repo))
		gi, exists := idx[gname]
		if !exists {
			gi = len(groups)
			idx[gname] = gi
			groups = append(groups, BranchGroup{Name: gname})
		}
		groups[gi].Branches = append(groups[gi].Branches, br)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groupOrder(groups[i].Name) < groupOrder(groups[j].Name)
	})
	si := 0
	for i := range groups {
		groups[i].StartIdx = si
		si += len(groups[i].Branches)
	}
	return groups
}

func BuildCommitGroups(commits []model.Commit, groupFor func(string) string, groupOrder func(string) int) []CommitGroup {
	var groups []CommitGroup
	idx := map[string]int{}
	for _, c := range commits {
		gname := groupFor(c.Repo)
		gi, exists := idx[gname]
		if !exists {
			gi = len(groups)
			idx[gname] = gi
			groups = append(groups, CommitGroup{Name: gname})
		}
		groups[gi].Commits = append(groups[gi].Commits, c)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groupOrder(groups[i].Name) < groupOrder(groups[j].Name)
	})
	si := 0
	for i := range groups {
		groups[i].StartIdx = si
		si += len(groups[i].Commits)
	}
	return groups
}

// CursorLine returns the visual line number (0-indexed, not counting the
// column header) of the item at flatIdx == cursor. Used to drive scrolling.
// Layout per group: [blank-sep (if gi>0)] [group header] [items…]

func RepoCursorLine(groups []RepoGroup, cursor int) int {
	vl := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			vl++ // blank separator
		}
		if g.Name != "" {
			vl++ // group header
		}
		for i := range g.Repos {
			if g.StartIdx+i == cursor {
				return vl
			}
			vl++
		}
	}
	return 0
}

func PRCursorLine(groups []PRGroup, cursor int) int {
	vl := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			vl++
		}
		if g.Name != "" {
			vl++
		}
		for i := range g.PRs {
			if g.StartIdx+i == cursor {
				return vl
			}
			vl++
		}
	}
	return 0
}

func BranchCursorLine(groups []BranchGroup, cursor int) int {
	vl := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			vl++
		}
		if g.Name != "" {
			vl++
		}
		for i := range g.Branches {
			if g.StartIdx+i == cursor {
				return vl
			}
			vl++
		}
	}
	return 0
}

func CommitCursorLine(groups []CommitGroup, cursor int) int {
	vl := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			vl++
		}
		if g.Name != "" {
			vl++
		}
		for i := range g.Commits {
			if g.StartIdx+i == cursor {
				return vl
			}
			vl++
		}
	}
	return 0
}

// *IndexAtVL functions return the flat item index at the given visual line
// in the grouped view, or -1 if the line falls on a blank separator or
// group header (i.e. not a selectable item).

func RepoIndexAtVL(groups []RepoGroup, vl int) int {
	cur := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			cur++
		}
		if g.Name != "" {
			cur++
		}
		for i := range g.Repos {
			if cur == vl {
				return g.StartIdx + i
			}
			cur++
		}
	}
	return -1
}

func PRIndexAtVL(groups []PRGroup, vl int) int {
	cur := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			cur++
		}
		if g.Name != "" {
			cur++
		}
		for i := range g.PRs {
			if cur == vl {
				return g.StartIdx + i
			}
			cur++
		}
	}
	return -1
}

func BranchIndexAtVL(groups []BranchGroup, vl int) int {
	cur := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			cur++
		}
		if g.Name != "" {
			cur++
		}
		for i := range g.Branches {
			if cur == vl {
				return g.StartIdx + i
			}
			cur++
		}
	}
	return -1
}

func CommitIndexAtVL(groups []CommitGroup, vl int) int {
	cur := 0
	for gi, g := range groups {
		if gi > 0 && g.Name != "" {
			cur++
		}
		if g.Name != "" {
			cur++
		}
		for i := range g.Commits {
			if cur == vl {
				return g.StartIdx + i
			}
			cur++
		}
	}
	return -1
}

// GroupStartIdxs extracts the StartIdx from any group slice — used for
// tab-agnostic group jumping in main.go.
func PRGroupStarts(groups []PRGroup) []int {
	s := make([]int, len(groups))
	for i, g := range groups {
		s[i] = g.StartIdx
	}
	return s
}

func BranchGroupStarts(groups []BranchGroup) []int {
	s := make([]int, len(groups))
	for i, g := range groups {
		s[i] = g.StartIdx
	}
	return s
}

func CommitGroupStarts(groups []CommitGroup) []int {
	s := make([]int, len(groups))
	for i, g := range groups {
		s[i] = g.StartIdx
	}
	return s
}
