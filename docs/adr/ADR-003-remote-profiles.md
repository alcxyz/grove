# ADR-003: Remote profiles for browsing and cloning uncloned repos

**Status:** Accepted
**Date:** 2026-04-24
**Applies to:** `internal/config/`, `internal/app/`, `internal/ui/`, `internal/clone/`

## Context

Grove currently only shows repos that are already cloned locally. To discover new repos in an org, users must run `grove clone` (a batch operation) or browse a forge manually. There is no way to interactively browse remote repos, see their metadata, and selectively clone the ones you need.

Existing tools each cover part of this workflow but none cover the full lifecycle:

- **`gh` CLI** can list GitHub repos (`gh repo list`) and clone them (`gh repo clone`), but operates on one repo or one entity at a time. There is no cross-repo overview — seeing PRs, CI status, and issues across dozens of repos requires scripting multiple commands. It also has no awareness of your directory layout or routing rules.
- **Forge web UIs** can browse repos on GitHub, Forgejo, Codeberg, or self-hosted instances, but they are not connected to grove's local checkout state, profile grouping, or clone destinations.
- **lazygit** is excellent for deep single-repo git work but is purely local and single-repo. It has no forge API integration (no PRs, issues, or CI) and no multi-repo awareness. Grove already launches lazygit as its drill-down tool — they are complementary layers, not competing.

Grove's value is as the **multi-repo orchestration layer**: a single dashboard showing PRs, CI, branches, issues, and activity across all repos. Remote profiles extend this to the full discover → clone → monitor lifecycle. You browse an org's uncloned repos in the same TUI where you monitor cloned ones, see forge context before deciding to clone, then clone with a keystroke into the right directory via group routing rules — all without leaving the terminal.

A shared config across machines (macOS/Linux) already has profiles with `owner` and group routing rules. Extending profiles with a `type` field lets us reuse this infrastructure for remote browsing without a separate concept.

Since ADR-006 introduced a `Provider` interface abstracting forge-specific API calls, remote profiles should work against any supported forge (GitHub, Forgejo/Codeberg), not just GitHub. The provider already covers PRs, issues, branches, CI runs, repo listing, and cloning.

ADR-009 later split configured remotes into **code**, **social**, and **CI** concerns. That matters for already-cloned repos whose code lives on Forgejo while PRs/issues/CI remain on GitHub. Remote discovery is different: it is about repositories that do not yet have a local checkout, so the browse/list/clone concern must be anchored on the resolved **code** remote. Social and CI remotes can still provide context when configured, but they are secondary and may not exist for uncloned repos.

## Decision

Add a `type` field to profiles: `local` (default, current behaviour) and `remote`.

**Remote profiles:**
- List repos via the profile's resolved **code** remote `Provider`, using its `owner`, `forge`, `instance_url`, `token_file`, `clone_proto`, and `ssh_host`, filtered by `prefixes`.
- Show a dashboard with API-derived columns: description, language, visibility, stars/forks, last pushed, archived status.
- `enter` opens a read-only detail view with full repo metadata (description, topics, license, default branch, issue/PR counts).
- PRs, CI, and Issues tabs may work via the resolved **social** and **CI** remotes when a matching local/remote repo name can be resolved. They are best-effort context, not a prerequisite for remote browsing.
- Activity and Branches tabs are not available (they require local git data).
- `@` key triggers clone via the code `Provider.CloneRepo()`. Clone destination is inferred from the profile's group `base_path` / `match` / `match_path` rules (same routing as `grove clone`), with the option to override.
- After cloning, the repo appears in the matching local profile on next refresh.

**Profile type defaults to `local`** so existing configs are unaffected.

Remote profiles should reuse the same remote resolution primitives as local profiles instead of creating a parallel config model. In practice:

- top-level profile fields remain code defaults
- group-level `code` overrides affect remote browsing and clone destination policy
- repo-level `repo` overrides are only useful for known one-off mappings; they should not be required for ordinary discovery
- `social` and `ci` overrides are only used for additional context where they can be mapped safely

## Alternatives Considered

**Separate `grove browse` subcommand** -- Would work but fragments the experience. Users would need to leave the TUI, run a command, then return. The whole point of grove is a unified dashboard.

**Always show remote repos inline with local ones** -- Mixing cloned and uncloned repos in one list is confusing. Different data shapes (local has branch/dirty state, remote doesn't) would require either blank columns or a complex hybrid view. Separate profile type keeps the views clean.

**Add a "remote" toggle within existing local profiles** -- Muddies the profile concept. A local profile scans directories; a remote profile queries an API. These are fundamentally different data sources and deserve distinct types.

**Reuse `grove clone` only** -- The existing subcommand is useful for batch cloning missing repos, but it intentionally skips per-repo review. Issue #8 asks for discovery before cloning, which needs an interactive TUI surface.

## Consequences

- Config gains a `type` field on profiles. Omitted or `local` preserves current behaviour.
- Dashboard rendering becomes profile-type-aware: different column sets for local vs remote.
- Detail view needs a remote variant showing API metadata instead of local git info.
- The `@` key is reserved for clone actions (currently unmapped).
- Clone routing reuses existing group `base_path` infrastructure, no new config needed.
- Issues/PRs/CI for remote profiles are best-effort because ADR-009 allows these concerns to live on a different forge from code hosting.
- Network dependency: remote profiles require API access on every load (no local fallback on first use, but cacheable after).
- The "All" profile view will need to handle mixed local/remote grouping gracefully.
- The `Provider` interface (ADR-006) needs extension for remote profile browsing: `ListRepos` currently returns `[]string` (names only), but the remote dashboard and detail view require richer metadata (description, language, stars, visibility, license, topics). A `ListReposDetailed` method or similar will be needed.
- Remote profile cache keys must include resolved code/social/CI remote config, including `repo` and `ssh_host`, just like local profile API caches.
- The config preview overlay added after ADR-009 should expose remote profile resolution so users can verify which forge and clone destination a remote profile will use before cloning.

## Implementation Notes After ADR-009

The original ADR was written before split remote concerns. The implementation should avoid assuming one profile equals one forge:

1. Resolve the profile's code remote first; this is the source of repo discovery and cloning.
2. Render remote dashboard/detail from detailed repo metadata returned by that code provider.
3. If social or CI remotes are configured and map cleanly to the same repo name, enrich the remote detail with PR/issue/run counts.
4. Do not infer social/CI location from local git remotes; ADR-009 explicitly rejects that as ambiguous.
5. Use `ssh_host` when cloning over SSH and the SSH host differs from the web/API host.
