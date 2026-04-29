# Contributing to grove

## Development setup

Prerequisites: Go 1.26+, [`gh` CLI](https://cli.github.com/) (grove uses it to talk to the GitHub API)

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
- `internal/git/` -- git operations
- `internal/model/` -- repository model
- `internal/ui/` -- dashboard, detail pane, groups, tabs, styles

## Making changes

1. Fork the repo and create a branch from `dev`
2. Make your changes
3. Add or update tests as needed
4. Run `go test ./...` and `go vet ./...`
5. Open a pull request against `dev`

CI runs build, vet, and tests. All checks must pass before merging.

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
2. Merge `dev` into `main`
3. CI automatically creates the git tag and runs GoReleaser

This builds binaries for linux/darwin x amd64/arm64, creates a GitHub release with changelog, updates the [Homebrew tap](https://github.com/alcxyz/homebrew-tap), and publishes to the [AUR](https://aur.archlinux.org/packages/grove-tui-bin) (`grove-tui-bin`).

The `release.yml` workflow also exists as a fallback for manually re-triggering a release by pushing a `v*.*.*` tag.

### Version numbering

Follow [semver](https://semver.org/):

- **Patch** (`v0.5.x`): bug fixes, minor tweaks
- **Minor** (`v0.x.0`): new features, non-breaking changes
- **Major** (`vx.0.0`): breaking changes to config format, CLI flags, or behavior

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
