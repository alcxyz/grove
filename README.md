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

- **Multi-profile**: define any number of profiles (personal, work org, etc.) and switch instantly with `<` / `>` or by clicking the profile tab bar; an **All** view merges every profile at once
- **Clone**: `grove clone` enumerates all repos for each profile's GitHub owner and clones any that are missing locally; groups can route clones to separate subdirectories
- **Dashboard**: all repos in one view with branch, dirty/clean state, sync status, open PR count, branch count, last author, and last commit time
- **Pull Requests**: open PRs across all repos with review status
- **Branches**: all remote branches with PR and merge indicators
- **Activity**: recent commits across repos with inline diff viewer
- **CI**: recent GitHub Actions workflow runs across all repos with pass/fail/running status; dashboard tab shows a per-repo CI status icon at a glance
- **Grouped / flat view**: toggle between config-defined groups and a flat sorted list
- **Cycle filters**: quickly narrow by author, subject prefix, repository, or date bucket
- **Sort**: ascending/descending by date, author, subject/name, or repository
- **Block-jump navigation**: jump between repo blocks `[ ]`, subject blocks `( )`, or config groups `{ }`
- **Detail pane**: full repo detail with local branches, open PRs, recent commits, and stats
- **Diff viewer**: inline `git show` output with syntax colouring; respects your configured diff pager (`delta`, `bat`)
- **Mouse support**: scroll wheel, click to select, tab-bar clicks, double-click to open
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
yay -S grove-bin
```

### Build from source

Requires Go 1.22+ and the [gh](https://cli.github.com/) CLI authenticated (`gh auth login`).

```sh
git clone git@github.com:alcxyz/grove.git
cd grove
go build -o grove .
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
| `$XDG_CACHE_HOME/grove/` | Cached PR / branch / activity / CI run data |
| `$XDG_STATE_HOME/grove/grove.log` | Runtime log |

## Caching

Grove caches GitHub API responses to disk so the UI opens instantly and remains usable while refreshes happen in the background.

**What is cached**

| File | Content | Cap |
|------|---------|-----|
| `prs.json` | Open pull requests | 500 items |
| `branches.json` | Remote branches | 2 000 items |
| `activity.json` | Recent commits | 100 items |
| `runs.json` | CI workflow runs | 500 items |

Each file is a JSON object `{ "cached_at": <RFC3339>, "config_key": <string>, "data": [...] }`.

**Startup** — all four cache files are read before the TUI launches. The UI renders immediately with the cached data; fresh data loads in the background and replaces it without any visual flicker.

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

## Key bindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `gg` | First item |
| `G` | Last item |
| `{ }` | Jump between config groups |
| `[ ]` | Jump between repo blocks |
| `( )` | Jump between CI status blocks (tab 1) / subject / branch / message blocks |
| `tab` / `shift+tab` | Next / previous tab |
| `1` `2` `3` `4` `5` | Switch to tab directly |
| `<` / `>` | Previous / next profile (cycles through All) |

### Filters and sort

| Key | Action |
|-----|--------|
| `/` | Open text filter |
| `esc` | Clear active filter / close pane |
| `d` / `D` | Cycle by author, sort by author |
| `s` / `S` | Cycle by subject prefix, sort by name / title / branch / subject |
| `a` / `A` | Cycle by repository, sort by repository |
| `f` / `F` | Cycle by date, sort by date / updated |

Date buckets: today, yesterday, this week, last week, this month, last month, this quarter, last quarter.

### Actions

| Key | Action |
|-----|--------|
| `enter` | Open detail pane (tab 1), open diff (tab 4), open run (tab 5) |
| `o` | Open PR in browser (tab 2, detail pane), open run (tab 5) |
| `p` | `git pull` current repo (tab 1) |
| `r` | Refresh current tab |
| `R` | Toggle auto-refresh |
| `ctrl+f` | `git fetch` all repos |
| `g` (single, 400ms) | Toggle grouped / flat view |
| `?` | Toggle help overlay |
| `!` | About: version, config path, cache and log locations |
| `q` / `ctrl+c` | Quit |

## CI / GitHub Actions

Tab 5 shows recent GitHub Actions workflow runs across all repos:

| Column | Meaning |
|--------|---------|
| Repository | Short repo name |
| Workflow | Workflow file display name |
| Branch | Branch the run was triggered on |
| Status | `✓ success` / `✗ failure` / `● in_progress` / `⊘ cancelled` |
| Event | Trigger event (`push`, `pull_request`, `schedule`, etc.) |
| When | Time since last update |

`o` or `enter` opens the run in GitHub. `r` refreshes. Filtering and sorting (`s`/`S` workflow, `a`/`A` repo, `f`/`F` date) all work as on other tabs.

The **Dashboard** tab (tab 1) also shows a compact CI status icon (`✓` / `✗` / `●` / `—`) in the `CI` column, reflecting the latest run for each repo.

### Dashboard indicators

| Column | Meaning |
|--------|---------|
| `PR` | Open PR count, colour scales blue to yellow to red |
| `Br` | Branch count, colour scales blue to yellow to red |
| `CI` | Latest CI run: `✓` success · `✗` failure · `●` running · `—` no data |
| `●` (tab 3) | Branch has an open PR |
| `∈` (tab 3) | Branch is merged into the default branch |

## Mouse

| Action | Effect |
|--------|--------|
| Scroll wheel | Scroll 3 lines per tick |
| Click tab bar | Switch tab |
| Click row | Select item |
| Double-click row | Open (same as `enter`) |

## License

MIT. See [LICENSE](LICENSE).
