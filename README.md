# trunkr

[worktrunk](https://github.com/max-sixty/worktrunk) worktrees inside
[herdr](https://herdr.dev/): switch, create, merge, and destroy git worktrees —
and check out PRs into them — without leaving your agent multiplexer.

worktrunk (`wt`) manages git worktrees addressed by branch name, built for
running multiple AI agents in parallel. herdr is the terminal multiplexer those
agents run in. trunkr is the glue: every worktree operation becomes a herdr
action, new worktrees open as panes running your agent, and an interactive
picker shows every worktree with its live agent status.

## Prerequisites

- **herdr** 0.7.5 or newer.
- **worktrunk** — the `wt` binary must be on your `PATH` (or point trunkr at it
  with the `wt_path` config knob). trunkr does not install it for you; actions
  fail with a clear message if it's missing.
- **git** — worktrunk drives it.
- **go** — trunkr is built from source at install time.

## Install

```sh
herdr plugin install disintegrator/trunkr
```

herdr clones the repo, builds the `trunkr` binary with `go build`, and
registers the actions and panes below.

**Updating:** trunkr releases by version bumps on the main branch. To pick up a
new version, re-run the install command.

## Actions

All actions are invoked through herdr's action menu.

| Action | What it does |
| --- | --- |
| **trunkr: picker** | Opens the interactive worktree picker (see below). |
| **trunkr: switch worktree** | Prompts for a branch, then focuses that worktree's existing panes — or, if it has none, opens a new pane in the configured container (default: a new tab). |
| **trunkr: open worktree in tab / workspace / split** | Like switch, but always opens a new pane in the named container. |
| **trunkr: create worktree** | Prompts for a new branch name, runs `wt switch --create`, and opens a pane in the fresh worktree. |
| **trunkr: checkout PR** | Prompts for a PR number or URL, runs `wt switch pr:N`, and opens a pane in the checked-out worktree. |
| **trunkr: merge worktree** | Merges the current worktree's branch into trunk, then tears the worktree down (see below). |
| **trunkr: destroy worktree** | Discards the current worktree — uncommitted changes included — after a confirmation. |
| **trunkr: hello** | Smoke test: verifies trunkr can reach `wt` and call back into herdr. |

New worktree panes run your configured `agent_command` (e.g. `claude`), or a
plain shell when unset. trunkr never re-points existing panes at a different
directory — switching always focuses or opens panes.

Long-running operations (switch, create, PR checkout, merge) stream their `wt`
output in a popup **runner** pane, so worktrunk's lifecycle hooks can prompt
you interactively. The runner closes itself on success; on failure it stays
open with the error and sends a notification. Notifications fire for failures
only — success is the visible effect.

### Merge

Merge is a controlled teardown:

1. `wt merge --no-remove` runs in the runner with hooks enabled (dirty trees
   are auto-committed by worktrunk).
2. On success, the worktree's herdr panes are closed.
3. The worktree is removed with `wt remove`.

If the merge fails, nothing is torn down: the runner holds open with the error
and your worktree and panes are untouched.

### Destroy

Destroy asks for confirmation first — the prompt warns that uncommitted
changes will be discarded and shows how many panes will close. On confirm it
closes the worktree's panes and runs `wt remove -f`. The trunk worktree can
never be merged or destroyed.

## The picker

The **trunkr: picker** action opens an overlay pane listing every worktree in
the current repository:

```
branch · panes · agent status · git state
```

- **Agent status** rolls up the worktree's panes worst-first:
  `blocked` > `working` > `done` > `idle` — the row shows whatever most needs
  your attention.
- **Git state** shows dirty/ahead/behind, straight from `wt list`.
- Rows refresh automatically every couple of seconds.

### Keys

| Key | Action |
| --- | --- |
| `enter` | Switch to the selected worktree (focus its panes, or open one). |
| `t` / `w` / `s` | Open the selected worktree in a new tab / workspace / split. |
| `c` | Create a worktree (prompts for a branch name). |
| `p` | Check out a PR (prompts for a number or URL). |
| `m` | Merge the selected worktree (inline `y/N` confirm). |
| `d` | Destroy the selected worktree (inline `y/N` confirm). |
| `↑`/`↓` or `k`/`j` | Move the cursor. |
| `/` | Filter rows. |
| `r` | Refresh now. |
| `q` / `esc` | Close the picker. |

The seven action keys (`t w s c p m d`) can be remapped in config; the rest
are fixed.

## Configuration

Configuration is TOML, resolved from three layers — later layers win per knob:

1. `trunkr.toml` in trunkr's herdr config directory
   (`$HERDR_PLUGIN_CONFIG_DIR`) — your global defaults.
2. `.trunkr.toml` committed in the repository — team-shared settings.
3. `.trunkr.local.toml` in the repository, untracked — your per-repo
   overrides.

Because a committed `.trunkr.toml` arrives with the repo, it is
**approval-gated**: the first time trunkr sees one (or sees it change), it asks
you to approve it before applying it. Approval is per repository and covers all
of its worktrees. A git-*tracked* `.trunkr.local.toml` is gated the same way,
so a repo can't smuggle ungated config.

All knobs, with their defaults:

```toml
# Command run in new worktree panes. Unset means a plain shell.
agent_command = ["claude"]

# Where the generic switch action opens a new pane when the worktree has
# none: "tab" (default), "workspace", or "split".
container = "tab"

# Absolute path to the wt binary. Unset means PATH lookup.
wt_path = "/usr/local/bin/wt"

[merge]
# Extra arguments appended to every `wt merge` invocation.
extra_args = []

[picker.keys]
# Remap the picker's action keys. Defaults shown.
tab = "t"
workspace = "w"
split = "s"
create = "c"
pr = "p"
merge = "m"
destroy = "d"
```

## How trunkr maps worktrees to panes

There is no state file. trunkr derives the mapping live by matching each herdr
pane's working directory against the worktree paths reported by `wt list`.
Panes you open yourself inside a worktree count just like panes trunkr opened
— they show up in the picker, get focused by switch, and are closed by merge
and destroy.

## License

See [LICENSE](LICENSE).
