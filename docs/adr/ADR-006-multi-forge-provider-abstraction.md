# ADR-006: Multi-forge provider abstraction

**Status:** Accepted
**Date:** 2026-05-01 (proposed 2026-04-29)
**Applies to:** `internal/forge/`, `internal/model/repo.go`, `internal/config/config.go`, `internal/app/commands.go`, `internal/clone/clone.go`

**Revision 2026-05-05:** GitHub now uses the same direct HTTP provider model as Forgejo. The `gh` CLI is no longer a provider dependency for PRs, issues, branches, CI, repo enumeration, or clone routing.

## Context

Grove started tightly coupled to GitHub at every layer of the stack. Data fetching went through the `gh` CLI wrapper in `internal/gh/gh.go`, model types in `internal/model/repo.go` assumed GitHub semantics, and the `grove clone` subcommand called the GitHub REST API directly.

Users increasingly self-host git forges — Forgejo, Gitea, GitLab — alongside or instead of GitHub. Supporting these would make grove useful to a much broader audience. Even without adding new providers immediately, extracting a provider abstraction would improve testability (the current `gh` package cannot be mocked without exec-level patching).

The branding and about text have been updated to reflect the intent: grove is a "multi-repo git forge monitoring TUI" that is currently GitHub-native.

Forgejo (a community fork of Gitea) is the priority second target. Codeberg — a major public hosting service — runs Forgejo, so a single `forgejo` provider covers both Codeberg and self-hosted Forgejo instances. Modern Gitea (which shares the same v1 REST API) is also covered as a side effect, but Gitea-specific compatibility is a non-goal.

## Decision

**Option B then C:** First extract a `Provider` interface with GitHub as the sole implementation (Phase 1), then add a `forgejo` provider targeting Forgejo 1.20+ (Phase 2).

- **Phase 1** — Create `internal/forge/forge.go` with a `Provider` interface. Move GitHub API access behind a `GitHubProvider` implementation. Add a `forge` field to `config.Profile` (default `"github"`). Wire `commands.go` to call through the interface. Models stay mostly as-is; GitHub-specific fields (`ReviewDecision`, `WorkflowRun` details) become optional/nullable for forges that don't support them.
- **Phase 2** — Implement `ForgejoProvider` using Forgejo's v1 REST API with token-based auth. Config gains an `instance_url` field for non-GitHub profiles. Feature-detect Actions support (graceful nil if unavailable on older instances).

**Provider naming:** The second provider is called `forgejo`, not `gitea` or `forgejo_compat`. It targets Forgejo and Codeberg directly; Gitea compatibility is incidental.

**Version floor:** Forgejo 1.20+ (stable v1 API, Actions support). No effort spent on older Gitea versions or Gitea-specific API divergences.

## Alternatives Considered

### A — Language only, no code changes
Update docs and about text to use "git forge" framing. No architectural change.

- **Pro:** zero risk, instant
- **Con:** misleading if grove never actually supports other forges; does nothing for testability

### B — Extract a `Fetcher` interface, GitHub remains the only implementation
Define a `Fetcher` (or `ForgeClient`) interface in a new `internal/forge/` package. The existing `gh` package becomes the `GitHubFetcher` concrete type. No other providers added yet.

- **Pro:** clean seam for future providers; enables unit-testing with a mock fetcher; minimal disruption to the rest of the codebase
- **Con:** interface design is hard to get right before a second consumer exists; risk of designing the wrong abstraction

### C — Interface extraction + one additional provider (e.g., Forgejo/Gitea)
Build the interface with a real second implementation from the start, so the abstraction is validated against two concrete cases.

- **Pro:** forces the abstraction to be genuinely generic, not GitHub-shaped with a thin veneer
- **Con:** much larger scope; requires working through all the edge cases below

### D — Stay GitHub-only
Explicitly commit to GitHub as the only supported forge.

- **Pro:** no complexity; `gh` CLI is an excellent foundation
- **Con:** excludes self-hosted users; misses testability gains

## Open Questions / Edge Cases

Resolved or scoped down by the decision above:

**CI runs:**
GitHub has Actions (`WorkflowRun`). Forgejo 1.20+ has its own Actions (GitHub Actions-compatible API, different endpoints). The `Provider` interface returns `[]model.WorkflowRun`; providers that don't support Actions return nil. No lowest-common-denominator abstraction needed — Forgejo Actions are close enough to GitHub Actions to share the model.

**Review decisions:**
`ReviewDecision` (APPROVED / CHANGES_REQUESTED / REVIEW_REQUIRED) is GitHub-specific. Forgejo has a similar review model. Make the field optional in the shared model; map Forgejo's review states where possible, leave empty where not.

**Org/user repo enumeration:**
`grove clone` calls `gh api orgs/<owner>/repos` or `users/<owner>/repos`. Forgejo uses `/api/v1/orgs/{org}/repos` and `/api/v1/users/{user}/repos` — structurally similar, different base URL. Per-provider implementation via the `Provider` interface.

**Auth:**
Forgejo uses token-based HTTP auth via `token_file`. GitHub supports two modes per resolved remote: `auth_mode: token` (default), which reads `token_file` or falls back to `GH_TOKEN` / `GITHUB_TOKEN`, and `auth_mode: gh`, which shells out to the authenticated GitHub CLI (`gh api`, `gh repo clone`). Fine-grained GitHub PATs should be repository-scoped and read-only when token mode is usable, with only the permissions that match configured concerns: Metadata, Pull requests, Issues, Actions, Commit statuses, and Contents when GitHub is the code remote for branch/commit metadata. `auth_mode: gh` exists for organizations that permit `gh auth login` but block PAT creation.

**Rate limits:**
GitHub caps authenticated REST API requests at 5,000 req/hr for normal user tokens. Grove limits GitHub-backed API calls to 5 concurrent HTTP requests. Forgejo instance limits are admin-configurable. The concurrency model is per-provider.

**Clone mechanism:**
`grove clone` uses `Provider.CloneRepo()`. GitHub and Forgejo both shell out to `git clone` with provider-specific HTTPS or SSH URLs. Private HTTPS clones depend on the user's git credential setup; `clone_proto: ssh` is preferred when API auth and git transport auth should remain separate.

**Labels, assignees, milestones on issues:**
Forgejo's model is very close to GitHub's. Mappable to a shared model with minimal loss of fidelity. GitLab is out of scope for now.

## Implementation Plan

1. **Phase 1:** Extract `Provider` interface in `internal/forge/forge.go`, implement direct HTTP `GitHubProvider`
2. **Phase 2:** Implement `ForgejoProvider` using Forgejo v1 REST API, add `forge` and `instance_url` config fields
3. Validate against Codeberg (public Forgejo instance) as the primary test target

## Consequences

- Grove becomes useful to Forgejo, Codeberg, and self-hosted users
- GitHub and Forgejo provider behavior can be tested with local HTTP servers instead of CLI subprocess mocks
- Config profiles gain `forge` and `instance_url` fields (breaking change to config schema)
- GitHub-specific fields (`ReviewDecision`, Actions-specific workflow metadata) become optional in the model
- Significant refactor across `internal/forge/`, `internal/model/`, `internal/app/commands.go`, and `internal/clone/`
- GitLab support is not in scope but the abstraction should not preclude it
