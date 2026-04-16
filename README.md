# grove

A terminal UI for monitoring GitHub repositories. See branch status, dirty working trees, ahead/behind counts, open PRs, recent branches, and commit activity across all your repos without leaving the terminal.

```
   __ _ _ __ _____   _____
  / _` | '__/ _ \ \ / / _ \
 | (_| | | | (_) \ V /  __/
  \__, |_|  \___/ \_/ \___|
     |_|
```

## Features

- **Dashboard**: all repos in one view with branch, dirty/clean state, sync status, open PR count, branch count, last author, and last commit time
- **Pull Requests**: open PRs across all repos with review status
- **Branches**: all remote branches with PR and merge indicators
- **Activity**: recent commits across repos with inline diff viewer
- **Grouped / flat view**: toggle between config-defined groups and a flat sorted list
- **Cycle filters**: quickly narrow by author, subject prefix, repository, or date bucket
- **Sort**: ascending/descending by date, author, subject/name, or repository
- **Block-jump navigation**: jump between repo blocks `[ ]`, subject blocks `( )`, or config groups `{ }`
- **Detail pane**: full repo detail with local branches, open PRs, recent commits, and stats
- **Diff viewer**: inline `git show` output with syntax colouring
- **Mouse support**: scroll wheel, click to select, tab-bar clicks, double-click to open
- **Screensaver**: bouncing logo after a configurable idle timeout
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
# Path to the directory that contains your local repo checkouts
base_path: ~/dev/git/my-org

# GitHub organisation (used by gh CLI for PRs and branches)
org: my-org

# Only repos whose names start with one of these prefixes are scanned
prefixes:
  - service-
  - platform-

# Auto-refresh interval in seconds (default: 300)
refresh_secs: 300

# Seconds of inactivity before the screensaver activates (0 = disabled, default: 300)
screensaver_secs: 300

# Named groups control how repos are grouped and ordered across all tabs.
# First matching group wins; unmatched repos fall into an implicit "other" group.
groups:
  - name: Infrastructure
    match: infra-
  - name: Services
    match: service-
  - name: Platform
    match: platform-
```

You can also pass a path directly to override `base_path`:

```sh
grove ~/dev/git/other-org
```

### Files

| Path | Purpose |
|------|---------|
| `$XDG_CONFIG_HOME/grove/config.yaml` | Config (falls back to `~/.grove.yaml`) |
| `$XDG_CACHE_HOME/grove/` | Cached PR / branch / activity data |
| `$XDG_STATE_HOME/grove/grove.log` | Runtime log |

## Key bindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `gg` | First item |
| `G` | Last item |
| `{ }` | Jump between config groups |
| `[ ]` | Jump between repo blocks |
| `( )` | Jump between subject / branch / message blocks |
| `tab` / `shift+tab` | Next / previous tab |
| `1` `2` `3` `4` | Switch to tab directly |

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
| `enter` | Open detail pane (tab 1), open diff (tab 4) |
| `o` | Open PR in browser (tabs 2 and 4) |
| `p` | `git pull` current repo (tab 1) |
| `r` | Refresh current tab |
| `R` | Toggle auto-refresh |
| `ctrl+f` | `git fetch` all repos |
| `g` (single, 400ms) | Toggle grouped / flat view |
| `?` | Toggle help overlay |
| `!` | About: version, config path, cache and log locations |
| `q` / `ctrl+c` | Quit |

### Dashboard indicators

| Column | Meaning |
|--------|---------|
| `PR` | Open PR count, colour scales blue to yellow to red |
| `Br` | Branch count, colour scales blue to yellow to red |
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
