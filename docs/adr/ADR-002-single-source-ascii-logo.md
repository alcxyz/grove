# ADR-002: Single-source ASCII logo via go:embed

**Status:** Accepted
**Date:** 2026-04-22
**Applies to:** `internal/ui/detail.go`, `internal/ui/logo.txt`

## Context

The ASCII owl logo was duplicated: once as an inline Go string literal in `internal/ui/detail.go` and once in `README.md`. Any logo change required updating both locations manually, with no mechanism to detect drift.

## Decision

Store the logo in a single file (`internal/ui/logo.txt`) and load it at compile time using Go's `//go:embed` directive. The README retains a visual copy with an HTML comment pointing to the canonical file.

## Alternatives Considered

- **CI check or pre-commit hook to sync README from logo.txt** — rejected as disproportionate machinery for a rarely-changing asset.
- **Remove logo from README and link to the file** — rejected because it degrades the README reading experience on GitHub.

## Consequences

The Go binary always reflects the contents of `logo.txt` at build time. The README copy must still be updated manually, but the comment makes the source of truth discoverable. This is an acceptable trade-off given how rarely the logo changes.
