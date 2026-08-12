# trunkr

A [herdr](https://herdr.dev/) plugin that integrates
[worktrunk](https://github.com/max-sixty/worktrunk), bringing branch-addressed
git worktree management (`wt new` / `wt switch` / `wt remove`) into
herdr's agent multiplexer — so each coding-agent pane runs in its own
isolated worktree.

## Agent skills

### Common skills

Auto-activate these skills when the task matches:

| Skill          | When to activate                                                                                             |
| -------------- | ------------------------------------------------------------------------------------------------------------ |
| `pull-request` | Creating, opening, or preparing a pull request — including "ship this", "push this up", or "get this merged". |

