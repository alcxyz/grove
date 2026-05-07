# Contributing to grove

## Development setup

Prerequisites: Go 1.26+. GitHub-backed profiles use `auth_mode: token` by default, or the [`gh` CLI](https://cli.github.com/) when configured with `auth_mode: gh`. Forgejo-backed profiles use token auth via `token_file`.

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
5. Open or import the change on the repository's integration authority

CI runs build, vet, and tests. All checks must pass before merging.

## Split-host workflow

GitHub can be used for public reach, issue intake, PR suggestions, and releases without being trusted as the integration authority.

Do not merge the same `dev -> main` change on more than one host. Merge once on the configured integration authority, then fast-forward mirrors to that exact commit.

For a Forgejo-authoritative repo with GitHub as public mirror:

```bash
git fetch origin main dev --tags
git fetch github main dev --tags
git switch dev
git merge --ff-only origin/main
git push origin dev
git push github origin/main:main
git push github dev
```

If any fast-forward step is rejected, stop and inspect the divergence before doing anything else.

GitHub PRs are acceptable as public patch suggestions, but they should not be merged on GitHub unless that repository explicitly chooses GitHub as its integration authority. Accepted GitHub PRs should be fetched, reviewed, and applied through the authoritative host or a trusted local clone.

## Commit messages

Use conventional-ish prefixes to keep history scannable:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `chore:` maintenance, CI, dependencies
- `refactor:` code changes that don't add features or fix bugs

## Releasing

Releases are automated via [GoReleaser](https://goreleaser.com/) and GitHub Actions. The `VERSION` file is the single source of truth.

To cut a release:

1. Bump the `VERSION` file on `dev`
2. Merge `dev -> main` on the integration authority
3. Create or push the release tag from the integration authority or a trusted local clone
4. Push the trusted tag to GitHub if GitHub Releases/Homebrew/AUR are used as distribution surfaces

This builds binaries for linux/darwin x amd64/arm64, creates a GitHub release with changelog, updates the [Homebrew tap](https://github.com/alcxyz/homebrew-tap), and publishes to the [AUR](https://aur.archlinux.org/packages/grove-tui-bin) (`grove-tui-bin`).

The release automation may need further changes so GitHub consumes trusted tags instead of creating authoritative tags itself.

### Version numbering

Follow [semver](https://semver.org/):

- **Patch** (`v0.5.x`): bug fixes, minor tweaks
- **Minor** (`v0.x.0`): new features, non-breaking changes
- **Major** (`vx.0.0`): breaking changes to config format, CLI flags, or behavior

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
