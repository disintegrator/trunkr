# Changelog

## trunkr 0.1.0

### Minor Changes

- eea6776: Initial herdr plugin skeleton: the plugin links into herdr, a hello action verifies worktrunk (wt) is on PATH and calls back into herdr, and a Bubble Tea smoke-test pane proves the picker's TUI stack runs in an overlay pane.
- fa52f93: Add switch, create, and PR checkout actions: each runs wt in an interactive popup runner (so project hooks can prompt), then opens or focuses the worktree's panes in a tab, workspace, or split, launching your configured agent command in new panes.
- 38eb4cf: Add the interactive worktree picker: a keybindable overlay listing every worktree with its live pane count, rolled-up agent status (worst-first: blocked > working > done > idle), and git state. Enter switches (focusing existing panes), t/w/s open in a new tab/workspace/split, c creates a branch, p checks out a PR, / filters, and keys are remappable via [picker.keys].
- 38eb4cf: Add a merge flow: merge a worktree's branch into trunk from the picker (`m` with a y/N confirm) or the "trunkr: merge worktree" action. The merge streams `wt merge` output in the runner so hook approval prompts work, then closes the worktree's panes and removes the worktree; on failure nothing is torn down and the runner holds open with the error. Extra `wt merge` flags can be set via `[merge] extra_args`.
- 38eb4cf: Add a destroy flow: tear down a worktree from the picker (`d` with a y/N confirm) or the "trunkr: destroy worktree" action. The confirm warns that uncommitted changes are discarded and panes close; trunkr then closes the worktree's panes and runs `wt remove -f` silently. Branch deletion stays on wt's merged-only default, so an unmerged branch always survives as the recovery net. Failures fire a notification.
