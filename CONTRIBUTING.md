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
5. Open a GitHub pull request targeting `dev`
6. Squash-merge after all checks pass

CI runs build, vet, and tests. All checks must pass before merging.

## Repository workflow

GitHub is Grove's source of truth for branches, pull requests, CI, and
releases. `dev` is the default development branch. Protected `main` accepts
only same-repository `dev` promotion pull requests with a new release version.
Secondary mirrors copy the resulting history; do not merge the same change on
another host.

## Commit messages

Use conventional-ish prefixes to keep history scannable:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `chore:` maintenance, CI, dependencies
- `refactor:` code changes that don't add features or fix bugs

## Releasing

Releases are automated via [GoReleaser](https://goreleaser.com/) and GitHub Actions. The `VERSION` file is the single source of truth.

`main` and release tags use plain `x.y.z` versions. Development builds identify
their source revision: ordinary `go build` output uses
`dev-<commit>[-dirty]`, while branch-based Nix packages use
`X.Y.Z-dev.<commit>[.dirty]`. GoReleaser artifacts retain the plain `X.Y.Z`
release version; when `VERSION` is plain semver, an intentional Nix release
build uses `.#release` from clean, identified release source.

To cut a release:

1. Bump the `VERSION` file on `dev` to a new plain release version like `0.10.0`
2. Open a same-repository pull request from `dev` to `main`
3. Wait for the promotion policy, code checks, and release snapshot to pass
4. Squash-merge the promotion; CI creates the tag and publishes the release
5. Merge `main` back into `dev`, then bump to the next development version like `0.10.1-dev`

This builds binaries for linux/darwin x amd64/arm64, creates a GitHub release with changelog, updates the [Homebrew tap](https://github.com/alcxyz/homebrew-tap), and publishes to the [AUR](https://aur.archlinux.org/packages/grove-tui-bin) (`grove-tui-bin`).

### Version numbering

Follow [semver](https://semver.org/):

- **Patch** (`v0.5.x`): bug fixes, minor tweaks
- **Minor** (`v0.x.0`): new features, non-breaking changes
- **Major** (`vx.0.0`): breaking changes to config format, CLI flags, or behavior

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
