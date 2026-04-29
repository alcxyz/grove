# ADR-003: Remote profiles for browsing and cloning uncloned repos

**Status:** Accepted
**Date:** 2026-04-24
**Applies to:** `internal/config/`, `internal/app/`, `internal/ui/`, `internal/clone/`

## Context

Grove currently only shows repos that are already cloned locally. To discover new repos in an org, users must run `grove clone` (a batch operation) or browse GitHub manually. There is no way to interactively browse remote repos, see their metadata, and selectively clone the ones you need.

Existing tools each cover part of this workflow but none cover the full lifecycle:

- **`gh` CLI** can list org repos (`gh repo list`) and clone them (`gh repo clone`), but operates on one repo or one entity at a time. There is no cross-repo overview — seeing PRs, CI status, and issues across dozens of repos requires scripting multiple commands. It also has no awareness of your directory layout or routing rules.
- **lazygit** is excellent for deep single-repo git work but is purely local and single-repo. It has no GitHub API integration (no PRs, issues, or CI) and no multi-repo awareness. Grove already launches lazygit as its drill-down tool — they are complementary layers, not competing.

Grove's value is as the **multi-repo orchestration layer**: a single dashboard showing PRs, CI, branches, issues, and activity across all repos. Remote profiles extend this to the full discover → clone → monitor lifecycle. You browse an org's uncloned repos in the same TUI where you monitor cloned ones, see their GitHub context (PRs, CI, issues) before deciding to clone, then clone with a keystroke into the right directory via group routing rules — all without leaving the terminal.

A shared config across machines (macOS/Linux) already has profiles with `owner` and group routing rules. Extending profiles with a `type` field lets us reuse this infrastructure for remote browsing without a separate concept.

## Decision

Add a `type` field to profiles: `local` (default, current behaviour) and `remote`.

**Remote profiles:**
- List repos via GitHub API (`gh api`) using the profile's `owner`, filtered by `prefixes`.
- Show a dashboard with API-derived columns: description, language, visibility, stars/forks, last pushed, archived status.
- `enter` opens a read-only detail view with full repo metadata (description, topics, license, default branch, issue/PR counts).
- PRs, CI, and Issues tabs work (already GitHub API data — see ADR-004 for the Issues tab).
- Activity and Branches tabs are not available (they require local git data).
- `@` key triggers clone. Clone destination is inferred from the profile's group `base_path` / `match` rules (same routing as `grove clone`), with the option to override.
- After cloning, the repo appears in the matching local profile on next refresh.

**Profile type defaults to `local`** so existing configs are unaffected.

## Alternatives Considered

**Separate `grove browse` subcommand** -- Would work but fragments the experience. Users would need to leave the TUI, run a command, then return. The whole point of grove is a unified dashboard.

**Always show remote repos inline with local ones** -- Mixing cloned and uncloned repos in one list is confusing. Different data shapes (local has branch/dirty state, remote doesn't) would require either blank columns or a complex hybrid view. Separate profile type keeps the views clean.

**Add a "remote" toggle within existing local profiles** -- Muddies the profile concept. A local profile scans directories; a remote profile queries an API. These are fundamentally different data sources and deserve distinct types.

## Consequences

- Config gains a `type` field on profiles. Omitted or `local` preserves current behaviour.
- Dashboard rendering becomes profile-type-aware: different column sets for local vs remote.
- Detail view needs a remote variant showing API metadata instead of local git info.
- The `@` key is reserved for clone actions (currently unmapped).
- Clone routing reuses existing group `base_path` infrastructure, no new config needed.
- Issues tab (ADR-004) is fully available for remote profiles since it uses the same GitHub API data source as PRs and CI.
- Network dependency: remote profiles require API access on every load (no local fallback on first use, but cacheable after).
- The "All" profile view will need to handle mixed local/remote grouping gracefully.
