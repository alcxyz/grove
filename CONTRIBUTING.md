# Contributing to grove

## Development setup

Prerequisites: Go 1.26+. GitHub-backed profiles require the [`gh` CLI](https://cli.github.com/); Forgejo-backed profiles use token auth via `token_file`.

```bash
git clone https://github.com/alcxyz/grove.git
cd grove
go build ./cmd/grove
```

## Running tests

```bash
go test ./...
```

## Vetting

CI runs `go vet`. Run it locally to catch issues before pushing:

```bash
go vet ./...
```

## Project structure

- `cmd/grove/` -- entry point, config example
- `internal/app/` -- core TUI application (commands, filtering, navigation, update, view)
- `internal/cache/` -- API response caching
- `internal/clone/` -- `grove clone` command
- `internal/config/` -- YAML config loading
- `internal/gh/` -- GitHub API via `gh` CLI
- `internal/forge/` -- forge provider abstraction and provider implementations
- `internal/git/` -- git operations
- `internal/model/` -- repository model
- `internal/ui/` -- dashboard, detail pane, groups, tabs, styles

## Making changes

1. Create a branch from `dev`
2. Make your changes
3. Add or update tests as needed
4. Run `go test ./...` and `go vet ./...`
5. Push `dev` or your feature branch to GitHub when PR checks are needed
6. Open a pull request against `main` on GitHub

CI runs build, vet, and tests. All checks must pass before merging.

## Split-host workflow

Grove is Forgejo-primary for code hosting but GitHub-fronted for PRs, CI, releases, and distribution. GitHub is therefore the integration authority for `main`.

Do not merge the same `dev -> main` change on both GitHub and Forgejo. Merge once on GitHub, then fast-forward Forgejo to the exact GitHub `main` commit:

```bash
git fetch github main dev --tags
git fetch origin main dev --tags
git switch dev
git merge --ff-only github/main
git push origin github/main:main
git push origin dev
git push github dev
```

If any fast-forward step is rejected, stop and inspect the divergence before doing anything else.

Forgejo `main` is still protected, but the branch protection must allow the maintainer identity to push-whitelisted fast-forward mirrors. A protected-branch push rejection is a repo setting problem, not a reason to create a second Forgejo PR for the same change.

## Commit messages

Use conventional-ish prefixes to keep history scannable:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `chore:` maintenance, CI, dependencies
- `refactor:` code changes that don't add features or fix bugs

## Releasing

Releases are automated via [GoReleaser](https://goreleaser.com/) and GitHub Actions. The `VERSION` file is the single source of truth.

`main` and release tags use plain `x.y.z` versions. Long-lived development branches use non-release versions such as `0.9.2-dev`, so branch-based Nix builds and `grove --version` do not present themselves as released builds.

To cut a release:

1. Bump the `VERSION` file on `dev` to a plain release version like `0.9.2`
2. Open and merge the GitHub PR from `dev` to `main`
3. CI automatically creates the git tag and runs GoReleaser
4. Fast-forward the Forgejo mirror after GitHub `main` is green
5. Bump `dev` to the next non-release version like `0.9.3-dev`

This builds binaries for linux/darwin x amd64/arm64, creates a GitHub release with changelog, updates the [Homebrew tap](https://github.com/alcxyz/homebrew-tap), and publishes to the [AUR](https://aur.archlinux.org/packages/grove-tui-bin) (`grove-tui-bin`).

The old `release.yml` tag-triggered fallback was removed. `.github/workflows/ci.yml` is the release pipeline.

### Version numbering

Follow [semver](https://semver.org/):

- **Patch** (`v0.5.x`): bug fixes, minor tweaks
- **Minor** (`v0.x.0`): new features, non-breaking changes
- **Major** (`vx.0.0`): breaking changes to config format, CLI flags, or behavior

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
