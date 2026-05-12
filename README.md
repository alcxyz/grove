# grove

TUI for multi-repo git forge monitoring across GitHub, Forgejo, and Azure DevOps. See branch status, dirty working trees, ahead/behind counts, open PRs, CI runs, issues, recent branches, and commit activity across all your repos without leaving the terminal.

```
               {o,o}
   ___ _ __ ___|)_)|___ __
  / _ \ '__/  _ \ \ / / _ \
 | (_| | | | (_) \ V /  __/
  \__, |_|  \___/ \_/ \___|
  |___/
  one repo to rule them all
```

## Features

- **Multi-profile**: define any number of profiles (personal, work org, etc.) and switch instantly with `H` / `L` or by clicking the profile tab bar; an **All** view merges every profile at once
- **Clone**: `grove clone` enumerates all repos for each profile's code-hosting remote and clones any that are missing locally; groups can route clones to separate subdirectories
- **Dashboard** (tab 1): all repos in one view with branch, dirty/clean state, sync status, open PR count, branch count, CI status, last author, and last commit time
- **Pull Requests** (tab 2): open PRs across all repos with review status and checks
- **CI Runs** (tab 3): recent workflow / pipeline runs across all repos with pass/fail/running status
- **Branches** (tab 4): all remote branches with PR and merge indicators
- **Activity** (tab 5): recent commits across repos with inline diff viewer
- **Issues** (tab 6): open forge issues across all repos with labels, assignees, milestones, and age
- **Detail pane**: full repo detail with local/remote branches, open PRs, open issues, CI runs, recent commits, and stats; item-level cursor with contextual actions per item type
- **Diff viewer**: scrollable inline `git show` output with syntax colouring; respects your configured diff pager (`delta`, `bat`); navigate between commits with `[` / `]` and between files with `{` / `}`
- **External tools**: `space` opens diffnav (commits) or lazygit (repos) based on context; `e` opens `$EDITOR` / nvim at repo root
- **Grouped / flat view**: toggle between config-defined groups and a flat sorted list
- **Cycle filters**: quickly narrow by author, subject prefix, repository, or date bucket
- **Sort**: ascending/descending by date, author, subject/name, or repository
- **Block-jump navigation**: jump between repo blocks `[ ]`, subject blocks `( )`, or config groups `{ }`
- **Mouse support**: scroll wheel, click to select, tab-bar clicks, double-click to open
- **Profile persistence**: reopens on the same profile you left
- **Screensaver**: bouncing logo after a configurable idle timeout
- **Update notifications**: footer and `grove --version` show when a newer release is available
- Catppuccin Mocha colour palette

## How grove fits in

Grove is a **multi-repo orchestration layer** — it sits above single-repo tools and below the browser, giving you a terminal-native overview across all your repositories.

| Tool | Scope | Strength | Gap grove fills |
| --- | --- | --- | --- |
| **`gh` CLI** | One repo / one entity at a time | Scriptable, full API access | No cross-repo dashboard; no directory-aware clone routing |
| **lazygit** | Single repo, local git | Deep interactive git operations | No forge API (PRs, CI, issues); no multi-repo view |
| **gh-dash** | PRs and issues across repos | Focused PR review workflow | No local git state, branches, activity, or CI overview |
| **grove** | All repos, all signals | Unified dashboard + detail drill-down | — |

Grove doesn't replace these tools — it launches them. `space` opens lazygit or diffnav for deep single-repo work; `o` opens the active forge in the browser; gh-dash handles GitHub-backed PR review. Grove is the flight-control layer that ties them together across repos, showing you where to focus before you drill down.

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

Requires Go 1.22+. Forgejo-backed profiles use direct HTTP API calls with `token_file`. GitHub-backed profiles support `auth_mode: token` (default) with `token_file`, `GH_TOKEN`, or `GITHUB_TOKEN`, and `auth_mode: gh` for environments where an authenticated `gh` CLI is available but PATs are blocked. Azure DevOps-backed profiles use the Azure CLI DevOps extension (`az repos ...`) and require `project` in config.

For GitHub private repos or higher rate limits, use a fine-grained PAT scoped to the specific GitHub-facing repositories. Grove only needs read-only repository permissions for the configured concerns: Metadata, Pull requests, Issues, Actions, Commit statuses, and Contents when GitHub is the code remote for branch/commit metadata.

Use `auth_mode: gh` for GitHub organizations that do not allow PATs. In that mode Grove shells out to `gh api` for PRs, issues, branches, CI, and repo enumeration, and `gh repo clone` for GitHub clones.

For Azure DevOps, authenticate outside grove with `az devops login` or your normal configured Azure CLI credentials. Grove does not read Azure DevOps token files directly.

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
    owner: my-org # code-hosting owner; defaults all concerns unless overridden
    forge: forgejo
    instance_url: https://git.example.com
    token_file: ~/.config/forgejo/token
    base_paths:
      - ~/dev/git/my-org
    prefixes:
      - service-
      - platform-
    groups:
      - name: Services
        match: service- # matches repo names starting with "service-"
      - name: Platform
        match: platform-

  - name: Personal
    owner: my-user
    base_paths:
      - ~/dev/git/personal
      - ~/nix
    prefixes: [] # all git repos in base_paths are scanned
    social:
      owner: my-github-username
      forge: github
    ci:
      owner: my-github-username
      forge: github
    groups:
      - name: nix
        match_path: ~/nix # matches repos under this directory
      - name: pages
        match: github.io # matches repo names containing "github.io"
        code:
          owner: my-github-username
          forge: github
        social:
          owner: my-github-username
          forge: github
        ci:
          owner: my-github-username
          forge: github
    repos:
      - name: annaetattoo # local directory name
        code:
          owner: my-github-username
          repo: annaetattoo.github.io # remote repo name, if different
          forge: github
        social:
          owner: my-github-username
          repo: annaetattoo.github.io
          forge: github
        ci:
          owner: my-github-username
          repo: annaetattoo.github.io
          forge: github

  - name: Azure DevOps
    owner: my-organization
    forge: azuredevops
    instance_url: https://dev.azure.com/my-organization
    project: MyProject
    base_paths:
      - ~/dev/git/azure
    prefixes: []

refresh_secs: 300
screensaver_secs: 300
```

Groups support two matching strategies: `match` matches against the repo name (prefix or substring), and `match_path` matches against the repo's filesystem path (prefix). Both can be used on the same group — either matching puts the repo in that group. First matching group wins; unmatched repos go to "other".

Remote resolution is concern-specific:

- top-level `owner` / `forge` / `instance_url` / `token_file` / `clone_proto` define the default code-hosting remote
- `auth_mode` controls GitHub authentication: `token` (default) or `gh`
- `project` is required for `azuredevops` remotes; Azure DevOps uses Azure CLI authentication (`az devops login` / configured `az`) instead of grove-managed token files
- `social` overrides PR + issue sourcing
- `ci` overrides workflow / pipeline sourcing
- a group may override `code`, `social`, and `ci` for every repo it matches
- a repo entry under `repos:` may override `code`, `social`, and `ci` for one specific repo
- `repo` inside any remote block overrides the remote repository name when it differs from the local directory name
- `ssh_host` overrides the SSH clone host when it differs from the forge web/API host

Resolution order is: profile defaults, then matching group override, then per-repo override.

The legacy single-profile format (`base_path`, `org`, `prefixes` at the top level) is still supported and auto-migrates to a single profile.

You can also pass one or more paths on the command line to override the first profile's `base_paths`:

```sh
grove ~/dev/git/other-org
grove ~/dir1 ~/dir2
```

### Files

| Path                                 | Purpose                                                      |
| ------------------------------------ | ------------------------------------------------------------ |
| `$XDG_CONFIG_HOME/grove/config.yaml` | Config (falls back to `~/.grove.yaml`); profile-based format |
| `$XDG_CACHE_HOME/grove/`             | Cached PR / branch / activity / CI run data + UI state       |
| `$XDG_STATE_HOME/grove/grove.log`    | Runtime log                                                  |

## Key bindings

### Navigation

| Key                 | Action                                                                    |
| ------------------- | ------------------------------------------------------------------------- |
| `j` / `k`           | Move down / up                                                            |
| `h` / `l`           | Previous / next tab                                                       |
| `H` / `L`           | Previous / next profile (cycles through All)                              |
| `gg` / `G`          | First / last item                                                         |
| `tab` / `shift+tab` | Jump 5 / 10 / 20 / 25 lines (accelerates on rapid press)                  |
| `{ }`               | Jump between config groups                                                |
| `[ ]`               | Jump between repo blocks                                                  |
| `( )`               | Jump between CI status blocks (tab 1) / subject / branch / message blocks |
| `1`–`6`              | Switch to tab directly                                                    |

### Filters and sort

| Key       | Action                                                                                    |
| --------- | ----------------------------------------------------------------------------------------- |
| `/`       | Open text filter                                                                          |
| `esc`     | Clear active filter / close pane                                                          |
| `d` / `D` | Cycle by author, sort by author                                                           |
| `s` / `S` | Cycle by subject prefix, sort by name / title                                             |
| `a` / `A` | Cycle by repository, sort by repository                                                   |
| `f` / `F` | Cycle by date, sort by date / updated                                                     |
| `x` / `X` | Cycle / sort by **PR count** (tab 1) / **review status** (tab 2) / **has-PR** (tab 4)     |
| `c` / `C` | Cycle / sort by **branch count** (tab 1) / **merged** (tab 4) / **branch prefix** (tab 3) |
| `v` / `V` | Cycle / sort by **CI status** (tabs 1, 3) / **checks result** (tab 2)                     |

The `x` / `c` / `v` keys follow the spatial layout of the columns they target (PR / Br / CI on the dashboard).

Date buckets: today, yesterday, this week, last week, this month, last month, this quarter, last quarter.

### Actions

| Key                 | Action                                                                                                                       |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `enter`             | Open detail pane (tabs 1-4, 6) / open diff (tab 5)                                                                           |
| `o`                 | Open in browser on the active forge (all tabs and views)                                                                     |
| `space`             | Per-tab tool: gh-dash (PRs), checkout + lazygit (branches), workflow in editor (CI), diffnav (activity), lazygit (dashboard) |
| `e`                 | Open `$EDITOR` / nvim at repo root (all tabs and views)                                                                      |
| `p`                 | `git pull` current repo (all tabs)                                                                                           |
| `r`                 | Refresh current tab                                                                                                          |
| `R`                 | Toggle auto-refresh                                                                                                          |
| `ctrl+f`            | `git fetch` all repos                                                                                                        |
| `g` (single, 400ms) | Toggle grouped / flat view                                                                                                   |
| `,`                 | Show config preview for the active profile; in detail view, show resolved repo config                                        |
| `?`                 | Toggle help overlay                                                                                                          |
| `!`                 | About: version, config path, cache and log locations                                                                         |
| `q` / `ctrl+c`      | Quit                                                                                                                         |

### Detail pane

The detail pane opens with `enter` on any tab and shows full repo info with an item-level cursor:

| Key        | Action                                                                                            |
| ---------- | ------------------------------------------------------------------------------------------------- |
| `j` / `k`  | Select next / previous item                                                                       |
| `{ }`      | Jump between sections (branches, PRs, issues, CI, commits)                                        |
| `[ ]`      | Previous / next repo (follows source tab, skips duplicates)                                       |
| `gg` / `G` | First / last item                                                                                 |
| `,`        | Show resolved config for this repo                                                                 |
| `space`    | Contextual: diffnav (commits), gh-dash (PRs), checkout + lazygit (branches), workflow editor (CI) |
| `o`        | Contextual: open item on the active forge (PR URL, commit, branch, CI run)                         |
| `e`        | Open editor at repo root                                                                          |
| `esc`      | Close detail pane                                                                                 |

### Diff view

The diff view opens with `enter` on the Activity tab:

| Key        | Action                         |
| ---------- | ------------------------------ |
| `j` / `k`  | Scroll up / down               |
| `{ }`      | Jump between files in the diff |
| `[ ]`      | Previous / next commit         |
| `gg` / `G` | Top / bottom                   |
| `space`    | Open in diffnav                |
| `o`        | Open commit in browser         |
| `e`        | Open editor at repo root       |
| `esc`      | Close diff view                |

## CI Runs

Tab 3 shows recent workflow / pipeline runs across all repos:

| Column     | Meaning                                                     |
| ---------- | ----------------------------------------------------------- |
| Repository | Short repo name                                             |
| Workflow   | Workflow file display name                                  |
| Branch     | Branch the run was triggered on                             |
| Status     | `✓ success` / `✗ failure` / `● in_progress` / `⊘ cancelled` |
| Event      | Trigger event (`push`, `pull_request`, `schedule`, etc.)    |
| When       | Time since last update                                      |

`o` opens the run in the browser. `enter` opens the repo's detail pane. `r` refreshes. Filtering and sorting (`s`/`S` workflow, `a`/`A` repo, `f`/`F` date) all work as on other tabs.

The **Dashboard** tab (tab 1) also shows a compact CI status icon (`✓` / `✗` / `●` / `—`) in the `CI` column, reflecting the latest run for each repo.

### Dashboard indicators

| Column           | Meaning                                                              |
| ---------------- | -------------------------------------------------------------------- |
| `PR`             | Open PR count, colour scales blue to yellow to red                   |
| `Br`             | Branch count, colour scales blue to yellow to red                    |
| `Is`             | Open issue count, colour scales blue to yellow to red                |
| `CI`             | Latest CI run: `✓` success / `✗` failure / `●` running / `—` no data |
| `●` (tab 4)      | Branch has an open PR                                                |
| `∈` (tab 4)      | Branch is merged into the default branch                             |
| PR checks (tab 2) | PR status rollup from Actions workflow runs and commit statuses: `✓ pass` / `✗ fail` / `● pending` / `—` none |

## External tools

Grove hands off to external tools via the `space` and `e` keys:

| Context                    | `space` opens                                       | `e` opens        |
| -------------------------- | --------------------------------------------------- | ---------------- |
| Dashboard                  | lazygit                                             | `$EDITOR` / nvim |
| PRs tab                    | gh-dash for GitHub-backed repos                     | `$EDITOR` / nvim |
| Branches tab               | checkout branch + lazygit (restores branch on exit) | `$EDITOR` / nvim |
| CI tab                     | workflow `.yml` in editor                           | `$EDITOR` / nvim |
| Activity tab               | diffnav                                             | `$EDITOR` / nvim |
| Detail pane (commit)       | diffnav                                             | `$EDITOR` / nvim |
| Detail pane (PR)           | gh-dash for GitHub-backed repos                     | `$EDITOR` / nvim |
| Detail pane (branch)       | checkout + lazygit                                  | `$EDITOR` / nvim |
| Detail pane (CI run)       | workflow `.yml` in editor                           | `$EDITOR` / nvim |
| Detail pane (local branch) | lazygit                                             | `$EDITOR` / nvim |
| Diff view                  | diffnav                                             | `$EDITOR` / nvim |

If a tool is not found on `PATH` or there is no valid target (no forge URL, no repo selected), a status message is shown instead of failing silently.

## Mouse

| Action            | Effect                                                       |
| ----------------- | ------------------------------------------------------------ |
| Scroll wheel      | Scroll 3 lines per tick (works in detail and diff views too) |
| Click tab bar     | Switch tab                                                   |
| Click profile bar | Switch profile                                               |
| Click row         | Select item                                                  |
| Double-click row  | Open (same as `enter`)                                       |

## Caching

Grove caches forge API responses to disk so the UI opens instantly and remains usable while refreshes happen in the background.

**What is cached**

| File            | Content                   | Cap         |
| --------------- | ------------------------- | ----------- |
| `prs.json`      | Open pull requests        | 500 items   |
| `branches.json` | Remote branches           | 2 000 items |
| `activity.json` | Recent commits            | 100 items   |
| `runs.json`     | CI workflow runs          | 500 items   |
| `issues.json`   | Open issues               | 500 items   |
| `state.json`    | UI state (active profile) | —           |

Each data file is a JSON object `{ "cached_at": <RFC3339>, "config_key": <string>, "data": [...] }`.

**Startup** — all five cache files are read before the TUI launches. The UI renders immediately with the cached data; fresh data loads in the background and replaces it without any visual flicker. The last active profile is restored from `state.json`.

**TTL** — controlled by `refresh_secs` in config (default 300 s). On startup and on every tab switch, grove checks whether the data for that tab is older than the TTL. If so, a background fetch is triggered automatically. Auto-refresh (toggled with `R`) repeats this on a timer.

**Cache invalidation** — each file stores a `config_key` derived from the resolved remote config and prefixes of all configured profiles. If the config changes (new profile, different owner, changed prefixes, or different concern-specific remotes), the key changes and all cached data is treated as a miss, forcing a full refresh on next launch.

**Forcing a refresh** — press `r` to refresh the active tab immediately, or `ctrl+f` to `git fetch` all repos.

## Rate limiting

GitHub and Forgejo calls use direct HTTP requests with token auth. GitHub-backed API calls are limited to **5 concurrent requests** at a time to stay inside rate limits.

`grove clone` uses a separate semaphore capped at **8 concurrent clones**, since `git clone` is network-bound rather than API-bound.

## Clone

`grove clone` enumerates all repos for each profile's code-hosting remote and clones any that are not already present locally:

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
    base_path: ~/dev/git/my-org/services # cloned here
  - name: Platform
    match: platform-
    # no base_path → falls back to profile base_paths[0]
```

Existing repos (detected by the presence of a `.git` directory) are skipped. Up to 8 clones run in parallel.

## Releasing

Releases are automated. To publish a new version:

1. Bump the `VERSION` file
2. Merge `dev -> main` on the repository's integration authority
3. Create or push the release tag from the authority or a trusted local clone
4. Push the trusted history and tags to GitHub if GitHub Releases/Homebrew/AUR are used

CI runs tests and goreleaser builds binaries for GitHub Releases, Homebrew, and AUR.

Grove supports split-host workflows: GitHub can be used for reach, issue intake, PR suggestions, and distribution without being trusted as the integration authority. Do not merge the same `dev -> main` change on multiple hosts. See [ADR-011](docs/adr/ADR-011-split-host-integration-authority.md).

## License

MIT. See [LICENSE](LICENSE).

<details>
<summary>Support</summary>

- **BTC:** `bc1pzdt3rjhnme90ev577n0cnxvlwvclf4ys84t2kfeu9rd3rqpaaafsgmxrfa`
- **ETH / ERC-20:** `0x2122c7817381B74762318b506c19600fF8B8372c`
</details>
