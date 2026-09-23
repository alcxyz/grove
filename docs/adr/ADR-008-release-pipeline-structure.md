# ADR-008: Release pipeline structure

**Status:** Accepted
**Date:** 2026-04-29 (revised 2026-09-23)
**Applies to:** `.github/workflows/ci.yml`, `flake.nix`

## Context

All Go TUI projects (grove, canopy, paperflow) share the same release toolchain: GoReleaser for binaries + Homebrew + AUR, and a Nix flake for NixOS/home-manager consumers. The Nix flake uses `buildGoModule` which requires a `vendorHash` — a SHA256 of the Go module dependencies. This hash changes whenever `go.mod`/`go.sum` change and must be updated manually, which broke the Nix build after the v0.8.0 release.

A monolithic release job also meant that a Nix build failure could block or complicate the binary release, and vice versa.

## Decision

The CI pipeline is structured as four jobs:

1. **Promotion policy** — allows pull requests to `main` only from this
   repository's `dev` branch, with a new plain-semver `VERSION` that does not
   already have a tag. Development pull requests are unaffected.
2. **Check** — runs on pushes to `dev` and `main`, and on pull requests to
   either branch. It builds, vets, checks formatting, lints, and runs tests with
   the race detector.
3. **Release** — runs after Promotion policy and Check pass, on pull requests
   (snapshot only) and on `main` pushes (auto-tag + GoReleaser). It produces
   binaries, a GitHub Release, a Homebrew tap update, and an AUR package.
4. **Nix** — runs after Release succeeds on promotion pull requests and `main`
   pushes. It verifies the Nix package without changing protected branches. A
   stale `vendorHash` must be corrected on `dev` before promotion.

Grove, canopy, and paperflow share the Check, Release, and Nix structure.
Grove also requires its promotion policy because `dev` is its explicit
development branch and `main` is release-only. The `release.yml` fallback
workflow (triggered by manual tag push) is removed; `ci.yml` handles everything.

ADR-011 identifies GitHub as Grove's integration authority. The `main` push
auto-tag flow therefore runs against the same protected history that passed the
promotion checks.

## Alternatives Considered

**Monolithic release job with inline Nix step**: simpler YAML, but Nix installation adds ~60s to every release even when the hash hasn't changed. A failure in vendorHash computation blocks the binary release. Rejected because distribution channels should be independent.

**Committed vendor/ directory with `vendorHash = null`**: eliminates the hash problem entirely, but adds significant repo bloat for projects with many transitive dependencies. Rejected for cleanliness.

**gomod2nix**: generates a `gomod2nix.toml` mapping file instead of a single hash. More precise but adds a flake input dependency and changes the build mechanism from `buildGoModule` to `buildGoApplication`. Rejected as overkill for single-binary projects.

**Pre-commit hook to update vendorHash**: requires Nix installed on developer machines and runs on every commit. Rejected as too invasive for contributors without Nix.

## Consequences

- Binary releases (GitHub, Homebrew, AUR) are never blocked by Nix issues.
- Nix consumers receive releases whose `vendorHash` was validated before the
  promotion merged.
- The shared Check, Release, and Nix job names reduce maintenance across the Go
  projects while Grove's promotion policy enforces its branch contract.
- CI requires the `DeterminateSystems/nix-installer-action` in the Nix job, adding ~30s of setup time.
- A Nix failure blocks a promotion before release. The post-merge run verifies
  the exact released commit without trying to push through branch protection.
- `main` cannot accept a feature branch or an unchanged/reused release version
  when its required protection checks are enabled.
