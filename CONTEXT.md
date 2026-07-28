# trunkr — Context

trunkr is a herdr plugin that exposes worktrunk's git-worktree workflow inside
herdr, so each coding agent runs in an isolated worktree with worktree
operations (create, switch, merge, clean up) available as herdr actions and
panes.

## Glossary

- **worktrunk (`wt`)** — CLI tool ([max-sixty/worktrunk](https://github.com/max-sixty/worktrunk))
  that manages git worktrees addressed by branch name, with paths computed from
  templates. Built for running multiple AI agents in parallel, each in its own
  worktree. Core commands: `wt switch`, `wt list`, `wt merge`, `wt remove`.
  Supports lifecycle **hooks** (create, pre-merge, post-merge), LLM-generated
  commit messages, build-cache sharing across worktrees, an interactive picker,
  and PR checkout (`wt switch pr:123`).
- **herdr** — terminal-based agent multiplexer ([herdr.dev](https://herdr.dev/)):
  persistent server sessions with real terminal **panes**, tabs, and
  **workspaces**; agent state visibility (blocked / working / done / idle);
  controllable via its CLI and a JSON socket API.
- **herdr plugin** — a directory with a `herdr-plugin.toml` manifest plus
  executable commands, in any language. Manifest sections: `[[build]]`,
  `[[startup]]`, `[[actions]]` (user-invokable workflows), `[[events]]` (hooks
  on herdr events), `[[panes]]` (terminal UI entrypoints), `[[link_handlers]]`.
  The entire herdr CLI is the plugin API, reached via `HERDR_BIN_PATH`; runtime
  env vars include `HERDR_SOCKET_PATH`, `HERDR_PLUGIN_ID`,
  `HERDR_PLUGIN_CONFIG_DIR`, `HERDR_PLUGIN_STATE_DIR`, and workspace/tab/pane
  IDs.
- **Distribution** — herdr plugins are auto-discovered from public GitHub repos
  tagged with the `herdr-plugin` topic (index refreshes every 30 minutes);
  installed with `herdr plugin install disintegrator/trunkr`.

## trunkr's role

The glue layer between the two: drive `wt` from herdr so that, for example, a
new agent pane can be spawned inside a fresh worktree, worktrees can be browsed
from a picker pane, and an agent's finished work can be merged and cleaned up
via herdr actions.
