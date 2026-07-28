# herdr plugin API — research findings (herdr 0.7.5)

Researched 2026-07-28 against primary sources only:

- **S1 — local binary**: herdr 0.7.5 at `/home/disintegrator/.local/share/mise/installs/github-ogulcancelik-herdr/0.7.5/herdr` (`herdr --version` → `herdr 0.7.5`). Help output, `herdr api schema --json` (the bundled JSON socket schema, protocol 17), read-only `list`/`status` invocations against the live server, and `strings` on the binary.
- **S2 — official docs**: https://herdr.dev/docs/ — notably https://herdr.dev/docs/plugins/, https://herdr.dev/docs/socket-api/, https://herdr.dev/docs/marketplace/, https://herdr.dev/docs/cli-reference/.
- **S3 — source repo**: github.com/ogulcancelik/herdr (public, Rust). Where behavior is version-sensitive, files were read at the `v0.7.5` tag, e.g. `docs/versions/0.7.5/website/src/content/docs/plugins.mdx` (the exact docs for the installed version), `src/api/schema/events.rs`, `src/app/api/plugins/manifest.rs`, `src/app/api/plugins/runtime.rs`.
- **S4 — published plugins** (GitHub topic `herdr-plugin`, via `gh search repos --topic herdr-plugin`): manifests of `ogulcancelik/herdr-browser`, `ogulcancelik/herdr-plugin-github-start`, `smarzban/herdr-file-viewer`, `persiyanov/herdr-reviewr`, `NathanFlurry/herdr-plugin-jj-workspace`.

Claims below are cited as [S1]…[S4]. Anything not backed by these sources is marked **not documented / not observed**.

---

## 1. `herdr-plugin.toml` manifest schema

A plugin is "a directory with a `herdr-plugin.toml` manifest and commands Herdr can launch" — there is no SDK; "the entire Herdr CLI is the plugin API" [S3 `plugins.mdx` v0.7.5]. The manifest file name `herdr-plugin.toml` is hard-coded [S1 binary strings; S3 `manifest.rs`].

The authoritative field set comes from three agreeing sources: the serde structs in `src/app/api/plugins/manifest.rs` at tag `v0.7.5` [S3], the `PluginManifest*` definitions in the bundled API schema (`herdr api schema --json`, `schemas.success_response.$defs`) [S1], and the v0.7.5 plugins docs [S3 docs].

### Top level

| Field | Type | Required | Semantics |
|---|---|---|---|
| `id` | string | **required** | Plugin id. "ASCII letters, digits, dot, colon, underscore, and hyphen" [S3 docs]. Validation error `invalid_plugin_id` [S1 strings]. |
| `name` | string | **required** | Display name ("plugin name is required" → `invalid_plugin_name`) [S1 strings; S3 `manifest.rs`]. |
| `version` | string | **required** | Plugin version (`invalid_plugin_version` if empty) [S1; S3]. |
| `min_herdr_version` | string | **required** | Optional in the TOML deserializer but rejected when absent: "plugin min_herdr_version is required" / `plugin_requires_newer_herdr` [S1 strings; S3 `manifest.rs`]. "Herdr refuses to link or install a plugin when its minimum version is newer than the current binary" [S3 docs]. |
| `description` | string | optional | [S3 docs; S1 schema]. |
| `platforms` | array of `"linux"` \| `"macos"` \| `"windows"` | optional | Where the plugin can run. "Local plugins without top-level `platforms` link with a warning" ("manifest does not declare platforms; platform support unknown") [S3 docs; S1 strings]. An **empty** array is an error: "platforms must not be an empty array; omit the field to leave platforms undeclared" [S1 strings]. |

Item-level `platforms` (available on every section below) override the top-level list [S3 docs].

All `command` values are **argv arrays** (array of strings; must be non-empty, with non-empty strings — `invalid_plugin_command`) [S1 strings; S3 `manifest.rs`]. "Herdr does not run them through a shell, so there is no shell expansion unless your command starts a shell itself" [S3 docs v0.7.5].

### `[[build]]`

| Field | Type | Required |
|---|---|---|
| `command` | array of string | **required** |
| `platforms` | array | optional |

Semantics [S3 docs v0.7.5]: run during GitHub `plugin install` after user confirmation and before registration; a failing build aborts the install. `plugin link` does **not** run build commands. Build commands may generate files, "but changing `herdr-plugin.toml` after the install preview aborts install" (also in binary: "plugin build changed herdr-plugin.toml after install preview; aborting install" [S1 strings]). Build commands "do not receive runtime plugin context or Herdr socket env" [S3 docs]. Real-world use: `cargo build --release`, `npm ci`, fetch-prebuilt-binary scripts [S4].

### `[[startup]]`

| Field | Type | Required |
|---|---|---|
| `command` | array of string | **required** |
| `platforms` | array | optional |

Semantics [S3 docs v0.7.5]: run once per enabled plugin "after Herdr restores the session and its API socket is ready"; run again on live-handoff server takeover, but not on client attach, config reload, or link/enable. Started asynchronously; logged in the plugin command log; failure does not stop the server. "One-shot initialization commands, not supervised daemons." They receive the normal runtime env plus `HERDR_PLUGIN_EVENT=startup`.

### `[[actions]]`

| Field | Type | Required | Semantics |
|---|---|---|---|
| `id` | string | **required** | Local id; "ASCII letters, digits, colon, underscore, and hyphen, but not dots"; unique per plugin (`duplicate_plugin_action_id`) [S3 docs; S1 strings]. Globally qualified as `plugin.id.action` (e.g. `example.layout.apply`) [S3 docs]. |
| `title` | string | **required** | `invalid_plugin_action_title` if empty [S1 strings]. |
| `description` | string | optional | [S3 `manifest.rs`; S1 schema]. |
| `contexts` | array of `"global"` \| `"workspace"` \| `"tab"` \| `"pane"` \| `"selection"` | optional (defaults to empty list) | Enum from `PluginActionContext` [S1 schema]. Where the action is offered. The precise UI meaning of each context value is **not documented** in the fetched pages. |
| `command` | array of string | **required** | argv. |
| `platforms` | array | optional | |

Invoked via keybinding (`[[keys.command]]` with `type = "plugin_action"`, `command = "example.layout.apply"` [S3 docs]), via `herdr plugin action invoke <ACTION_ID> [--plugin <ID>]` [S1 help], via socket `plugin.action.invoke` `{action_id, plugin_id?, context?}` [S1 schema], or via a link handler.

### `[[events]]`

| Field | Type | Required |
|---|---|---|
| `on` | string | **required** (`invalid_plugin_event` "event name is required") [S1 strings] |
| `command` | array of string | **required** |
| `platforms` | array | optional |

An **unknown** event name is a non-fatal warning, not an error: link-time validation collects `unknown event '<name>'` into the plugin's `warnings` list surfaced by `plugin.list` [S3 `manifest.rs` v0.7.5 `validate_event_names`; S1 schema `InstalledPluginInfo.warnings`]. See section 4 for the full event list and hook payloads.

### `[[panes]]`

| Field | Type | Required | Semantics |
|---|---|---|---|
| `id` | string | **required** | Local id, no dots; unique per plugin (`duplicate_plugin_pane_id`) [S3 docs; S1 strings]. |
| `title` | string | **required** | "pane title is required" [S1 strings]. |
| `description` | string | optional | [S3 `manifest.rs`]. |
| `placement` | `"overlay"` \| `"popup"` \| `"split"` \| `"tab"` \| `"zoomed"` | optional, **default `"overlay"`** | [S1 schema `PluginManifestPane.placement` default; S3 docs]. |
| `width`, `height` | integer (outer terminal cells) or string percentage `"1%"`–`"100%"` | optional | **Only valid when `placement = "popup"`**: "pane width and height are only supported when placement is popup" [S1 strings; S1 schema `PopupSize`]. Omit for the default half-size popup [S3 docs]. |
| `command` | array of string | **required** | argv; the process runs inside the opened terminal pane. |
| `platforms` | array | optional | |

Placement semantics [S3 docs v0.7.5]: `overlay` = temporary zoomed overlay over the active pane, restores focus/zoom on close. `popup` = session-modal terminal popup, **not a Herdr pane** — "no pane ID, does not change plugin focus context, emits no pane lifecycle events, and does not participate in pane, layout, persistence, or agent APIs. Its process does not receive `HERDR_PANE_ID`." Closes when its command exits or on a `popup.close` request; opening returns `ui_busy` while another modal is active. `split`/`tab`/`zoomed`/`overlay` panes are normal Herdr panes once open and can be driven by the standard pane APIs.

### `[[link_handlers]]`

| Field | Type | Required | Semantics |
|---|---|---|---|
| `id` | string | **required** | Local id, no dots [S3 docs; S1 strings `invalid_plugin_link_handler_id`]. |
| `title` | string | **required** | [S1 strings]. |
| `pattern` | string | **required** | "a Rust regular expression matched against the clicked URL" [S3 docs; S1 `invalid_plugin_link_handler_pattern`]. |
| `action` | string | **required** | "must name an action declared by the same plugin" [S3 docs; S1 `invalid_plugin_link_handler_action`]. |
| `platforms` | array | optional | |

Routes **Ctrl+click** (Control on every platform, including macOS) on matching terminal URLs to the named action instead of the browser. "Handlers are checked in manifest order inside each plugin" [S3 docs v0.7.5].

### Real-world example (verbatim, abridged) [S4 `persiyanov/herdr-reviewr`]

```toml
id = "persiyanov.reviewr"
name = "reviewr"
version = "0.26.1"
min_herdr_version = "0.7.5"
platforms = ["macos", "linux"]
description = "Review agent-written diffs beside the chat and add line comments to the agent input."

[[build]]
command = ["bash", "herdr/install.sh"]

[[panes]]
id = "sidebar"
title = "reviewr"
placement = "split"
command = ["sh", "-c", "exec \"$HERDR_PLUGIN_ROOT/bin/herdr-reviewr\""]

[[actions]]
id = "toggle"
title = "reviewr: toggle sidebar"
contexts = ["pane", "workspace"]
command = ["bash", "herdr/sidebar.sh", "toggle"]

[[events]]
on = "worktree.created"
command = ["bash", "herdr/sidebar.sh", "auto-open"]
```

---

## 2. Runtime contract: env vars and command invocation

### Invocation

- Runtime commands (startup, action, event, pane) are spawned as argv (no shell) **with the plugin root as the working directory** [S3 docs v0.7.5 "Runtime commands run with the plugin directory as their working directory"; S3 `runtime.rs` `command_for_argv_in_dir(&program, &args, &plugin_root)`].
- stdout/stderr are captured, capped at 64 KiB each ("[herdr truncated plugin output after 65536 bytes]"), and stored in the plugin command log (last 200 entries; `herdr plugin log list`) [S3 `runtime.rs` `PLUGIN_COMMAND_OUTPUT_MAX_BYTES`, `PLUGIN_COMMAND_LOG_LIMIT`].
- At most **32 plugin commands in flight**; beyond that invocation fails with `plugin_command_limit_reached` [S3 `runtime.rs` `MAX_PLUGIN_COMMANDS_IN_FLIGHT`].
- Nothing is passed on stdin or as extra argv; all context arrives via environment variables [S3 `runtime.rs` — the spawn wires only env, stdout, stderr].
- On Windows, build/action/event commands resolve `PATHEXT` shims (`npm.cmd`, `bun.cmd`, `pnpm.cmd`); pane commands use the normal Windows pane launcher [S3 docs v0.7.5]. (A published plugin documents that **relative pane commands fail to spawn on Windows**, resolved against herdr's own directory — verified by that author on herdr 0.7.1 [S4 `herdr-file-viewer` manifest comments]; not confirmed elsewhere.)

### Environment variables (all runtime plugin commands) [S3 docs v0.7.5 + `runtime.rs`, corroborated by S1 binary strings]

| Var | Value |
|---|---|
| `HERDR_SOCKET_PATH` | Path to the API socket (Unix socket on Unix, named pipe on Windows). |
| `HERDR_BIN_PATH` | Absolute path of the running herdr binary (from `current_exe()`); the recommended, transport-portable way to call back into herdr. |
| `HERDR_ENV` | `"1"`. |
| `HERDR_PLUGIN_ID` | The plugin's manifest id. |
| `HERDR_PLUGIN_ROOT` | Installed/linked plugin directory (managed checkout for GitHub installs — do not store state there). |
| `HERDR_PLUGIN_CONFIG_DIR` | Per-plugin user-editable config directory (created by herdr). |
| `HERDR_PLUGIN_STATE_DIR` | Per-plugin runtime-state directory (created by herdr). |
| `HERDR_PLUGIN_CONTEXT_JSON` | JSON-serialized `PluginInvocationContext` (see below). |
| `HERDR_WORKSPACE_ID`, `HERDR_TAB_ID`, `HERDR_PANE_ID` | Set when available for the invocation (pane id = focused pane). Popup pane processes never get `HERDR_PANE_ID`. |

Plus, per entrypoint kind:

| Var | When | Value |
|---|---|---|
| `HERDR_PLUGIN_ACTION_ID` | actions (incl. link-handler-invoked) | the action id |
| `HERDR_PLUGIN_EVENT` | startup + event hooks | `"startup"` or the event dot-name (e.g. `pane.agent_status_changed`) |
| `HERDR_PLUGIN_EVENT_JSON` | event hooks only | full JSON event envelope (see section 4) |
| `HERDR_PLUGIN_ENTRYPOINT_ID` | pane commands | the `[[panes]]` id |
| `HERDR_PLUGIN_CLICKED_URL` | link-handler invocations | the clicked URL |
| `HERDR_PLUGIN_LINK_HANDLER_ID` | link-handler invocations | the handler id |

`HERDR_PLUGIN_CONTEXT_JSON` shape (`PluginInvocationContext`, all fields nullable) [S1 schema]: `workspace_id`, `workspace_label`, `workspace_cwd`, `worktree` (WorkspaceWorktreeInfo), `tab_id`, `tab_label`, `focused_pane_id`, `focused_pane_agent`, `focused_pane_status` (`idle|working|blocked|done|unknown`), `focused_pane_cwd`, `selected_text`, `clicked_url`, `link_handler_id`, `invocation_source` (e.g. `"startup"`, `"link_click"`), `correlation_id`.

Other `HERDR_*` vars seen in the binary (`HERDR_HOOK_INPUT_FILE`, `HERDR_INTEGRATION_ID`, `HERDR_ACTIVE_PANE_CWD`, …) belong to the built-in agent *integrations* (Claude Code hooks etc.), not the plugin surface [S1 strings; **plugin relevance not documented**].

---

## 3. CLI / JSON socket API

### Transport and framing [S2 socket-api docs]

- "Herdr uses newline-delimited JSON over a local socket. On Unix, that socket is a Unix domain socket. On Windows, it is a named pipe."
- Default socket `~/.config/herdr/herdr.sock`; named sessions use `~/.config/herdr/sessions/<name>/herdr.sock`. Resolution order: CLI `--session`, `HERDR_SOCKET_PATH`, `HERDR_SESSION`, default. (Live server confirmed at `/home/disintegrator/.config/herdr/herdr.sock` via `herdr status --json` [S1].)
- Request: `{"id":"req_1","method":"ping","params":{}}` → success `{"id":"req_1","result":{"type":"pong"}}` / error `{"id":"req_1","error":{"code":"not_found","message":"pane not found"}}` [S2].
- The full machine-readable contract ships in the binary: `herdr api schema --json` (protocol 17, schema_version, `schemas.{request,success_response,error_response,event,subscription_event}`) [S1]. `herdr api snapshot` prints the live session snapshot [S1 help].
- Docs recommendation: "most automation should start with the CLI wrappers" [S2]. Every CLI helper emits the raw JSON response envelope with an id like `cli:pane:list` [S1 observed].

### Creating panes/tabs/workspaces with cwd + command

There is **no single "create pane with command" parameter** — you split (optionally with `cwd`/`env`), then `pane run` or `pane send-text`/`send-keys` into it; docs example: `herdr pane run w1:p2 "npm test"` [S2].

- `herdr pane split [PANE_ID] [--pane <ID>|--current] [--direction right|down] [--ratio <FLOAT>] [--cwd <PATH>] [--env KEY=VALUE] [--focus|--no-focus]` — socket `pane.split`, params `{direction (required), target_pane_id?, workspace_id?, ratio?, cwd?, env{}, focus=false}` [S1 help + schema].
- `herdr pane run <PANE_ID> <COMMAND>...` — run a command in an existing pane [S1 help].
- `herdr tab create [--workspace <ID>] [--cwd <PATH>] [--label <TEXT>] [--env KEY=VALUE] [--focus|--no-focus]` — socket `tab.create` `{workspace_id?, cwd?, label?, env{}, focus=false}` [S1 help + schema].
- `herdr workspace create [--cwd <PATH>] [--label <TEXT>] [--env KEY=VALUE] [--focus|--no-focus]` — socket `workspace.create` [S1 help + schema].
- Git-worktree-backed workspaces: `herdr worktree create --branch <NAME> [--base <REF>] [--path <PATH>] [--cwd <PATH>] [--label <TEXT>] [--json]`, plus `worktree open|list|remove` [S1 help].
- Plugin-owned panes: `herdr plugin pane open --plugin <ID> --entrypoint <ID> [--placement overlay|split|tab|zoomed] [--workspace <ID>] [--target-pane <PANE>] [--direction right|down] [--cwd <PATH>] [--env KEY=VALUE] [--focus|--no-focus]`; `herdr plugin pane focus|close <PANE_ID>` [S1 help]. Note: the 0.7.5 CLI `--placement` list omits `popup` and has no width/height flags, while the socket `plugin.pane.open` params do accept `"popup"` plus `width`/`height` (`PopupSize`) [S1 help vs S1 schema] — use the socket (or manifest placement) for popups.

### Listing panes — **pane cwd IS exposed**

`herdr pane list [--workspace <ID>]` → `{"id":"cli:pane:list","result":{"type":"pane_list","panes":[PaneInfo...]}}`.

`PaneInfo` fields [S1 schema; confirmed in live output]: `pane_id`, `terminal_id`, `workspace_id`, `tab_id`, `focused`, `agent_status` (**required**: `idle|working|blocked|done|unknown`), `revision` plus nullable/optional `agent`, `agent_session` (`{source, agent, kind, value}` — e.g. the Claude Code session UUID), **`cwd`**, **`foreground_cwd`**, `display_agent`, `label`, `title`, `terminal_title`, `terminal_title_stripped`, `scroll`, `state_labels{}`, `tokens{}`.

Live sample (abridged) [S1, `herdr pane list` against running 0.7.5 server]:

```json
{"id":"cli:pane:list","result":{"type":"pane_list","panes":[{
  "agent":"claude",
  "agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"be6be873-…"},
  "agent_status":"working",
  "cwd":"/home/disintegrator/github.com/speakeasy-api/_gram_podman",
  "foreground_cwd":"/home/disintegrator/github.com/speakeasy-api/_gram_podman",
  "focused":false,"pane_id":"w1S:p2","tab_id":"w1S:t1","workspace_id":"w1S","revision":39, "…":"…"}]}}
```

Caveat from a plugin author: `cwd` "is only verified present from 0.7.5"; on older herdr it may be absent [S4 `herdr-reviewr` manifest comment]. Deeper process detail (per-process cwd/argv/pid) via `herdr pane process-info` → `PaneProcessInfo{shell_pid, tty, foreground_process_group_id, foreground_processes[{pid,name,argv,argv0,cmdline,cwd}]}` [S1 help + schema].

`herdr agent list` returns the same shape (agent panes only) plus `state_change_seq` [S1 observed].

### Close / focus / navigate

- `herdr pane close <pane_id>` (socket `pane.close`); `herdr tab close <tab_id>`; `herdr workspace close <workspace_id>` [S1 help].
- `herdr pane focus --direction left|right|up|down [--pane <ID>|--current]` (directional); `herdr tab focus <tab_id>`; `herdr workspace focus <workspace_id>`; `herdr agent focus <target>` (focuses that agent's pane, any workspace) [S1 help]. There is no CLI "focus pane by id" — socket has `pane.focus` `{pane_id}` [S1 schema].
- Also: `pane move` (to tab/new-tab/workspace/new-workspace/split), `pane swap`, `pane zoom`, `pane resize`, `pane rename`, `pane read` (`--source visible|recent|recent-unwrapped|detection`, `--lines`, `--format text|ansi`), `pane wait-output --match/--regex [--timeout <MS>]` [S1 help].

### Notifications

`herdr notification show <TITLE> [--body <TEXT>] [--position top-left|top-right|bottom-left|bottom-right] [--sound none|done|request]` — socket `notification.show` `{title (required), body?, position?, sound?}` [S1 help + schema].

### Confirmation prompts

**No confirmation-prompt API exists in herdr 0.7.5.** There is no `herdr confirm` command (falls through to general help), no `confirm`-like method in the bundled request schema, and zero occurrences of "confirm" in the 248 KB API schema [S1]. The building blocks a plugin has instead: an interactive `[[panes]]` entrypoint with `placement = "overlay"` or `"popup"` (popup "receives all terminal input, including Escape", closes on `popup.close`) [S3 docs v0.7.5], plus `notification.show` with `sound = "request"`. Published plugins that "prompt" do exactly this (e.g. `herdr-plugin-jj-workspace`'s `wizard` overlay pane; `herdr-plugin-github-start`'s `prompt` overlay pane) [S4].

### Agent control (relevant to orchestration)

`herdr agent wait <TARGET> [--until idle|working|blocked|done|unknown]... [--timeout <MS>]` (default matches idle|done|blocked); `herdr agent prompt <TARGET> <TEXT> [--wait] [--until <STATUS>] [--timeout <MS>]` (submission from a non-working state requires an observed state change within 5000 ms, else `agent_prompt_stalled`); `herdr agent start <NAME> --kind claude|codex|… --pane <ID>`; `herdr agent read`; `herdr agent send-keys` [S1 help, verbatim semantics in help text]. Plugins can also *report* agent state for panes they own: `herdr pane report-agent --source <ID> --agent <LABEL> --state idle|working|blocked|unknown [--message] [--seq] …`, `report-agent-session`, `release-agent`, and display-only `report-metadata` (`--title`, `--state-label STATUS=TEXT`, `--token NAME=VALUE`, `--ttl-ms`) [S1 help].

---

## 4. Events

### Socket subscriptions (`events.subscribe` / `events.wait`)

`{"id":"sub_1","method":"events.subscribe","params":{"subscriptions":[{"type":"pane.agent_status_changed","pane_id":"w1:p1","agent_status":"blocked"}]}}` [S2]. The 0.7.5 `Subscription` union [S1 schema] accepts these types (most take only `type`; noted params otherwise):

`workspace.created`, `workspace.updated`, `workspace.metadata_updated`, `workspace.renamed`, `workspace.moved`, `workspace.closed`, `workspace.focused`, `worktree.created`, `worktree.opened`, `worktree.removed`, `tab.created`, `tab.closed`, `tab.focused`, `tab.renamed`, `tab.moved`, `pane.created`, `pane.closed`, `pane.updated`, `pane.focused`, `pane.moved`, `pane.exited`, `pane.agent_detected`, `pane.output_matched` (requires `pane_id`, `source`, `match`; optional `lines`, `strip_ansi`), `pane.agent_status_changed` (requires `pane_id`; optional `agent_status` filter), `pane.scroll_changed` (requires `pane_id`), `layout.updated`.

`events.wait` blocks for one matching event: `{match_event: EventMatch, timeout_ms?}` [S1 schema]. Delivered events are envelopes `{"event": <snake_case kind>, "data": {...}}` [S1 schema `EventEnvelope`].

### `[[events]]` manifest hooks — full 0.7.5 list

Plugin hooks are **deliberately narrower** than subscriptions: "Event names that manifest `[[events]] on` hooks can reference. This is intentionally narrower than `EventKind` until high-volume output-change hook semantics are implemented" [S3 `src/api/schema/events.rs` @ v0.7.5]. `PLUGIN_HOOK_EVENT_KINDS` at v0.7.5 (dot-names as written in `on =`):

```
workspace.created  workspace.updated  workspace.closed  workspace.renamed
workspace.moved    workspace.focused
worktree.created   worktree.opened    worktree.removed
tab.created  tab.closed  tab.renamed  tab.moved  tab.focused
pane.created  pane.closed  pane.focused  pane.moved  pane.exited
pane.agent_detected  pane.agent_status_changed
```

Not hookable at 0.7.5 (subscription-only): `workspace.metadata_updated`, `pane.updated`, `pane.output_matched`, `pane.scroll_changed`, `layout.updated`. (Master adds `workspace.reordered` — post-0.7.5.) The official docs only ever show `worktree.created` and never enumerate this list [S2/S3 docs] — the source is the only complete authority.

### What a hook receives

No stdin, no extra argv. The hook command gets the standard plugin env (section 2) plus `HERDR_PLUGIN_EVENT=<dot-name>` and `HERDR_PLUGIN_EVENT_JSON=<serialized EventEnvelope>`; `HERDR_PLUGIN_CONTEXT_JSON` is populated from the event (workspace/tab/pane ids when the event carries them) [S3 `runtime.rs` `run_plugin_event_hooks`; S3 docs].

The `HERDR_PLUGIN_EVENT_JSON` payload is `{"event": "<snake_case>", "data": {"type": "<snake_case>", ...}}` per the bundled event schema [S1]. Key payloads (0.7.5 `EventData` variants, snake_case `type`):

- **`pane_agent_status_changed`** — the agent state-change event (blocked/working/done/idle): required `pane_id`, `workspace_id`, `agent_status` (`idle|working|blocked|done|unknown`); nullable `agent`, `display_agent`, `title`; `state_labels{}` [S1 schema].
- **`pane_agent_detected`** — required `pane_id`, `workspace_id`; nullable `agent`, `final_status`; boolean `released` [S1 schema].
- `pane_created` → `{pane: PaneInfo}` (full PaneInfo incl. `cwd`); `pane_closed`/`pane_focused`/`pane_moved`/`pane_exited` → `{pane_id, workspace_id}` (+ extra fields on `pane_focused`: previous/created workspace & tab info) [S1 schema].
- `worktree_created` → `{workspace: WorkspaceInfo, worktree: WorktreeInfo}`; `worktree_opened` adds `already_open`; `worktree_removed` has `forced`, `workspace_id`, nullable `workspace`, `worktree` [S1 schema].
- `tab_created` → `{tab: TabInfo}`; `workspace_created` → `{workspace: WorkspaceInfo}`; renames carry `label`; moves carry orderings [S1 schema].

Note there is **no `agent.*` event family**; agent state arrives as `pane.agent_status_changed`. There is also no "session started/stopped" or "plugin lifecycle" event [S1 schema — absent].

---

## 5. Plugin configuration

- **There is no declarative user-facing config schema in v1.** "There is no Herdr-managed plugin storage API in v1. Plugins that need durable state should own their files or database" [S3 docs v0.7.5 "Storage"].
- Convention: "Put user-editable config such as `.env` files under `HERDR_PLUGIN_CONFIG_DIR`, and put local runtime state under `HERDR_PLUGIN_STATE_DIR`. Herdr creates those directories … but it does not validate, sync, or delete their contents. The plugin owns the file format and lifecycle" [S3 docs v0.7.5].
- Both directories are created at install/link time (`plugin install` and `plugin link` create them; `ensure_plugin_user_dirs` also runs before every command) [S3 docs; S3 `runtime.rs`].
- Users find the config dir with `herdr plugin config-dir <PLUGIN_ID>` — "prints the config directory for setup docs and shell scripts" [S1 help; S3 docs]. At runtime the plugin reads `HERDR_PLUGIN_CONFIG_DIR` from its environment [S3 docs].
- The exact on-disk location of the config/state dirs is **not documented** in the fetched pages (it is an internal path under herdr's data dirs; discover it via `plugin config-dir`).

---

## 6. Distribution: `herdr plugin install` and marketplace discovery

### Install [S1 help; S3 docs v0.7.5]

```
herdr plugin install [--ref <REF>] [-y|--yes] <OWNER/REPO[/SUBDIR]>
```

- GitHub shorthand **only** (`owner/repo` or `owner/repo/subdir`); no arbitrary URLs. It "clones with `git`, shows a preview in interactive terminals, runs supported build commands, then stores the checkout under Herdr-managed plugin data and registers it" [S3 docs]. `--yes` for noninteractive installs; `--ref` pins a git revision.
- Requirements on the repo: a normal public GitHub repository with `herdr-plugin.toml` at the repo root or in the named subdirectory — "Publish a normal public GitHub repository with a herdr-plugin.toml manifest at its root, or in a subdirectory, and that command works" [S2 marketplace docs]. **No tags, releases, or build artifacts are required** ("The documentation does not specify requirements for tags, releases, or build artifacts") [S2]; prebuilt binaries are an optional pattern plugins implement themselves in `[[build]]` [S4 file-viewer, reviewr].
- Registration is global to the user, across sessions; works while no server is running. Reinstall replaces the managed checkout (that is also the "update" story — "There is no separate `plugin update` in v1"). Installing over a locally linked plugin is refused. `plugin uninstall <id-or-source>` removes the managed checkout; `plugin unlink <id>` leaves files alone [S3 docs].
- Local development: `herdr plugin link [--enabled|--disabled] <PATH>` (no build commands run); `enable`/`disable` toggle; `plugin list [--json]` shows registered plugins including manifest contents, `source` (`local|github` + owner/repo/subdir/requested_ref/resolved_commit/managed_path), and `warnings` [S1 help + schema].
- The plugin id comes solely from the manifest `id` field, not the repo name [S3 docs example flow; S1 schema].

### Marketplace / `herdr-plugin` topic [S2 marketplace docs; S3 docs]

- "Add the GitHub topic `herdr-plugin` to a public repository. That topic is the only signal the index uses." Index refreshes every 30 minutes; forks and archived repos are excluded; listing metadata comes from GitHub (name, owner, description, stars, language, last push) and "the `herdr-plugin.toml` manifest fields are not parsed for display in version 1."
- Discovery works: `gh search repos --topic herdr-plugin` returned 30+ live plugins on 2026-07-28 (crabbox, herdr-file-viewer, herdr-reviewr, herdr-plus, herdr-browser, …) [S4].
- Official example cookbook: `ogulcancelik/herdr-plugin-examples` with subdirectory plugins (`agent-telegram-notify`, `github-link-preview`, `dev-layout-bootstrap`) — "examples to copy, not maintained official plugins" [S3 docs v0.7.5].

---

## 7. Gaps, surprises, contradictions

- **No confirmation-prompt primitive.** Anything needing user confirmation must be built from an overlay/popup `[[panes]]` entrypoint (or misuse `notification … --sound request`, which is non-interactive) [S1 schema — no such method].
- **Pane cwd IS exposed** (`cwd` + `foreground_cwd` in `pane list`/`agent list`/`pane.get`), but only reliably from 0.7.5 [S1 observed; S4 reviewr comment].
- **The hookable event list is undocumented.** Docs show a single example (`worktree.created`); the authoritative list is `PLUGIN_HOOK_EVENT_KINDS` in the source (21 events at v0.7.5). Unknown names are silently downgraded to `plugin.list` warnings — a typo in `on =` will not error [S3 source; S1 schema].
- **CLI vs socket placement mismatch:** `herdr plugin pane open --placement` offers only `overlay|split|tab|zoomed` at 0.7.5 while the socket/manifest also support `popup` (with `width`/`height`) [S1 help vs S1 schema].
- **`herdr plugin` is a hidden top-level command** — absent from `herdr --help`, discovered only via `herdr plugin --help` [S1].
- **No plugin daemon model:** startup hooks are one-shot; long-running plugin logic must live in a pane command or an externally-managed process. Output capped at 64 KiB, 32 concurrent commands, 200 log entries [S3 docs + `runtime.rs`].
- Docs pages at herdr.dev are the rendered form of `website/src/content/docs/*.mdx` in the source repo; the repo's `docs/versions/0.7.5/` tree pins the docs for the installed version — the two agreed everywhere checked [S2 vs S3].
- `min_herdr_version` looks optional in TOML but is enforced as required at validation [S3 `manifest.rs`; S1 strings].
