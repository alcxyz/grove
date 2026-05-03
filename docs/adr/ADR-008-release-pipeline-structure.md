# ADR-008: Release pipeline structure

**Status:** Accepted
**Date:** 2026-04-29 (revised 2026-05-03)
**Applies to:** `.github/workflows/ci.yml`, `flake.nix`

## Context

All Go TUI projects (grove, canopy, paperflow) share the same release toolchain: GoReleaser for binaries + Homebrew + AUR, and a Nix flake for NixOS/home-manager consumers. The Nix flake uses `buildGoModule` which requires a `vendorHash` — a SHA256 of the Go module dependencies. This hash changes whenever `go.mod`/`go.sum` change and must be updated manually, which broke the Nix build after the v0.8.0 release.

A monolithic release job also meant that a Nix build failure could block or complicate the binary release, and vice versa.

## Decision

The CI pipeline is structured as three sequential jobs:

1. **Check** — runs on every push to `dev` and on PRs to `main`. Build, vet, gofmt, golangci-lint, and race-detected tests.
2. **Release** — runs after Check passes, on PRs (snapshot only) and on main push (auto-tag + GoReleaser). Produces binaries, GitHub Release, Homebrew tap update, and AUR package.
3. **Nix** — runs after Release succeeds, only on main push. Verifies the Nix flake builds. If `vendorHash` is stale, computes the correct hash from the build error, updates `flake.nix`, and pushes the fix automatically.

All three projects (grove, canopy, paperflow) use this same structure. The `release.yml` fallback workflow (triggered by manual tag push) is removed — `ci.yml` handles everything.

ADR-011 revises the split-host trust model: GitHub can be a release and distribution surface without being the integration authority. The current GitHub `main` push auto-tag flow is therefore acceptable only for repositories that explicitly choose GitHub as integration authority. For repositories where Forgejo, Codeberg, or a trusted local clone owns protected history, the release flow should move toward trusted tags created outside GitHub and pushed to GitHub for distribution.

## Alternatives Considered

**Monolithic release job with inline Nix step**: simpler YAML, but Nix installation adds ~60s to every release even when the hash hasn't changed. A failure in vendorHash computation blocks the binary release. Rejected because distribution channels should be independent.

**Committed vendor/ directory with `vendorHash = null`**: eliminates the hash problem entirely, but adds significant repo bloat for projects with many transitive dependencies. Rejected for cleanliness.

**gomod2nix**: generates a `gomod2nix.toml` mapping file instead of a single hash. More precise but adds a flake input dependency and changes the build mechanism from `buildGoModule` to `buildGoApplication`. Rejected as overkill for single-binary projects.

**Pre-commit hook to update vendorHash**: requires Nix installed on developer machines and runs on every commit. Rejected as too invasive for contributors without Nix.

## Consequences

- Binary releases (GitHub, Homebrew, AUR) are never blocked by Nix issues.
- Nix consumers get a working `vendorHash` within minutes of a release, automatically.
- The pipeline is identical across all Go projects, reducing maintenance.
- CI requires the `DeterminateSystems/nix-installer-action` in the Nix job, adding ~30s of setup time.
- If the Nix job fails for reasons other than vendorHash (e.g., nixpkgs breakage), it surfaces as a separate failure that doesn't affect the release.
- Split-host repos must not infer release authority from GitHub reach alone; release automation should consume trusted history and tags from the integration authority.
