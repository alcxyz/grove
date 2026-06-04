# ADR-012: First-class Milestones tab

**Status:** Accepted
**Date:** 2026-06-04
**Applies to:** `internal/model/`, `internal/forge/`, `internal/cache/`, `internal/app/`, `internal/ui/`

## Context

Grove already carries milestone names on open issues, but that treats milestones
as incidental issue metadata. A milestone is a planning object in its own right:
it has state, open and closed issue counts, an optional due date, and progress
even when not all related issues are currently visible in the open Issues tab.

Using issue rows as the only milestone surface has two important gaps:

- milestones with no open issues disappear
- closed issue progress and due dates are not available

Labels are different. They are facets for filtering, classification, and hygiene;
they do not naturally provide progress or schedule metadata. A dedicated Labels
tab can be revisited later if label management becomes a workflow Grove should
own, but it is not required for milestone planning.

## Decision

Add a dedicated **Milestones** tab after Issues.

### Data model

Add `model.Milestone` with:

- `Repo`, `RepoPath`, `Profile`
- `Number`, `Title`, `Description`, `State`, `URL`
- `OpenIssues`, `ClosedIssues`
- `DueOn`, `CreatedAt`, `UpdatedAt`, `ClosedAt`

`DueOn` and `ClosedAt` are pointers because forge APIs can return explicit nulls.

### Provider contract

Extend `forge.Provider` with:

```go
ListMilestones(repoFullName string) ([]model.Milestone, error)
```

GitHub and Forgejo use their repository milestone REST endpoints and fetch open
milestones by default. Azure DevOps returns nil for now because Azure Boards work
items and iterations are not direct repository milestone equivalents.

### Caching

Add `milestones.json` using the same cache envelope as existing API data. The
entry cap is 500 milestones.

### Tab rendering

Columns: repo, title, state, progress, open count, closed count, due date, and
last update age.

Milestones are sorted by due date first, then recently updated milestones. Open
milestones without a due date remain visible after dated milestones.

### Integration points

- **Tab order:** Milestones is tab `[7]`, appended after Issues to preserve
  existing `[1]`-`[6]` muscle memory.
- **Opening:** `o` opens the milestone URL in the browser.
- **Details:** `enter` opens the related repo detail when the milestone's repo is
  local.
- **Filtering/sorting:** milestones support text filtering and cycle/sort by
  repository, title, state, due date, and updated date.

## Alternatives Considered

**Only improve the Issues tab** -- useful for quick filtering, but still hides
milestones with no open issues and cannot show closed-progress counters without
extra API calls.

**Add Labels and Milestones tabs together** -- larger UI surface with weaker
signal. Labels are better represented as issue facets until Grove has explicit
label-management actions or hygiene views.

**Derive milestones from issue rows** -- avoids one API call per repo, but loses
state, due date, closed counts, and empty milestones. This is not a durable
planning model.

## Consequences

- One additional forge API call per repo per refresh for GitHub/Forgejo profiles.
- The cache directory gains `milestones.json`.
- The tab bar gains a seventh entry. Existing tab numbers stay stable because the
  new tab is appended.
- Provider implementations must decide how to handle milestone-like concepts for
  future forges rather than forcing them into issue metadata.
