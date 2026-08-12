# trunkr

Use [Worktrunk](https://worktrunk.dev) for Herdr worktree operations only when
the focused directory, or one of its parents, contains `.config/wt.toml`.
Repositories without that file continue to use Herdr's native worktree
commands.

Worktrunk creates and removes the Git checkout. The plugin opens every selected
checkout with `herdr worktree open`, so Herdr retains native worktree provenance,
grouping, labels, status, and sidebar presentation.

## Requirements

- Herdr 0.8.0 or newer
- Worktrunk's `wt` executable for opted-in repositories
- Go 1.25 or newer when linking a development checkout

## Install for development

```sh
mkdir -p bin
go build -o bin/trunkr ./cmd/trunkr
herdr plugin link "$PWD"
```

## Keybindings

Herdr plugin v1 cannot intercept built-in actions or sidebar context-menu
items. Disable the native key and bind the plugin action to make a keyboard
shortcut a drop-in replacement:

```toml
[keys]
new_worktree = ""
open_worktree = ""
remove_worktree = ""

[[keys.command]]
key = "prefix+shift+g"
type = "plugin_action"
command = "disintegrator.trunkr.create"
description = "create worktree"

[[keys.command]]
key = "prefix+shift+o"
type = "plugin_action"
command = "disintegrator.trunkr.open"
description = "open worktree"

[[keys.command]]
key = "prefix+d"
type = "plugin_action"
command = "disintegrator.trunkr.remove"
description = "remove worktree"
```

Run `herdr server reload-config` after changing the configuration.

### Remote Herdr sessions

`herdr --remote` uses local keybindings by default, and Herdr does not send
local plugin-action bindings to the remote server. Use the remote server's
keybindings when attaching so the actions above are available:

```sh
herdr --remote <ssh-target> --remote-keybindings server
```

Keybindings are captured when the client attaches. Detach and reattach after
changing this option or the remote keybinding configuration.

Press `Escape` or `Ctrl+C` at any trunkr prompt to cancel and dismiss its
popup.

The open action uses a filterable `huh` switcher. Navigate with arrow keys or
`j`/`k`, press `/` to filter by branch or path, and press Enter to open the
selected worktree with Herdr's native sidebar grouping.

## Behavior

| Action | `.config/wt.toml` found | Not found |
| --- | --- | --- |
| Create | `wt switch --create`, then `herdr worktree open` | `herdr worktree create` |
| Open | `wt list`, then `herdr worktree open` | `herdr worktree list/open` |
| Remove | `wt remove`, then close the Herdr workspace | `herdr worktree remove` |
