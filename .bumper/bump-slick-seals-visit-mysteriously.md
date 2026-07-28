---
trunkr: minor
---

Add a destroy flow: tear down a worktree from the picker (`d` with a y/N confirm) or the "trunkr: destroy worktree" action. The confirm warns that uncommitted changes are discarded and panes close; trunkr then closes the worktree's panes and runs `wt remove -f` silently. Branch deletion stays on wt's merged-only default, so an unmerged branch always survives as the recovery net. Failures fire a notification.
