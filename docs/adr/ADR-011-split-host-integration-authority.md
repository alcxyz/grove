# ADR-011: Split-host integration authority

**Status:** Accepted
**Date:** 2026-05-03
**Applies to:** repository workflow, release workflow, split remote concern configuration

## Context

ADR-009 allows one local repo to use different forges for code, social data, and CI. That matches the current migration state: Forgejo is the primary git host for most code, while a smaller set of repositories still uses GitHub for PRs, issues, CI, GitHub Releases, Homebrew, AUR, or GitHub Pages.

This split creates a workflow trap. If the same `dev -> main` change is merged once on GitHub and again on Forgejo, each forge creates a different merge commit for the same tree. The resulting history is functionally equivalent but operationally messy:

- both hosts show separate PRs for one integration
- `main` briefly points at different commits on different hosts
- CI can run more than once for the same effective change
- release automation becomes harder to reason about
- subsequent mirroring requires manual cleanup

## Decision

Each repository has exactly one **integration authority** for `main`.

For GitHub-fronted repositories, GitHub is the integration authority. This includes repositories where PR review, issues, CI, releases, GitHub Pages, or distribution automation still live on GitHub. These repositories may still use Forgejo as the primary code remote for day-to-day pushes, but `main` is integrated on GitHub because GitHub owns the checks and release side effects.

For Forgejo-first repositories, Forgejo is the integration authority.

The non-authoritative host is mirror-only for `main` and `dev`:

- push working branches such as `dev` to both hosts when needed for visibility or PR checks
- open and merge the `dev -> main` PR only on the integration authority
- after the authoritative merge, fetch that `main`
- fast-forward the mirror host's `main` to the exact authoritative commit
- fast-forward `dev` to the same commit on both hosts when the repo uses long-lived `dev`
- do not open a second PR on the mirror host for the same integration
- do not force-push except as an explicit break-glass repair after inspecting divergence

Mirror branch protection must allow the mirroring identity to push to protected `main` without force. For Forgejo, keep `main` protected, but enable a push whitelist for the maintainer or deploy identity that performs fast-forward mirrors. If branch protection rejects the fast-forward push, fix the protection rule before falling back to a mirror PR.

For a GitHub-fronted repository, the normal sync after a GitHub PR merge is:

```sh
git fetch github main dev --tags
git fetch origin main dev --tags
git switch dev
git merge --ff-only github/main
git push origin github/main:main
git push origin dev
git push github dev
```

For a Forgejo-first repository with GitHub as a mirror, swap `origin` and `github` in the authoritative and mirror roles.

If any fast-forward push or merge is rejected, stop and inspect the divergence. A rejection means the mirror host has commits that are not descendants of the authoritative integration commit, and blindly continuing would recreate the duplicate-merge problem this ADR avoids.

## Consequences

- There is one PR record per integration, on the host that owns review and checks.
- There is one merge commit per integration.
- Release automation is triggered from the authoritative host only.
- Mirror hosts remain useful for browsing and cloning, but they do not create integration commits.
- Mirror branch protection has to allow controlled fast-forward pushes.
- Grove's split remote concerns describe where data comes from; they do not imply that every configured forge should receive its own PR merge.
- Automation should prefer fast-forward mirroring commands over API-created mirror PRs.

## Alternatives Considered

**Open matching PRs on both hosts** - rejected because it creates separate merge commits for the same tree and makes release state harder to reason about.

**Force-push the mirror to match the authority** - rejected for normal workflow because it hides divergence. Fast-forward-only updates are safer and surface real conflicts.

**Make Forgejo authoritative for all code even when GitHub owns CI and releases** - rejected for GitHub-fronted repositories because the release and distribution side effects currently run from GitHub `main`.

**Move every repository fully to Forgejo immediately** - desirable long term for Forgejo-first projects, but not true for GitHub Pages, forks, and projects whose issues, PRs, pipelines, or distribution channels intentionally remain on GitHub.
