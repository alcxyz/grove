# ADR-011: Explicit integration authority and public intake mirrors

**Status:** Accepted
**Date:** 2026-05-03 (revised 2026-05-03)
**Applies to:** repository workflow, release workflow, split remote concern configuration

## Context

ADR-009 allows one local repo to use different forges for code, social data, and CI. That matches the current migration state: Forgejo is the primary git host for most code, while a smaller set of repositories still uses GitHub for public reach, issues, suggested PRs, GitHub Releases, Homebrew, AUR, or GitHub Pages.

The first version of this ADR treated GitHub-fronted repositories as GitHub-authoritative. That was too broad. GitHub can be useful as a public social and distribution surface without being trusted as the merge/history authority.

The trust concern is not just duplicate merge commits. If GitHub is treated as the authority, a GitHub-side history rewrite, bad merge, or platform bug can be copied into the authoritative history. A blind mirror would preserve the problem, not protect against it.

This split creates two workflow traps:

- merging the same `dev -> main` change once on GitHub and again on another forge creates different merge commits for the same tree
- blindly mirroring GitHub into Forgejo or Codeberg can replicate destructive or incorrect GitHub state

GitHub's public surface is still valuable. People can find projects, fork them, open issues, and suggest PRs there. The important distinction is that GitHub PRs can be an intake queue without being the merge queue.

## Decision

Each repository has exactly one explicit **integration authority** for `main`.

Integration authority is independent of social, CI, release, and clone concerns:

- `code` answers where the normal repository remote lives
- `social` answers where issues, discussions, and public PR suggestions are surfaced
- `ci` answers where checks or pipelines run
- `release` answers where release artifacts are published
- `integration` answers where `dev -> main` is actually merged and where protected history is trusted

GitHub may be configured as `social` or `release` without being `integration`.

For repositories where GitHub is not trusted as the integration authority:

- GitHub is a public intake mirror
- GitHub issues may remain open for reach
- GitHub PRs are treated as patch suggestions, not merge buttons
- accepted GitHub PRs are fetched, reviewed, and applied into `dev` on the authoritative host or local trusted clone
- `dev -> main` is merged only on the integration authority
- GitHub is updated from the authoritative history, not the other way around

If the user's private Forgejo is not intended for public collaboration, Codeberg is the preferred public collaboration authority for projects that need open contribution without trusting GitHub merges. In that model:

- private Forgejo remains the personal protected replica or working authority
- Codeberg owns public collaboration and integration for public contributors
- GitHub remains the reach/discovery mirror and optional release surface

The non-authoritative host is mirror-only for protected refs:

- push working branches such as `dev` to public surfaces only when useful for visibility or checks
- open and merge the `dev -> main` PR only on the integration authority
- after the authoritative merge, fetch that `main`
- fast-forward mirror `main` to the exact authoritative commit
- fast-forward `dev` to the same commit on both hosts when the repo uses long-lived `dev`
- do not open a second merge PR on a mirror host for the same integration
- do not force-push except as an explicit break-glass repair after inspecting divergence

Mirror branch protection must allow the mirroring identity to push to protected `main` without force, or mirroring must be performed by an explicit automation identity with the same restriction. If branch protection rejects a fast-forward push, fix the protection rule before falling back to a mirror PR.

For a Forgejo-authoritative repository with GitHub as a public mirror, the normal sync after an authoritative merge is:

```sh
git fetch origin main dev --tags
git fetch github main dev --tags
git switch dev
git merge --ff-only origin/main
git push origin dev
git push github origin/main:main
git push github dev
```

For a Codeberg-authoritative repository, replace `origin` with the Codeberg remote or make Codeberg the `origin` remote. Private Forgejo can then be updated as another fast-forward mirror.

For the exceptional case where a repository explicitly chooses GitHub as integration authority, swap the authoritative and mirror roles. This must be an explicit per-repository choice, not inferred from GitHub issues, PR suggestions, CI, or releases being enabled.

If any fast-forward push or merge is rejected, stop and inspect the divergence. A rejection means the mirror host has commits that are not descendants of the authoritative integration commit, and blindly continuing would recreate the duplicate-merge or trust problem this ADR avoids.

Tags are part of protected history:

- create tags from the integration authority or a trusted local clone
- push tags outward to public/distribution surfaces
- never overwrite mirror tags from GitHub
- do not use blind `git push --mirror` between hosts

## Consequences

- There is one authoritative PR or merge record per integration.
- There is one merge commit per integration.
- GitHub can keep public reach without being trusted for protected history.
- GitHub PRs become patch suggestions unless the repo explicitly selects GitHub as integration authority.
- Mirror hosts remain useful for browsing, cloning, issue intake, PR suggestions, and distribution, but they do not create integration commits.
- Mirror branch protection has to allow controlled fast-forward pushes.
- Grove's split remote concerns describe where data comes from; they do not imply that every configured forge should receive its own PR merge or be trusted as authority.
- Automation should mirror from an explicit authority to public surfaces with fast-forward-only checks.
- The current GitHub release pipeline may need to move from GitHub-created tags to trusted tags pushed from the integration authority before GitHub is reduced to a release surface only.

## Alternatives Considered

**Open matching PRs on both hosts** - rejected because it creates separate merge commits for the same tree and makes release state harder to reason about.

**Force-push or blind mirror to match GitHub** - rejected because it can replicate GitHub-side history corruption into the trusted host.

**Treat GitHub as authority whenever it owns social or release concerns** - rejected because social reach and distribution are not the same as protected-history trust.

**Disable GitHub entirely** - rejected for public projects because GitHub's reach, forks, issue visibility, and release/distribution ecosystem are still useful.

**Use Codeberg as public collaboration authority** - accepted as the preferred option for public projects that should not trust GitHub merges and cannot use the user's private Forgejo as the public collaboration surface.
