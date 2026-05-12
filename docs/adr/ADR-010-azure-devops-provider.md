# ADR-010: Azure DevOps provider support

**Status:** Accepted
**Date:** 2026-05-03
**Revision 2026-05-12:** Initial implementation uses Azure CLI authentication only.
**Applies to:** `internal/forge/`, `internal/config/config.go`, `internal/app/commands.go`, `internal/clone/`

## Context

Grove now has a forge abstraction (ADR-006), a Forgejo provider, and split remote
concerns for code/social/CI (ADR-009). The next likely provider outside the
GitHub/Forgejo family is Azure DevOps.

Azure DevOps is not just another GitHub-like forge:

- Repositories live under an organization and usually a project.
- Azure Repos is the code-hosting concern.
- Azure Boards work items are issue-like but not identical to GitHub/Forgejo issues.
- Azure Pipelines builds are CI-like but are modeled differently from GitHub Actions and Forgejo Actions.
- Authentication is handled through the Azure DevOps CLI for the initial provider.
- Clone URLs are returned by Azure DevOps metadata and may be HTTPS or SSH depending on organization policy.

This intersects with ADR-003 remote profiles. Remote discovery will need richer repo
metadata than the current `Provider.ListRepos` method returns, and Azure DevOps is
a good stress test for that interface because project/repository identity matters
more than a simple `owner/name` string.

## Decision

Add Azure DevOps as a provider named `azuredevops`. The first implementation is
intentionally Azure CLI-only: grove shells out to the `az` DevOps extension and
does not manage Azure DevOps PAT files directly. Authentication is delegated to
`az devops login`, configured Azure CLI credentials, or whatever credential
source the Azure DevOps CLI supports.

Azure DevOps config should extend the existing remote block model rather than
introducing a parallel config tree:

```yaml
profiles:
  - name: work-ado
    owner: my-organization
    forge: azuredevops
    instance_url: https://dev.azure.com/my-organization
    project: MyProject
    clone_proto: https
```

The `owner` field remains the organization/account name. A new `project` field is
needed because most Azure DevOps REST endpoints are scoped to
`/{organization}/{project}/...`.

Initial provider scope:

- List repositories in an organization/project.
- List pull requests for a repository.
- List branches for a repository.
- Clone using provider-returned clone URLs where possible.
- Build browser URLs for repo, branch, and commit targets.

Deferred provider scope:

- List recent builds/pipeline runs for a repository where Azure DevOps exposes a stable mapping.
- Build browser URLs for PR, build, and work item targets when the CLI returns stable links.

Issue/work-item support should be cautious:

- Grove's `Issue` model can represent a subset of Azure Boards work items, but work items are broader than issues.
- The first implementation may expose only work items linked to PRs/builds or query-backed open work items if configured later.
- Full Azure Boards query support is explicitly a follow-up unless a concrete workflow requires it.

## Provider Interface Implications

ADR-003 already notes that `Provider.ListRepos` returning `[]string` is too weak
for remote discovery. Azure DevOps reinforces that.

Before or alongside Azure DevOps, introduce provider capability and metadata types,
for example:

- `Capabilities() ProviderCapabilities`
- `ListReposDetailed(owner string, opts RepoListOptions) ([]RemoteRepo, error)`
- `CloneURL(repo RemoteRepo, proto string) string`

Capabilities should indicate whether a provider supports:

- repository metadata
- pull requests
- issues/work items
- CI/build runs
- branch listing
- clone URL discovery
- organization-wide listing versus project-scoped listing

The existing local-dashboard provider methods can remain for currently cloned
repos, but ADR-003 and Azure DevOps should converge on the richer metadata shape
instead of adding two incompatible extensions.

## Alternatives Considered

**Implement Azure DevOps immediately with the current Provider interface** --
accepted for an initial CLI-only local-repo provider. This gives usable PR,
branch, URL, and clone support without adding direct Azure DevOps PAT handling.
It may still be refactored once ADR-003 adds richer remote repo metadata.

**Wait until ADR-003 is implemented first** -- cleaner because remote discovery
will define the richer provider contract. The tradeoff is delaying Azure support
for existing local repos.

**Create a separate Azure-only integration path** -- rejected. It would bypass
the provider abstraction and make ADR-009's split remote concerns harder to
reason about.

## Consequences

- Config gains `project` on `Remote` / profile remote defaults.
- Cache keys must include `project`.
- URL parsing and repo identity need to handle `organization/project/repo`, not only `owner/repo`.
- Work items should not be treated as exact GitHub issue equivalents without
  explicit mapping decisions.
- ADR-003 should still be implemented with provider capabilities so Azure DevOps can
  participate without special-casing the TUI.
- Tests use mocked `az` command fixtures because most Azure DevOps behavior
  cannot be validated through a public unauthenticated target.

## References

- Azure DevOps REST API overview: https://learn.microsoft.com/en-us/azure/devops/integrate/how-to/call-rest-api
- Azure DevOps Git repositories API: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/repositories/list
- Azure DevOps pull requests API: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests
- Azure DevOps builds API: https://learn.microsoft.com/en-us/rest/api/azure/devops/build/builds/list
