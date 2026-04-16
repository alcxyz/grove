package main

import (
	"sort"
	"strings"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

// repoBaseName strips the org prefix from "org/repo" → "repo".
func repoBaseName(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}

// cyclePrefix extracts the grouping key for subject-field cycling.
// Splits on the first space or slash so that:
//   - "feat: add login"  → "feat:"
//   - "fix: timeout"     → "fix:"
//   - "feature/my-thing" → "feature"
//   - "bugfix/foo"       → "bugfix"
func cyclePrefix(s string) string {
	if i := strings.IndexAny(s, " /"); i > 0 {
		return s[:i]
	}
	return s
}

// normalizeCI maps a run's status/conclusion to a simple string used for cycle
// filtering and sorting.  Returns "" for unknown/no-data (skipped by add()).
func normalizeCI(status, conclusion string) string {
	if status != "completed" {
		return "running"
	}
	switch conclusion {
	case "success":
		return "success"
	case "failure", "timed_out", "startup_failure":
		return "failure"
	case "cancelled":
		return "cancelled"
	default:
		return ""
	}
}

// repoPRCounts returns a map of repo base-name → open PR count derived from m.prs.
func (m appModel) repoPRCounts() map[string]int {
	counts := map[string]int{}
	for _, pr := range m.prs {
		counts[repoBaseName(pr.Repo)]++
	}
	return counts
}

// repoBranchCounts returns a map of repo base-name → remote branch count derived from m.branches.
func (m appModel) repoBranchCounts() map[string]int {
	counts := map[string]int{}
	for _, br := range m.branches {
		counts[repoBaseName(br.Repo)]++
	}
	return counts
}

// repoLatestCI returns a map of repo base-name → normalised CI conclusion
// (runs are assumed newest-first so the first entry per repo wins).
func (m appModel) repoLatestCI() map[string]string {
	ci := map[string]string{}
	for _, r := range m.runs {
		name := repoBaseName(r.Repo)
		if _, seen := ci[name]; !seen {
			ci[name] = normalizeCI(r.Status, r.Conclusion)
		}
	}
	return ci
}

// prBranchSet returns the set of branch names that have an open PR.
func (m appModel) prBranchSet() map[string]bool {
	set := map[string]bool{}
	for _, pr := range m.prs {
		set[pr.Branch] = true
	}
	return set
}

func applyRepoSort(out []model.Repo, ts tabSortState) {
	asc := ts.Order == sortAsc
	byName := func(i, j int) bool {
		if asc {
			return out[i].Name < out[j].Name
		}
		return out[i].Name > out[j].Name
	}
	switch ts.Field {
	case "date":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].LastCommit.Before(out[j].LastCommit)
			}
			return out[i].LastCommit.After(out[j].LastCommit)
		})
	case "author":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].LastAuthor < out[j].LastAuthor
			}
			return out[i].LastAuthor > out[j].LastAuthor
		})
	case "subject", "repo": // both map to repo name on the dashboard
		sort.SliceStable(out, byName)
	}
}

func applyPRSort(out []model.PR, ts tabSortState) {
	asc := ts.Order == sortAsc
	switch ts.Field {
	case "date":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].UpdatedAt.Before(out[j].UpdatedAt)
			}
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		})
	case "author":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Author < out[j].Author
			}
			return out[i].Author > out[j].Author
		})
	case "subject":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Title < out[j].Title
			}
			return out[i].Title > out[j].Title
		})
	case "repo":
		sort.SliceStable(out, func(i, j int) bool {
			ri, rj := repoBaseName(out[i].Repo), repoBaseName(out[j].Repo)
			if asc {
				return ri < rj
			}
			return ri > rj
		})
	case "review":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].ReviewDecision < out[j].ReviewDecision
			}
			return out[i].ReviewDecision > out[j].ReviewDecision
		})
	case "checks":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Checks < out[j].Checks
			}
			return out[i].Checks > out[j].Checks
		})
	}
}

func applyBranchSort(out []model.BranchInfo, ts tabSortState) {
	asc := ts.Order == sortAsc
	switch ts.Field {
	case "date":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].LastCommit.Before(out[j].LastCommit)
			}
			return out[i].LastCommit.After(out[j].LastCommit)
		})
	case "author":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Author < out[j].Author
			}
			return out[i].Author > out[j].Author
		})
	case "subject":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Name < out[j].Name
			}
			return out[i].Name > out[j].Name
		})
	case "repo":
		sort.SliceStable(out, func(i, j int) bool {
			ri, rj := repoBaseName(out[i].Repo), repoBaseName(out[j].Repo)
			if asc {
				return ri < rj
			}
			return ri > rj
		})
	case "merged":
		sort.SliceStable(out, func(i, j int) bool {
			mi, mj := out[i].IsMerged, out[j].IsMerged
			if asc {
				return !mi && mj // false < true: not-merged first
			}
			return mi && !mj // true > false: merged first
		})
	}
}

func applyRunSort(out []model.WorkflowRun, ts tabSortState) {
	asc := ts.Order == sortAsc
	switch ts.Field {
	case "date":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].UpdatedAt.Before(out[j].UpdatedAt)
			}
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		})
	case "subject":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].WorkflowName < out[j].WorkflowName
			}
			return out[i].WorkflowName > out[j].WorkflowName
		})
	case "repo":
		sort.SliceStable(out, func(i, j int) bool {
			ri, rj := repoBaseName(out[i].Repo), repoBaseName(out[j].Repo)
			if asc {
				return ri < rj
			}
			return ri > rj
		})
	case "ci":
		sort.SliceStable(out, func(i, j int) bool {
			ci := normalizeCI(out[i].Status, out[i].Conclusion)
			cj := normalizeCI(out[j].Status, out[j].Conclusion)
			if asc {
				return ci < cj
			}
			return ci > cj
		})
	case "branch":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Branch < out[j].Branch
			}
			return out[i].Branch > out[j].Branch
		})
	}
}

func applyCommitSort(out []model.Commit, ts tabSortState) {
	asc := ts.Order == sortAsc
	switch ts.Field {
	case "date":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Date.Before(out[j].Date)
			}
			return out[i].Date.After(out[j].Date)
		})
	case "author":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Author < out[j].Author
			}
			return out[i].Author > out[j].Author
		})
	case "subject":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Subject < out[j].Subject
			}
			return out[i].Subject > out[j].Subject
		})
	case "repo":
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return out[i].Repo < out[j].Repo
			}
			return out[i].Repo > out[j].Repo
		})
	}
}

func (m appModel) filteredRepos() []model.Repo {
	q := strings.ToLower(m.filterQuery)
	ts := m.tabSort[tabDashboard]
	hasCycle := m.cycleField == "author" || m.cycleField == "date" ||
		m.cycleField == "prcount" || m.cycleField == "brcount" || m.cycleField == "ci"
	profileFilter := m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles)
	if q == "" && !hasCycle && ts.Field == "" && !profileFilter {
		return m.repos
	}
	activeProfileName := ""
	if profileFilter {
		activeProfileName = m.cfg.Profiles[m.activeProfile].Name
	}
	// Compute derived maps only when needed for cycle filtering.
	var prCounts map[string]int
	if m.cycleField == "prcount" {
		prCounts = m.repoPRCounts()
	}
	var brCounts map[string]int
	if m.cycleField == "brcount" {
		brCounts = m.repoBranchCounts()
	}
	var ciMap map[string]string
	if m.cycleField == "ci" {
		ciMap = m.repoLatestCI()
	}
	out := make([]model.Repo, 0, len(m.repos))
	for _, r := range m.repos {
		if profileFilter && r.Profile != activeProfileName {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(r.Name), q) &&
				!strings.Contains(strings.ToLower(r.Branch), q) &&
				!strings.Contains(strings.ToLower(r.LastAuthor), q) {
				continue
			}
		}
		if !m.cycleMatch("author", r.LastAuthor) {
			continue
		}
		if !m.cycleMatchDate(r.LastCommit) {
			continue
		}
		if prCounts != nil {
			bucket := "no PRs"
			if prCounts[r.Name] > 0 {
				bucket = "has PRs"
			}
			if !m.cycleMatch("prcount", bucket) {
				continue
			}
		}
		if brCounts != nil {
			bucket := "no branches"
			if brCounts[r.Name] > 0 {
				bucket = "has branches"
			}
			if !m.cycleMatch("brcount", bucket) {
				continue
			}
		}
		if ciMap != nil {
			if !m.cycleMatch("ci", ciMap[r.Name]) {
				continue
			}
		}
		out = append(out, r)
	}
	// Special sort fields require derived data.
	asc := ts.Order == sortAsc
	switch ts.Field {
	case "prcount":
		pc := m.repoPRCounts()
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return pc[out[i].Name] < pc[out[j].Name]
			}
			return pc[out[i].Name] > pc[out[j].Name]
		})
	case "brcount":
		bc := m.repoBranchCounts()
		sort.SliceStable(out, func(i, j int) bool {
			if asc {
				return bc[out[i].Name] < bc[out[j].Name]
			}
			return bc[out[i].Name] > bc[out[j].Name]
		})
	case "ci":
		cm := m.repoLatestCI()
		sort.SliceStable(out, func(i, j int) bool {
			ci, cj := cm[out[i].Name], cm[out[j].Name]
			if asc {
				return ci < cj
			}
			return ci > cj
		})
	default:
		applyRepoSort(out, ts)
	}
	return out
}

func (m appModel) filteredPRs() []model.PR {
	q := strings.ToLower(m.filterQuery)
	ts := m.tabSort[tabPRs]
	hasCycle := m.cycleField == "author" || m.cycleField == "subject" || m.cycleField == "repo" ||
		m.cycleField == "date" || m.cycleField == "review" || m.cycleField == "checks"
	profileFilter := m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles)
	if q == "" && !hasCycle && ts.Field == "" && !profileFilter {
		return m.prs
	}
	activeProfileName := ""
	if profileFilter {
		activeProfileName = m.cfg.Profiles[m.activeProfile].Name
	}
	out := make([]model.PR, 0, len(m.prs))
	for _, pr := range m.prs {
		if profileFilter && pr.Profile != activeProfileName {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(pr.Repo), q) &&
				!strings.Contains(strings.ToLower(pr.Title), q) &&
				!strings.Contains(strings.ToLower(pr.Author), q) &&
				!strings.Contains(strings.ToLower(pr.Branch), q) {
				continue
			}
		}
		if !m.cycleMatch("author", pr.Author) {
			continue
		}
		if !m.cycleMatch("subject", pr.Title) {
			continue
		}
		if !m.cycleMatch("repo", repoBaseName(pr.Repo)) {
			continue
		}
		if !m.cycleMatchDate(pr.UpdatedAt) {
			continue
		}
		if !m.cycleMatch("review", pr.ReviewDecision) {
			continue
		}
		if !m.cycleMatch("checks", pr.Checks) {
			continue
		}
		out = append(out, pr)
	}
	applyPRSort(out, ts)
	return out
}

func (m appModel) filteredBranches() []model.BranchInfo {
	q := strings.ToLower(m.filterQuery)
	ts := m.tabSort[tabBranches]
	hasCycle := m.cycleField == "subject" || m.cycleField == "repo" || m.cycleField == "author" ||
		m.cycleField == "date" || m.cycleField == "prcount" || m.cycleField == "merged"
	profileFilter := m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles)
	if q == "" && !hasCycle && ts.Field == "" && !profileFilter {
		return m.branches
	}
	activeProfileName := ""
	if profileFilter {
		activeProfileName = m.cfg.Profiles[m.activeProfile].Name
	}
	var prBranchesMap map[string]bool
	if m.cycleField == "prcount" {
		prBranchesMap = m.prBranchSet()
	}
	out := make([]model.BranchInfo, 0, len(m.branches))
	for _, br := range m.branches {
		if profileFilter && br.Profile != activeProfileName {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(br.Repo), q) &&
				!strings.Contains(strings.ToLower(br.Name), q) &&
				!strings.Contains(strings.ToLower(br.Author), q) {
				continue
			}
		}
		if !m.cycleMatch("subject", br.Name) {
			continue
		}
		if !m.cycleMatch("repo", repoBaseName(br.Repo)) {
			continue
		}
		if !m.cycleMatch("author", br.Author) {
			continue
		}
		if !m.cycleMatchDate(br.LastCommit) {
			continue
		}
		if prBranchesMap != nil {
			bucket := "no PR"
			if prBranchesMap[br.Name] {
				bucket = "has PR"
			}
			if !m.cycleMatch("prcount", bucket) {
				continue
			}
		}
		mergedVal := "not merged"
		if br.IsMerged {
			mergedVal = "merged"
		}
		if !m.cycleMatch("merged", mergedVal) {
			continue
		}
		out = append(out, br)
	}
	// "prcount" (has-PR) sort requires the prBranches set.
	if ts.Field == "prcount" {
		pb := m.prBranchSet()
		asc := ts.Order == sortAsc
		sort.SliceStable(out, func(i, j int) bool {
			hi, hj := pb[out[i].Name], pb[out[j].Name]
			if asc {
				return !hi && hj
			}
			return hi && !hj
		})
	} else {
		applyBranchSort(out, ts)
	}
	return out
}

func (m appModel) filteredRuns() []model.WorkflowRun {
	q := strings.ToLower(m.filterQuery)
	ts := m.tabSort[tabCI]
	hasCycle := m.cycleField == "subject" || m.cycleField == "repo" || m.cycleField == "date" ||
		m.cycleField == "ci" || m.cycleField == "branch"
	profileFilter := m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles)
	if q == "" && !hasCycle && ts.Field == "" && !profileFilter {
		return m.runs
	}
	activeProfileName := ""
	if profileFilter {
		activeProfileName = m.cfg.Profiles[m.activeProfile].Name
	}
	out := make([]model.WorkflowRun, 0, len(m.runs))
	for _, r := range m.runs {
		if profileFilter && r.Profile != activeProfileName {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(repoBaseName(r.Repo)), q) &&
				!strings.Contains(strings.ToLower(r.WorkflowName), q) &&
				!strings.Contains(strings.ToLower(r.Branch), q) &&
				!strings.Contains(strings.ToLower(r.Conclusion), q) {
				continue
			}
		}
		if !m.cycleMatch("subject", r.WorkflowName) {
			continue
		}
		if !m.cycleMatch("repo", repoBaseName(r.Repo)) {
			continue
		}
		if !m.cycleMatchDate(r.UpdatedAt) {
			continue
		}
		if !m.cycleMatch("ci", normalizeCI(r.Status, r.Conclusion)) {
			continue
		}
		if !m.cycleMatch("branch", cyclePrefix(r.Branch)) {
			continue
		}
		out = append(out, r)
	}
	applyRunSort(out, ts)
	return out
}

func (m appModel) filteredActivity() []model.Commit {
	q := strings.ToLower(m.filterQuery)
	ts := m.tabSort[tabActivity]
	hasCycle := m.cycleField == "author" || m.cycleField == "subject" || m.cycleField == "repo" || m.cycleField == "date"
	profileFilter := m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles)
	if q == "" && !hasCycle && ts.Field == "" && !profileFilter {
		return m.activity
	}
	activeProfileName := ""
	if profileFilter {
		activeProfileName = m.cfg.Profiles[m.activeProfile].Name
	}
	out := make([]model.Commit, 0, len(m.activity))
	for _, c := range m.activity {
		if profileFilter && c.Profile != activeProfileName {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(c.Repo), q) &&
				!strings.Contains(strings.ToLower(c.Subject), q) &&
				!strings.Contains(strings.ToLower(c.Author), q) {
				continue
			}
		}
		if !m.cycleMatch("author", c.Author) {
			continue
		}
		if !m.cycleMatch("subject", c.Subject) {
			continue
		}
		if !m.cycleMatch("repo", c.Repo) {
			continue
		}
		if !m.cycleMatchDate(c.Date) {
			continue
		}
		out = append(out, c)
	}
	applyCommitSort(out, ts)
	return out
}

// timeBuckets are the fixed date-range labels for the "t" date-cycle filter.
var timeBuckets = []string{"today", "yesterday", "this week", "last week", "this month", "last month", "this quarter", "last quarter"}

// dateInBucket returns true if t falls within the named time bucket.
func dateInBucket(t time.Time, label string) bool {
	if t.IsZero() {
		return false
	}
	now := time.Now()
	y, mo, d := now.Date()
	loc := now.Location()
	todayStart := time.Date(y, mo, d, 0, 0, 0, 0, loc)
	switch label {
	case "today":
		return !t.Before(todayStart)
	case "yesterday":
		return !t.Before(todayStart.AddDate(0, 0, -1)) && t.Before(todayStart)
	case "this week":
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		return !t.Before(todayStart.AddDate(0, 0, -(wd - 1)))
	case "last week":
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		thisWeek := todayStart.AddDate(0, 0, -(wd - 1))
		return !t.Before(thisWeek.AddDate(0, 0, -7)) && t.Before(thisWeek)
	case "this month":
		return !t.Before(time.Date(y, mo, 1, 0, 0, 0, 0, loc))
	case "last month":
		thisMonth := time.Date(y, mo, 1, 0, 0, 0, 0, loc)
		return !t.Before(thisMonth.AddDate(0, -1, 0)) && t.Before(thisMonth)
	case "this quarter":
		qStart := time.Date(y, ((mo-1)/3)*3+1, 1, 0, 0, 0, 0, loc)
		return !t.Before(qStart)
	case "last quarter":
		qStart := time.Date(y, ((mo-1)/3)*3+1, 1, 0, 0, 0, 0, loc)
		lastQStart := qStart.AddDate(0, -3, 0)
		return !t.Before(lastQStart) && t.Before(qStart)
	}
	return false
}

// cycleMatch returns true if item passes the active cycle filter for the given field value.
// For "subject" the match is by prefix (first word/path segment) rather than exact value.
func (m appModel) cycleMatch(field, value string) bool {
	if m.cycleField != field || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return true
	}
	target := m.cycleValues[m.cycleIdx]
	if field == "subject" {
		return cyclePrefix(value) == target
	}
	return value == target
}

// cycleMatchDate returns true if t falls in the active date bucket (when
// cycleField == "date"), or true if date cycling is not active.
func (m appModel) cycleMatchDate(t time.Time) bool {
	if m.cycleField != "date" || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return true
	}
	return dateInBucket(t, m.cycleValues[m.cycleIdx])
}

// collectCycleValues gathers unique sorted values for a field from the raw
// (text-filtered only) data of the active tab.  Called when a cycle key is
// first pressed or the field changes.
func (m appModel) collectCycleValues(field string) []string {
	if field == "date" {
		return timeBuckets
	}
	q := strings.ToLower(m.filterQuery)
	match := func(fields ...string) bool {
		if q == "" {
			return true
		}
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), q) {
				return true
			}
		}
		return false
	}
	seen := map[string]struct{}{}
	var vals []string
	add := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		vals = append(vals, v)
	}
	switch m.activeTab {
	case tabDashboard:
		switch field {
		case "author":
			for _, r := range m.repos {
				if match(r.Name, r.Branch, r.LastAuthor) {
					add(r.LastAuthor)
				}
			}
		case "prcount":
			pc := m.repoPRCounts()
			seenNone, seenAny := false, false
			for _, r := range m.repos {
				if match(r.Name, r.Branch, r.LastAuthor) {
					if pc[r.Name] == 0 {
						seenNone = true
					} else {
						seenAny = true
					}
				}
			}
			if seenNone {
				vals = append(vals, "no PRs")
			}
			if seenAny {
				vals = append(vals, "has PRs")
			}
			return vals // fixed order — skip sort.Strings
		case "brcount":
			bc := m.repoBranchCounts()
			seenNone, seenAny := false, false
			for _, r := range m.repos {
				if match(r.Name, r.Branch, r.LastAuthor) {
					if bc[r.Name] == 0 {
						seenNone = true
					} else {
						seenAny = true
					}
				}
			}
			if seenNone {
				vals = append(vals, "no branches")
			}
			if seenAny {
				vals = append(vals, "has branches")
			}
			return vals
		case "ci":
			cm := m.repoLatestCI()
			for _, r := range m.repos {
				if match(r.Name, r.Branch, r.LastAuthor) {
					add(cm[r.Name])
				}
			}
		default:
			return nil
		}
	case tabPRs:
		for _, pr := range m.prs {
			if !match(pr.Repo, pr.Title, pr.Author, pr.Branch) {
				continue
			}
			switch field {
			case "author":
				add(pr.Author)
			case "subject":
				add(cyclePrefix(pr.Title))
			case "repo":
				add(repoBaseName(pr.Repo))
			case "review":
				add(pr.ReviewDecision)
			case "checks":
				add(pr.Checks)
			}
		}
	case tabBranches:
		switch field {
		case "author", "subject", "repo":
			for _, br := range m.branches {
				if !match(br.Repo, br.Name, br.Author) {
					continue
				}
				switch field {
				case "author":
					add(br.Author)
				case "subject":
					add(cyclePrefix(br.Name))
				case "repo":
					add(repoBaseName(br.Repo))
				}
			}
		case "prcount":
			prBranches := m.prBranchSet()
			seenNo, seenYes := false, false
			for _, br := range m.branches {
				if !match(br.Repo, br.Name, br.Author) {
					continue
				}
				if prBranches[br.Name] {
					seenYes = true
				} else {
					seenNo = true
				}
			}
			if seenNo {
				vals = append(vals, "no PR")
			}
			if seenYes {
				vals = append(vals, "has PR")
			}
			return vals
		case "merged":
			seenNo, seenYes := false, false
			for _, br := range m.branches {
				if !match(br.Repo, br.Name, br.Author) {
					continue
				}
				if br.IsMerged {
					seenYes = true
				} else {
					seenNo = true
				}
			}
			if seenNo {
				vals = append(vals, "not merged")
			}
			if seenYes {
				vals = append(vals, "merged")
			}
			return vals
		default:
			return nil
		}
	case tabActivity:
		for _, c := range m.activity {
			if !match(c.Repo, c.Subject, c.Author) {
				continue
			}
			switch field {
			case "author":
				add(c.Author)
			case "subject":
				add(cyclePrefix(c.Subject))
			case "repo":
				add(c.Repo)
			}
		}
	case tabCI:
		for _, r := range m.runs {
			if !match(repoBaseName(r.Repo), r.WorkflowName, r.Branch, r.Conclusion) {
				continue
			}
			switch field {
			case "subject":
				add(cyclePrefix(r.WorkflowName))
			case "repo":
				add(repoBaseName(r.Repo))
			case "ci":
				add(normalizeCI(r.Status, r.Conclusion))
			case "branch":
				add(cyclePrefix(r.Branch))
			}
		}
	}
	sort.Strings(vals)
	return vals
}

// doCycleFilter advances (or starts) a cycle filter for field.
// Pressing the same key again advances to the next value; wrapping past the
// end clears the filter.
func (m *appModel) doCycleFilter(field string) {
	newVals := m.collectCycleValues(field)
	if len(newVals) == 0 {
		return
	}
	if m.cycleField != field {
		// New field — start fresh
		m.cycleField = field
		m.cycleValues = newVals
		m.cycleIdx = 0
	} else {
		// Same field — find current value in refreshed list and advance
		currentVal := ""
		if m.cycleIdx >= 0 && m.cycleIdx < len(m.cycleValues) {
			currentVal = m.cycleValues[m.cycleIdx]
		}
		nextIdx := 0
		for i, v := range newVals {
			if v == currentVal {
				nextIdx = i + 1
				break
			}
		}
		m.cycleValues = newVals
		if nextIdx >= len(newVals) {
			// Wrapped past end — clear
			m.cycleField = ""
			m.cycleValues = nil
			m.cycleIdx = -1
			m.cursor = 0
			m.adjustScroll()
			return
		}
		m.cycleIdx = nextIdx
	}
	m.cursor = 0
	m.adjustScroll()
}

// cycleSortField advances the sort for a field:
//   - if a different field is active, switch to this field at sortAsc
//   - sortAsc → sortDesc → clear (Field="")
func (m *appModel) cycleSortField(field string) {
	cur := m.tabSort[m.activeTab]
	if cur.Field != field {
		m.tabSort[m.activeTab] = tabSortState{Field: field, Order: sortAsc}
	} else {
		switch cur.Order {
		case sortAsc:
			m.tabSort[m.activeTab] = tabSortState{Field: field, Order: sortDesc}
		default: // sortDesc or anything else → clear
			m.tabSort[m.activeTab] = tabSortState{}
		}
	}
	m.cursor = 0
	m.adjustScroll()
}

// clearCycleFilter resets any active cycle filter and block highlight.
func (m *appModel) clearCycleFilter() {
	m.cycleField = ""
	m.cycleValues = nil
	m.cycleIdx = -1
	m.highlightField = ""
}
