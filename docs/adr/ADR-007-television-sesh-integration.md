# ADR-007: Television and sesh integration

**Status:** Proposed
**Date:** 2026-04-29
**Applies to:** `internal/app/commands.go`, `internal/app/update.go`

## Context

Grove launches external tools (lazygit, diffnav, gh-dash, `$EDITOR`) as one-shot subprocesses via `tea.ExecProcess`. When the tool exits, all context is lost — there is no persistent workspace per repo. The current tool set is also hardcoded per tab/item context with no extension point for additional tools.

Two terminal tools complement grove's orchestration role at different scopes:

**Television** is a fast fuzzy-finder TUI with a cable (plugin) system that can source data from any command and execute actions on selected items. It already integrates with git repos, kubernetes, docker, and other data sources via TOML-defined cables. The user's setup includes a catppuccin theme and shell-level triggers.

**Sesh** is a tmux session manager that aggregates sessions from tmux, zoxide history, and configured directories. It provides persistent, named workspaces that survive tool exits and terminal restarts. The user already has television and sesh wired together — television's `sesh.toml` cable pulls session lists from sesh and connects to selected sessions.

Together, these tools fill two gaps in grove's current workflow:

1. **No persistent workspace**: drilling into a repo via `space` opens a transient tool. When lazygit exits, the working context (open files, terminal history, running processes) is gone. Sesh would give each repo a persistent tmux session that accumulates state across grove visits.

2. **No cross-tool fuzzy search**: grove's built-in filters work within the TUI, but there is no way to fuzzy-search grove's data (repos, PRs, issues, branches) from outside the TUI — for example, from a shell prompt or another tool. A television cable sourcing from grove's cache would make grove's data searchable from anywhere.

### How grove fits between these tools

The three tools operate at distinct scopes:

| Scope | Tool | Role |
| --- | --- | --- |
| Fleet | **grove** | Which repos need attention? Overview across all repos |
| Project | **sesh** | Persistent workspace for a single repo. Editor, terminals, lazygit — all survive across sessions |
| Item | **television** | Find anything fast. Fuzzy search across repos, PRs, branches, sessions, files |

## Decision

Not yet decided. This ADR records the integration opportunities and open questions.

### Integration A — Grove → sesh (persistent repo sessions)

Replace or complement one-shot `launchLazygit` with `sesh connect <repo-name>`:

- `space` on a repo opens (or attaches to) a named tmux session rooted at the repo path.
- The session persists after grove exits. Returning to the same repo reattaches to the existing session with all state intact.
- Session naming could follow the pattern `grove/<profile>/<repo>` for easy identification in sesh listings.
- Fallback: if sesh or tmux is not available, fall back to current one-shot behaviour.

**Open questions:**
- Should this replace lazygit on `space`, or use a different key? Lazygit is valuable as a quick in-and-out tool; sesh is for longer work sessions. Perhaps `space` stays lazygit, and a new key (e.g., `t` for tmux) opens sesh.
- Should grove pre-configure the session layout (editor + terminal panes), or let the user's sesh/tmux config handle that?
- How does this interact with `launchLazygitOnBranch` (branches tab), which checks out a branch before opening and restores it after?

### Integration B — Television cable for grove (fuzzy search from anywhere)

Ship a television cable definition (`grove.toml`) that sources data from grove's cache files:

- **Source**: read from `$XDG_CACHE_HOME/grove/prs.json`, `branches.json`, `issues.json`, etc.
- **Preview**: show item details (PR description, issue body, branch status).
- **Actions**: open in browser (`o`), open in grove at that item, connect to repo via sesh.
- The cable is a TOML file — no grove code changes needed, just a config file shipped alongside grove or documented for users to add.

**Open questions:**
- Grove's cache files are JSON with a specific envelope format. Television cables typically source from command stdout. Should grove provide a `grove list` subcommand that outputs structured data for television, or should the cable parse the JSON directly?
- What actions make sense? Opening in browser is straightforward. "Jump to item in grove" would require grove to accept a startup argument (e.g., `grove --goto pr:42`).

### Integration C — Grove → television (as a picker inside grove)

Use television as the fuzzy picker for selection workflows within grove:

- ADR-003's remote profiles feature needs a way to select from potentially hundreds of org repos to clone. Television could be launched as the picker instead of building a custom fuzzy finder into grove.
- Similarly, branch selection, issue filtering, or label filtering could delegate to television.

**Open questions:**
- Television returns the selected item on stdout. Grove would need to capture this via `exec.Command` output rather than `tea.ExecProcess` (which doesn't capture stdout). Is this feasible within Bubble Tea's lifecycle?
- Does this create a hard dependency on television, or can grove fall back to its own filter system?

## Alternatives Considered

**Build fuzzy search into grove directly** — grove already has text filters and cycle filters. Adding full fuzzy matching (e.g., via the `go-fuzzyfinder` library) would keep everything self-contained but duplicate what television already does well. It also wouldn't help with cross-tool search (integration B).

**Use fzf instead of television** — fzf is more widely installed and has a simpler interface. However, the user's workflow is already built around television (cables, shell triggers, catppuccin theme), and television's cable system provides richer integration than fzf's `--preview` flag.

**No integration — keep tools independent** — each tool works fine alone. But the manual workflow (grove shows a repo needs attention → user manually opens tmux → navigates to repo → opens tools) has friction that sesh removes with a single keystroke.

## Consequences

If adopted:

- Integration A (sesh): grove gains persistent workspaces. `internal/app/commands.go` adds a `launchSesh` function following the existing `launchLazygit` pattern. Config may gain a `session_tool` option for users who prefer other session managers.
- Integration B (television cable): no grove code changes; a `grove.toml` cable file is shipped or documented. May motivate adding a `grove list` subcommand for structured output.
- Integration C (television as picker): grove gains a fuzzy selection UX for ADR-003's remote clone workflow. Requires solving the stdout-capture question within Bubble Tea.
- All integrations are optional — tools degrade gracefully when television or sesh are not installed, matching the existing pattern for lazygit/diffnav/gh-dash.
