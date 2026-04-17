# grove

A terminal UI for monitoring GitHub repositories. See branch status, dirty working trees, ahead/behind counts, open PRs, recent branches, and commit activity across all your repos without leaving the terminal.

```
               {o,o}
   ___ _ __ ___|)_)|___  ___
  / _ \ '__/  _ \ \ / / _ \
 | (_| | | | (_) \ V /  __/
  \__, |_|  \___/ \_/ \___|
  |___/
  one repo to rule them all
```

## Features

- **Multi-profile**: define any number of profiles (personal, work org, etc.) and switch instantly with `H` / `L` or by clicking the profile tab bar; an **All** view merges every profile at once
- **Clone**: `grove clone` enumerates all repos for each profile's GitHub owner and clones any that are missing locally; groups can route clones to separate subdirectories
- **Dashboard** (tab 1): all repos in one view with branch, dirty/clean state, sync status, open PR count, branch count, CI status, last author, and last commit time
- **Pull Requests** (tab 2): open PRs across all repos with review status and checks
- **CI** (tab 3): recent GitHub Actions workflow runs across all repos with pass/fail/running status
- **Branches** (tab 4): all remote branches with PR and merge indicators
- **Activity** (tab 5): recent commits across repos with inline diff viewer
- **Detail pane**: full repo detail with local/remote branches, open PRs, CI runs, recent commits, and stats; item-level cursor with contextual actions per item type
- **Diff viewer**: scrollable inline `git show` output with syntax colouring; respects your configured diff pager (`delta`, `bat`); navigate between commits with `[` / `]` and between files with `{` / `}`
- **External tools**: `space` opens diffnav (commits) or lazygit (repos) based on context; `e` opens `$EDITOR` / nvim at repo root
- **Grouped / flat view**: toggle between config-defined groups and a flat sorted list
- **Cycle filters**: quickly narrow by author, subject prefix, repository, or date bucket
- **Sort**: ascending/descending by date, author, subject/name, or repository
- **Block-jump navigation**: jump between repo blocks `[ ]`, subject blocks `( )`, or config groups `{ }`
- **Mouse support**: scroll wheel, click to select, tab-bar clicks, double-click to open
- **Profile persistence**: reopens on the same profile you left
- **Screensaver**: bouncing logo after a configurable idle timeout
- **Update notifications**: footer shows when a newer release is available
- Catppuccin Mocha colour palette

## Installation

### Homebrew

```sh
brew tap alcxyz/tap
brew install grove
```

### Nix

```sh
nix profile install github:alcxyz/grove
```

Or in a flake:

```nix
inputs.grove.url = "github:alcxyz/grove";
```

### AUR (Arch Linux)

```sh
yay -S grove-tui-bin
```

### Build from source

Requires Go 1.22+ and the [gh](https://cli.github.com/) CLI authenticated (`gh auth login`).

```sh
git clone git@github.com:alcxyz/grove.git
cd grove
go build -o grove ./cmd/grove
mv grove ~/.local/bin/
```

## Configuration

On first run grove writes an example config to `$XDG_CONFIG_HOME/grove/config.yaml` (usually `~/.config/grove/config.yaml`). Edit it to match your setup:

```yaml
profiles:
  - name: Work
    owner: my-org          # GitHub org or your username — drives PR/branch tabs
    base_paths:
      - ~/dev/git/my-org
    prefixes:
      - service-
      - platform-
    groups:
      - name: Services
        match: service-
      - name: Platform
        match: platform-

  - name: Personal
    owner: my-github-username
    base_paths:
      - ~/dev/git/personal
    # prefixes omitted → all git repos in base_paths are scanned

refresh_secs: 300
screensaver_secs: 300
```

The legacy single-profile format (`base_path`, `org`, `prefixes` at the top level) is still supported and auto-migrates to a single profile.

You can also pass one or more paths on the command line to override the first profile's `base_paths`:

```sh
grove ~/dev/git/other-org
grove ~/dir1 ~/dir2
```

### Files

| Path | Purpose |
|------|---------|
| `$XDG_CONFIG_HOME/grove/config.yaml` | Config (falls back to `~/.grove.yaml`); profile-based format |
| `$XDG_CACHE_HOME/grove/` | Cached PR / branch / activity / CI run data + UI state |
| `$XDG_STATE_HOME/grove/grove.log` | Runtime log |

## Key bindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `h` / `l` | Previous / next tab |
| `H` / `L` | Previous / next profile (cycles through All) |
| `gg` / `G` | First / last item |
| `tab` / `shift+tab` | Jump 5 / 10 / 20 / 25 lines (accelerates on rapid press) |
| `{ }` | Jump between config groups |
| `[ ]` | Jump between repo blocks |
| `( )` | Jump between CI status blocks (tab 1) / subject / branch / message blocks |
| `1` `2` `3` `4` `5` | Switch to tab directly |

### Filters and sort

| Key | Action |
|-----|--------|
| `/` | Open text filter |
| `esc` | Clear active filter / close pane |
| `d` / `D` | Cycle by author, sort by author |
| `s` / `S` | Cycle by subject prefix, sort by name / title |
| `a` / `A` | Cycle by repository, sort by repository |
| `f` / `F` | Cycle by date, sort by date / updated |
| `x` / `X` | Cycle / sort by **PR count** (tab 1) / **review status** (tab 2) / **has-PR** (tab 4) |
| `c` / `C` | Cycle / sort by **branch count** (tab 1) / **merged** (tab 4) / **branch prefix** (tab 3) |
| `v` / `V` | Cycle / sort by **CI status** (tabs 1, 3) / **checks result** (tab 2) |

The `x` / `c` / `v` keys follow the spatial layout of the columns they target (PR / Br / CI on the dashboard).

Date buckets: today, yesterday, this week, last week, this month, last month, this quarter, last quarter.

### Actions

| Key | Action |
|-----|--------|
| `enter` | Open detail pane (tabs 1-4) / open diff (tab 5) |
| `o` | Open on GitHub in browser (all tabs and views) |
| `space` | Per-tab tool: gh-dash (PRs), checkout + lazygit (branches), workflow in editor (CI), diffnav (activity), lazygit (dashboard) |
| `e` | Open `$EDITOR` / nvim at repo root (all tabs and views) |
| `p` | `git pull` current repo (all tabs) |
| `r` | Refresh current tab |
| `R` | Toggle auto-refresh |
| `ctrl+f` | `git fetch` all repos |
| `g` (single, 400ms) | Toggle grouped / flat view |
| `?` | Toggle help overlay |
| `!` | About: version, config path, cache and log locations |
| `q` / `ctrl+c` | Quit |

### Detail pane

The detail pane opens with `enter` on any tab and shows full repo info with an item-level cursor:

| Key | Action |
|-----|--------|
| `j` / `k` | Select next / previous item |
| `{ }` | Jump between sections (branches, PRs, CI, commits) |
| `[ ]` | Previous / next repo (follows source tab, skips duplicates) |
| `gg` / `G` | First / last item |
| `space` | Contextual: diffnav (commits), gh-dash (PRs), checkout + lazygit (branches), workflow editor (CI) |
| `o` | Contextual: open item on GitHub (PR URL, commit, branch, CI run) |
| `e` | Open editor at repo root |
| `esc` | Close detail pane |

### Diff view

The diff view opens with `enter` on the Activity tab:

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll up / down |
| `{ }` | Jump between files in the diff |
| `[ ]` | Previous / next commit |
| `gg` / `G` | Top / bottom |
| `space` | Open in diffnav |
| `o` | Open commit on GitHub |
| `e` | Open editor at repo root |
| `esc` | Close diff view |

## CI / GitHub Actions

Tab 3 shows recent GitHub Actions workflow runs across all repos:

| Column | Meaning |
|--------|---------|
| Repository | Short repo name |
| Workflow | Workflow file display name |
| Branch | Branch the run was triggered on |
| Status | `✓ success` / `✗ failure` / `● in_progress` / `⊘ cancelled` |
| Event | Trigger event (`push`, `pull_request`, `schedule`, etc.) |
| When | Time since last update |

`o` opens the run on GitHub. `enter` opens the repo's detail pane. `r` refreshes. Filtering and sorting (`s`/`S` workflow, `a`/`A` repo, `f`/`F` date) all work as on other tabs.

The **Dashboard** tab (tab 1) also shows a compact CI status icon (`✓` / `✗` / `●` / `—`) in the `CI` column, reflecting the latest run for each repo.

### Dashboard indicators

| Column | Meaning |
|--------|---------|
| `PR` | Open PR count, colour scales blue to yellow to red |
| `Br` | Branch count, colour scales blue to yellow to red |
| `CI` | Latest CI run: `✓` success / `✗` failure / `●` running / `—` no data |
| `●` (tab 4) | Branch has an open PR |
| `∈` (tab 4) | Branch is merged into the default branch |
| `Checks` (tab 2) | PR status check rollup: `✓ pass` / `✗ fail` / `● pending` / `—` none |

## External tools

Grove hands off to external tools via the `space` and `e` keys:

| Context | `space` opens | `e` opens |
|---------|---------------|-----------|
| Dashboard | lazygit | `$EDITOR` / nvim |
| PRs tab | gh-dash (from repo dir) | `$EDITOR` / nvim |
| Branches tab | checkout branch + lazygit (restores branch on exit) | `$EDITOR` / nvim |
| CI tab | workflow `.yml` in editor | `$EDITOR` / nvim |
| Activity tab | diffnav | `$EDITOR` / nvim |
| Detail pane (commit) | diffnav | `$EDITOR` / nvim |
| Detail pane (PR) | gh-dash | `$EDITOR` / nvim |
| Detail pane (branch) | checkout + lazygit | `$EDITOR` / nvim |
| Detail pane (CI run) | workflow `.yml` in editor | `$EDITOR` / nvim |
| Detail pane (local branch) | lazygit | `$EDITOR` / nvim |
| Diff view | diffnav | `$EDITOR` / nvim |

If a tool is not found on `PATH` or there is no valid target (no GitHub URL, no repo selected), a status message is shown instead of failing silently.

## Mouse

| Action | Effect |
|--------|--------|
| Scroll wheel | Scroll 3 lines per tick (works in detail and diff views too) |
| Click tab bar | Switch tab |
| Click profile bar | Switch profile |
| Click row | Select item |
| Double-click row | Open (same as `enter`) |

## Caching

Grove caches GitHub API responses to disk so the UI opens instantly and remains usable while refreshes happen in the background.

**What is cached**

| File | Content | Cap |
|------|---------|-----|
| `prs.json` | Open pull requests | 500 items |
| `branches.json` | Remote branches | 2 000 items |
| `activity.json` | Recent commits | 100 items |
| `runs.json` | CI workflow runs | 500 items |
| `state.json` | UI state (active profile) | — |

Each data file is a JSON object `{ "cached_at": <RFC3339>, "config_key": <string>, "data": [...] }`.

**Startup** — all four cache files are read before the TUI launches. The UI renders immediately with the cached data; fresh data loads in the background and replaces it without any visual flicker. The last active profile is restored from `state.json`.

**TTL** — controlled by `refresh_secs` in config (default 300 s). On startup and on every tab switch, grove checks whether the data for that tab is older than the TTL. If so, a background fetch is triggered automatically. Auto-refresh (toggled with `R`) repeats this on a timer.

**Cache invalidation** — each file stores a `config_key` derived from the owner names and prefixes of all configured profiles. If the config changes (new profile, different owner, changed prefixes), the key changes and all cached data is treated as a miss, forcing a full refresh on next launch.

**Forcing a refresh** — press `r` to refresh the active tab immediately, or `ctrl+f` to `git fetch` all repos.

## Rate limiting

All GitHub API calls go through the `gh` CLI. To avoid hitting GitHub's rate limits when scanning many repos, grove limits concurrent `gh` invocations to **5 at a time** (a buffered semaphore channel in `internal/gh`). This applies to PR listing, branch listing, and CI run listing.

`grove clone` uses a separate semaphore capped at **8 concurrent clones**, since `git clone` is network-bound rather than API-bound and GitHub's clone rate limits are more permissive.

## Clone

`grove clone` enumerates all repos for each profile's GitHub owner via the GitHub API and clones any that are not already present locally:

```sh
grove clone                  # all profiles
grove clone Personal         # one profile by name
grove clone Work Personal    # multiple profiles
```

Repos are cloned into the profile's `base_paths[0]` by default. If a group has its own `base_path`, repos matching that group's prefix are cloned there instead:

```yaml
groups:
  - name: Services
    match: service-
    base_path: ~/dev/git/my-org/services  # cloned here
  - name: Platform
    match: platform-
    # no base_path → falls back to profile base_paths[0]
```

Existing repos (detected by the presence of a `.git` directory) are skipped. Up to 8 clones run in parallel.

## Releasing

Releases are automated. To publish a new version:

1. Bump the `VERSION` file
2. Merge to `main`

CI runs tests, creates a git tag from `VERSION`, and triggers goreleaser which builds binaries and publishes to GitHub Releases, Homebrew, and AUR.

## License

MIT. See [LICENSE](LICENSE).
