# trunkr

Use [Worktrunk](https://worktrunk.dev) for Herdr worktree creation, switching,
and removal.

Worktrunk creates and removes the Git checkout. The plugin opens every selected
checkout with `herdr worktree open`, so Herdr retains native worktree provenance,
grouping, labels, status, and sidebar presentation.

## Requirements

- Herdr 0.8.0 or newer
- Worktrunk's `wt` executable
- Go 1.26.5 when linking a development checkout

## Install

Install trunkr directly from GitHub:

```sh
herdr plugin install disintegrator/trunkr
```

Confirm the plugin and its actions are available:

```sh
herdr plugin list --plugin disintegrator.trunkr
herdr plugin action list --plugin disintegrator.trunkr
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

| Action | Behavior |
| --- | --- |
| Create | `wt switch --create`, then `herdr worktree open` |
| Open | `wt list`, then `herdr worktree open` |
| Remove | `wt remove`, then close the Herdr workspace |

## Development

Install [mise](https://mise.jdx.dev/) if it is not already available:

```sh
curl https://mise.run | sh
```

Install the pinned tools and verify the plugin:

```sh
mise install
mise exec -- go test ./...
mise exec -- go vet ./...
```

Build and link the working tree into Herdr:

```sh
mkdir -p bin
mise exec -- go build -trimpath -o bin/trunkr ./cmd/trunkr
herdr plugin link "$PWD"
```

`herdr plugin link` registers this checkout directly and does not run the
manifest's `[[build]]` commands. Rebuild `bin/trunkr` after code changes.

Invoke actions while developing:

```sh
herdr plugin action invoke disintegrator.trunkr.create
herdr plugin action invoke disintegrator.trunkr.open
herdr plugin action invoke disintegrator.trunkr.remove
```

Inspect action logs or unlink the development checkout:

```sh
herdr plugin log list --plugin disintegrator.trunkr
herdr plugin unlink disintegrator.trunkr
```
