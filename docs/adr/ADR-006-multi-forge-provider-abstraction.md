# ADR-006: Multi-forge provider abstraction

**Status:** Proposed
**Date:** 2026-04-29
**Applies to:** `internal/gh/`, `internal/model/repo.go`, `internal/config/config.go`, `internal/app/commands.go`, `internal/clone/clone.go`

## Context

Grove is currently tightly coupled to GitHub at every layer of the stack. All data fetching goes through the `gh` CLI wrapper in `internal/gh/gh.go`, all model types in `internal/model/repo.go` assume GitHub semantics, and the `grove clone` subcommand calls the GitHub REST API directly.

Users increasingly self-host git forges — GitLab, Gitea, Forgejo (a Gitea fork) — alongside or instead of GitHub. Supporting these would make grove useful to a much broader audience. Even without adding new providers immediately, extracting a provider abstraction would improve testability (the current `gh` package cannot be mocked without exec-level patching).

The branding and about text have been updated to reflect the intent: grove is a "multi-repo git forge monitoring TUI" that is currently GitHub-native.

## Decision

Not yet decided. This ADR records the problem, the candidate approaches, and the open questions that need resolution before a direction can be chosen.

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

These need resolution before a final design can be accepted:

**CI runs:**
GitHub has Actions (`WorkflowRun`). GitLab has Pipelines. Gitea/Forgejo has its own Actions (GitHub Actions-compatible API, but different endpoints). A generic model would need either: a lowest-common-denominator `PipelineRun` type, or provider-specific extensions on a shared base.

**Review decisions:**
`ReviewDecision` (APPROVED / CHANGES_REQUESTED / REVIEW_REQUIRED) is GitHub-specific. GitLab has approvals but a different model. Forgejo has a similar concept but not identical. Can these map to a shared enum, or do they need to be optional/provider-specific?

**Org/user repo enumeration:**
`grove clone` calls `gh api orgs/<owner>/repos` or `users/<owner>/repos`. Each forge has a different REST path and pagination scheme. This is solvable but needs a per-provider implementation.

**Auth:**
The `gh` CLI handles all GitHub authentication transparently. Other forges would need token-based auth configured per profile. Where does this live in config, and how are tokens stored securely?

**Rate limits:**
GitHub caps at 5,000 req/hr (enforced via semaphore in `internal/gh/gh.go`). GitLab and Gitea have different limits (and Gitea/Forgejo instances are admin-configurable). The concurrency model would need to be per-provider.

**Clone mechanism:**
`grove clone` uses `gh repo clone` which wraps git with GitHub auth. Other forges would use plain `git clone` with a token embedded in the URL or SSH. The clone interface would need to abstract this.

**Labels, assignees, milestones on issues:**
These exist on GitHub, GitLab, and Gitea, but the schemas differ (GitLab labels are project-scoped objects; Gitea labels are similar to GitHub's). Probably mappable to a shared model with some loss of fidelity.

## Proposed Next Steps

1. Decide on alternative A, B, C, or D above
2. If B or C: sketch the `Fetcher` interface and validate it covers the open questions
3. If C: pick a second forge target (Forgejo is a strong candidate given its GitHub Actions compatibility)
4. Write a follow-up ADR once the interface design is settled

## Consequences

If a provider abstraction is adopted:
- Grove becomes useful to self-hosted Forgejo/Gitea/GitLab users
- The `gh` package becomes testable via mock injection
- Config profiles gain a `forge` or `provider` field (breaking change to config schema)
- Some GitHub-specific richness (ReviewDecision, Actions-specific fields) may be demoted to optional or provider-specific extensions
- Significant refactor across `internal/gh/`, `internal/model/`, `internal/app/commands.go`, and `internal/clone/`
