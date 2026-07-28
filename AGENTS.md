# trunkr

A [herdr](https://herdr.dev/) plugin that integrates
[worktrunk](https://github.com/max-sixty/worktrunk), bringing branch-addressed
git worktree management (`wt switch` / `list` / `merge` / `remove`) into
herdr's agent multiplexer — so each coding-agent pane runs in its own
isolated worktree.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues (`disintegrator/trunkr`) via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: one `CONTEXT.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
