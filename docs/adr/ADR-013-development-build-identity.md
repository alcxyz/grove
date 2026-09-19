# ADR-013: Identify development builds by source revision

**Status:** Accepted
**Date:** 2026-09-19
**Applies to:** `VERSION`, Go builds, Nix packaging, version output

## Context

Grove already used a `-dev` source version on its long-lived development branch,
but builds from different commits shared that version. Ordinary `go build`
binaries only reported `dev`, so installed QA builds could not be traced back to
their source.

## Decision

- Keep `VERSION` and the GoReleaser release flow unchanged. Normalize an
  existing `-dev` suffix before adding build identity.
- Nix branch builds use `X.Y.Z-dev.<12-character-commit>`, adding `.dirty` for
  modified source. Source without Git metadata reports `X.Y.Z-dev.unknown`.
  Nix flake metadata does not reliably retain the requested tag ref, so a
  deliberate `release` package is exposed only when `VERSION` is plain semver;
  it keeps stable `X.Y.Z` and rejects dirty or unidentified source.
- Ordinary `go build` binaries derive `dev-<12-character-commit>[-dirty]` from
  Go's embedded VCS metadata and fall back to `dev` when it is unavailable.
- An explicitly injected version always wins, preserving GoReleaser releases
  and ensuring the CLI, footer, and About view report the package identity.

## Alternatives and consequences

Writing hashes into `VERSION` would create source changes for every build and
interfere with release preparation. Keeping only the branch's shared `-dev`
version would remain ambiguous. The selected formats keep release tags stable,
make development artifacts traceable, and remain excluded from release update
checks. An explicit Nix release output avoids guessing provenance from a commit
shared by branches and tags.
