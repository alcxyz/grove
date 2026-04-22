# ADR-001: Path-based group matching

**Status:** Accepted
**Date:** 2026-04-22
**Applies to:** `internal/config/config.go`

## Context

Groups in grove config control how repos are visually grouped in the dashboard, and how clone routes repos to directories. The only matching strategy was name-based: `match` checked whether a repo name started with or contained a given substring.

This worked well for orgs with naming conventions (e.g. `service-api`, `platform-web`), but not for personal profiles where repos with unrelated names are organised by filesystem directory (e.g. `~/nix/`, `~/gitops/`, `~/dev/git/user/`). Users had to either accept most repos landing in "other" or adopt artificial naming conventions.

## Decision

Add a `match_path` field to the `Group` config struct. When set, it matches if the repo's absolute filesystem path starts with the configured path. This runs alongside the existing `match` field -- either one matching is sufficient to assign the repo to that group. First matching group wins (existing behaviour preserved).

`GroupFor` now accepts both `repoName` and `repoPath`. The navigation layer builds a name-to-path lookup from the loaded repos and wraps `GroupFor` in a closure so the `Build*Groups` callback signature (`func(string) string`) stays unchanged.

`match_path` does not affect clone routing -- `cloneDestFor` still uses `match` and `base_path`, since repos being cloned don't have a local path yet.

## Alternatives Considered

**Make `match` a general pattern language (globs, regex, `path:` prefix).** Rejected because it overloads one field with multiple semantics and makes the config harder to read. A dedicated `match_path` field is explicit and self-documenting.

**Change `GroupFor` callback to `func(name, path string) string` everywhere.** Rejected because it would require changing all `Build*Groups` functions and every call site. Using a closure in the navigation layer keeps the change localised.

## Consequences

- Users can group repos by directory without relying on naming conventions.
- `match` and `match_path` can coexist on the same group (OR semantics).
- The `GroupFor` method signature changed from one to two parameters; any external callers (none currently) would need updating.
- Clone routing is unaffected; `match_path` is purely a display-grouping concern.
