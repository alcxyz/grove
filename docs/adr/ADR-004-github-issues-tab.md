# ADR-004: GitHub Issues tab

**Status:** Accepted
**Date:** 2026-04-28
**Applies to:** `internal/model/`, `internal/gh/`, `internal/cache/`, `internal/app/`, `internal/ui/`

## Context

Grove monitors PRs, CI runs, branches, and recent commits across repos — but has no visibility into GitHub Issues. For teams that use issues for bug tracking, feature requests, and task management, switching to a browser to check issue status breaks the terminal-centric workflow grove provides.

Issues are a natural extension of the existing data model. Every other GitHub-native entity (PRs, branches, workflow runs) already has a dedicated tab and follows the same fetch → cache → render pipeline. Issues can reuse this architecture with minimal new concepts.

## Decision

Add a new **Issues** tab as tab `[6]`, after Activity `[5]`.

### Data model

A new `Issue` struct in `internal/model/repo.go`:

- `Repo`, `Number`, `Title`, `Author`, `State` (open/closed), `URL`, `Profile` — same shape as other entities
- `Labels` (slice of label names), `Assignees` (slice of logins), `Milestone` (name string)
- `UpdatedAt`, `CreatedAt` — timestamps for sorting and display

### Fetching

`gh issue list` per repo via `internal/gh/`, following the same semaphore-guarded pattern as `ListPRs`. Default: open issues only, limit 50. `gh issue list` already excludes pull requests from the result set, so no additional filtering is needed.

### Caching

`issues.json` in the cache directory, same envelope format (cached_at, config_key, data) and cap (500 entries) as PRs.

### Tab rendering

Columns: repo, `#number`, title, author, labels (comma-joined), assignee(s), milestone, age. Sorted by most recently updated by default.

### Integration points

- **Dashboard tab:** add an open-issue count column per repo (like the existing PR count).
- **Detail pane:** show open issues for the selected repo alongside PRs, branches, and commits.
- **Keybinding:** `6` switches to Issues tab; `o` opens issue in browser.
- **Filtering:** text filter, cycle-filter on author/repo/label, and sort by date/title.

### Tab ordering

Issues is added as `[6]` rather than inserted between existing tabs. This avoids changing established muscle memory for `[1]`–`[5]`.

## Alternatives Considered

**Embed issues into the PRs tab** — Issues and PRs are different workflows with different metadata (labels/milestones vs review status/checks). Mixing them would clutter the PR view without adding clarity.

**Use GraphQL for issue fetching** — `gh issue list --json` already provides all needed fields in a single call and handles pagination. GraphQL would only help if we needed fields beyond what the CLI exposes, which we don't currently. Can revisit if needed.

**Show closed issues by default** — Closed issues accumulate fast and would dominate the list. Defaulting to open-only matches how the PRs tab works. A future enhancement could add a state toggle.

## Consequences

- Tab bar gains a 6th entry. Addressed by wrapping tabs into a 2×3 grid with number-first labels and equal-width columns.
- One additional `gh issue list` call per repo per refresh. With the semaphore cap of 5 and typical repo counts, this is within rate-limit budget but increases total refresh time proportionally.
- Cache directory gains one more JSON file (`issues.json`).
- The dashboard table gains one more "Is" column. Addressed by the dynamic column width system (see ADR-005) which distributes all columns proportionally within the terminal width.
- Label colors from GitHub are not used (terminal color mapping is unreliable). Labels render as plain text.
