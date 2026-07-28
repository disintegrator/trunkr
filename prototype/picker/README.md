# Prototype: trunkr picker pane

Throwaway Bubble Tea stub for [#5](https://github.com/disintegrator/trunkr/issues/5).
All data is fake; every action prints the `wt`/`herdr` command it *would* run.

```sh
cd prototype/picker
go run .            # interactive (needs a TTY)
go run . -snapshot  # static render of the list view
```

## What it proposes

**Rows** — branch (trunk marked, shown dimmed), live-pane count, rolled-up agent
status (worst-first: blocked > working > done > idle), and git state from
`wt list` (+ahead −behind, `~dirty`, or `clean`). Paths are omitted from rows —
worktrunk paths are template-noise; the branch is the identity.

**Keybindings**

| Key | Action |
| --- | --- |
| `enter` | Generic switch — focus existing panes, else open in configured default container |
| `t` / `w` / `s` | Explicitly open in a new tab / workspace / split (always a new pane) |
| `c` | Create worktree — prompts for branch, runs `wt switch -c` |
| `p` | PR checkout — prompts for number/URL, runs `wt switch pr:N` |
| `m` | Merge — inline y/N confirm, warns when it will commit a dirty tree |
| `d` | Destroy — inline y/N confirm, warns how many live panes it closes first |
| `/` | Filter by branch substring |
| `r` | Refresh (`wt list` + `herdr pane list`) |
| `q` / `esc` | Quit (closes the overlay) |

**Trunk-native, not `wt`'s picker** — `wt`'s built-in picker can't show herdr
pane counts or agent status, which are the point of this pane. Library:
Bubble Tea + Lipgloss (this stub is the proof).

**Placement** — intended as a `[[panes]]` entrypoint with the default
`overlay` placement; confirms happen inline in the pane (herdr has no
confirmation API).
