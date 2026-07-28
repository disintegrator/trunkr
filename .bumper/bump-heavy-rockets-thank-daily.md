---
trunkr: minor
---

Add a merge flow: merge a worktree's branch into trunk from the picker (`m` with a y/N confirm) or the "trunkr: merge worktree" action. The merge streams `wt merge` output in the runner so hook approval prompts work, then closes the worktree's panes and removes the worktree; on failure nothing is torn down and the runner holds open with the error. Extra `wt merge` flags can be set via `[merge] extra_args`.
