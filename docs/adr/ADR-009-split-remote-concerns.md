# ADR-009: Split remote concerns per profile, group, and repo

**Status:** Accepted
**Date:** 2026-05-02
**Applies to:** `internal/config/config.go`, `internal/app/commands.go`, `internal/app/update.go`, `internal/clone/clone.go`, `cmd/grove/config.example.yaml`

## Context

ADR-006 introduced multi-forge support at the profile level: one profile, one forge provider, one owner. That was enough for "all repos in this profile live on GitHub" or "all repos in this profile live on Forgejo", but it does not match the user's actual setup.

The real-world migration pattern is mixed:

- code hosting and primary git remotes have moved to Forgejo
- some repos still keep pull requests, issues, or CI on GitHub
- some groups of repos (for example forks or Pages repos) share the same exception policy
- some nested repos need repo-specific exceptions inside an otherwise consistent directory tree

Treating the entire profile as one remote forces bad compromises:

- Forgejo-first repos get incorrectly queried against GitHub
- GitHub-frontfacing repos need to be split into artificial extra profiles
- the browser/open actions, clone behaviour, PRs/issues, and CI all assume the same forge even when they should not

ADR-003 remote profiles are out of scope here. This is about already-cloned local repos whose remote concerns differ.

Operationally, split concerns do not mean the same change should be integrated on every configured forge. ADR-011 defines the integration authority rule: one explicit authority creates the `main` merge commit, while GitHub or other public surfaces may remain social, CI, release, or intake mirrors.

## Decision

Split remote configuration into three concerns:

- **Code**: repository URL, commit URL, branch URL, branch listing, and `grove clone`
- **Social**: pull requests and issues
- **CI**: workflow / pipeline runs

The top-level profile fields (`owner`, `forge`, `instance_url`, `token_file`, `auth_mode`, `clone_proto`) remain the default **code** remote.

Profiles gain:

- `social`
- `ci`
- `repos`

Groups gain:

- `code`
- `social`
- `ci`

`repos` entries gain:

- `code`
- `social`
- `ci`

Each remote block also supports `repo`, an optional remote repository name.
When omitted, grove uses the local directory name as before. This handles
cases such as a local checkout named `annaetattoo` whose GitHub Pages remote is
`annaetattoo.github.io`.

Each remote block also supports `ssh_host`, an optional SSH clone host used when
`clone_proto: ssh` and the SSH host differs from the forge web/API host.

Resolution order is:

1. profile defaults
2. first matching group override
3. per-repo override

This keeps the common case compact while allowing directory-wide exceptions and one-off repo exceptions.

## Consequences

- Forgejo-first profiles can stay simple while still preserving a few GitHub-frontfacing repos
- forks or Pages-style repos can be handled as group policies instead of duplicating repo entries
- `grove clone` follows the code-hosting remote instead of assuming the PR/issues/CI source is the same
- cache invalidation must include concern-specific remote config, not just owner names and prefixes
- API calls and browser URLs can target a remote repo name that differs from the local checkout directory
- SSH clone URLs no longer have to guess the SSH host from `instance_url`
- A repo with split concerns still needs one integration authority for `main`; the configured code, social, and CI remotes are data sources, not instructions to trust or merge on every forge

## Alternatives Considered

**Keep one forge per profile** — rejected because it does not fit the mixed GitHub/Forgejo reality without exploding the number of profiles.

**Per-repo overrides only** — rejected because it is too verbose for directory-wide policies such as forks or nested plugin repos.

**Infer the correct forge from git remotes automatically** — rejected because local remotes do not encode where PRs, issues, or CI are actually frontfacing. A repo can have both `origin` and `github` remotes while only one concern remains active on GitHub.
