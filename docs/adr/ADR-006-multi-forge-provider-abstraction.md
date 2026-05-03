# ADR-006: Multi-forge provider abstraction

**Status:** Accepted
**Date:** 2026-05-01 (proposed 2026-04-29)
**Applies to:** `internal/gh/`, `internal/model/repo.go`, `internal/config/config.go`, `internal/app/commands.go`, `internal/clone/clone.go`

## Context

Grove is currently tightly coupled to GitHub at every layer of the stack. All data fetching goes through the `gh` CLI wrapper in `internal/gh/gh.go`, all model types in `internal/model/repo.go` assume GitHub semantics, and the `grove clone` subcommand calls the GitHub REST API directly.

Users increasingly self-host git forges — Forgejo, Gitea, GitLab — alongside or instead of GitHub. Supporting these would make grove useful to a much broader audience. Even without adding new providers immediately, extracting a provider abstraction would improve testability (the current `gh` package cannot be mocked without exec-level patching).

The branding and about text have been updated to reflect the intent: grove is a "multi-repo git forge monitoring TUI" that is currently GitHub-native.

Forgejo (a community fork of Gitea) is the priority second target. Codeberg — a major public hosting service — runs Forgejo, so a single `forgejo` provider covers both Codeberg and self-hosted Forgejo instances. Modern Gitea (which shares the same v1 REST API) is also covered as a side effect, but Gitea-specific compatibility is a non-goal.

## Decision

**Option B then C:** First extract a `Provider` interface with GitHub as the sole implementation (Phase 1), then add a `forgejo` provider targeting Forgejo 1.20+ (Phase 2).

- **Phase 1** — Create `internal/forge/forge.go` with a `Provider` interface. Move `internal/gh/` behind a `GitHubProvider` implementation. Add a `forge` field to `config.Profile` (default `"github"`). Wire `commands.go` to call through the interface. Models stay mostly as-is; GitHub-specific fields (`ReviewDecision`, `WorkflowRun` details) become optional/nullable for forges that don't support them.
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
The `gh` CLI handles all GitHub authentication transparently. Forgejo profiles will use token-based auth. Token storage location in config is TBD (likely a `token_command` field that shells out, similar to how `gh` works, to avoid storing tokens in plaintext config).

**Rate limits:**
GitHub caps at 5,000 req/hr (enforced via semaphore in `internal/gh/gh.go`). Forgejo instance limits are admin-configurable. The concurrency model will be per-provider, configured at the `Provider` level.

**Clone mechanism:**
`grove clone` uses `gh repo clone` which wraps git with GitHub auth. Forgejo will use `git clone` with token-based HTTPS or SSH. Abstracted via `Provider.CloneRepo()`.

**Labels, assignees, milestones on issues:**
Forgejo's model is very close to GitHub's. Mappable to a shared model with minimal loss of fidelity. GitLab is out of scope for now.

## Implementation Plan

1. **Phase 1:** Extract `Provider` interface in `internal/forge/forge.go`, refactor `internal/gh/` into `GitHubProvider`
2. **Phase 2:** Implement `ForgejoProvider` using Forgejo v1 REST API, add `forge` and `instance_url` config fields
3. Validate against Codeberg (public Forgejo instance) as the primary test target

## Consequences

- Grove becomes useful to Forgejo, Codeberg, and self-hosted users
- The `gh` package becomes testable via mock injection
- Config profiles gain `forge` and `instance_url` fields (breaking change to config schema)
- GitHub-specific fields (`ReviewDecision`, Actions-specific workflow metadata) become optional in the model
- Significant refactor across `internal/gh/`, `internal/model/`, `internal/app/commands.go`, and `internal/clone/`
- GitLab support is not in scope but the abstraction should not preclude it
