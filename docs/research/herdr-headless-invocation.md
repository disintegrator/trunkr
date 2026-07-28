# herdr headless invocation surface — research findings

Researched 2026-07-28 for issue #43 (part of #42), against primary sources only:

- **S1 — local binary, live**: herdr 0.7.5 at `/home/disintegrator/.local/share/mise/installs/github-ogulcancelik-herdr/0.7.5/herdr` (`herdr --version` → `herdr 0.7.5`; `herdr status --json` → client and server both `0.7.5`, protocol 17, stable channel, socket `/home/disintegrator/.config/herdr/herdr.sock`). Help output, `herdr api schema --json` (protocol 17), and live probing from inside an ordinary herdr agent pane (`w22:p1`), including a throwaway locally-linked plugin used to capture the exact env an action process receives (linked, invoked via CLI and via a raw Python Unix-socket client, then unlinked — registry left clean).
- **S2 — official docs**: herdr.dev content, read from the source-of-truth MDX in the herdr repo: `docs/versions/0.7.5/website/src/content/docs/{cli-reference,socket-api,agent-automation}.mdx` (the pinned docs for the installed version) and `website/src/content/docs/*` on `master` (the currently published docs). `cli-reference.mdx` and `socket-api.mdx` are byte-identical between the 0.7.5 pin and master (verified by diff).
- **S3 — source repo**: github.com/ogulcancelik/herdr (Rust), read at tag `v0.7.5` and at `master` (pushed 2026-07-28): `src/cli/plugin.rs`, `src/cli/worktree.rs`, `src/app/api/plugins/{mod,context,runtime,env}.rs`, `src/pane.rs`, `src/integration/env.rs`.
- **S4 — releases**: `gh release list --repo ogulcancelik/herdr` on 2026-07-28.

Each claim is marked **[live 0.7.5]** (executed on this machine against the running server), **[docs]**, **[source vX]**, or **[releases]**.

## Version landscape

- Installed: **0.7.5**. Latest stable release: **v0.7.5** (2026-07-21). The only newer artifacts are a pre-release preview build (`preview-2026-07-21-0f10e1453a7f`) and unreleased `master` commits [S4; S1]. So "current release" and "installed" are the same version; where behavior might have moved, `master` was checked too.

---

## Q1 — Can `herdr plugin action invoke` pass arguments or a target context?

**Bottom line: the CLI cannot — in 0.7.5, in the latest release (also 0.7.5), and on unreleased master. The socket method underneath can: `plugin.action.invoke` accepts a full `PluginInvocationContext`, and caller-provided fields override the server's UI-derived context. Verified live.**

- CLI surface at 0.7.5: `herdr plugin action invoke <ACTION_ID> [--plugin <ID>]` — `--plugin` is the only option; anything else is rejected with `unknown option` [live 0.7.5 `herdr plugin action invoke --help`; source v0.7.5 `src/cli/plugin.rs` `plugin_action_invoke`]. This confirms issue #41's observation.
- Master (2026-07-28) is unchanged: `src/cli/plugin.rs` differs from v0.7.5 only in an internal cwd-handling refactor; the invoke arg parser still accepts only `--plugin` [source master]. No released or in-flight version adds argument or target flags to the CLI.
- What the CLI actually sends: `Method::PluginActionInvoke` with `context: Some(PluginInvocationContext { invocation_source: Some("cli"), ..all None })` — so a CLI invocation is distinguishable via `invocation_source: "cli"`, but carries no target [source v0.7.5 + master `src/cli/plugin.rs` ~line 453].
- Socket params (protocol 17): `PluginActionInvokeParams { action_id (required), plugin_id?, context?: PluginInvocationContext }` where the context has nullable `workspace_id`, `workspace_label`, `workspace_cwd`, `worktree`, `tab_id`, `tab_label`, `focused_pane_id`, `focused_pane_cwd`, `focused_pane_agent`, `focused_pane_status`, `selected_text`, `clicked_url`, `link_handler_id`, `invocation_source`, `correlation_id` [live 0.7.5 `herdr api schema --json`].
- Merge semantics: the server computes the current UI context, then **provided fields win** field-by-field (`context.workspace_id = provided.workspace_id.or(context.workspace_id)`, etc.) [source v0.7.5 `src/app/api/plugins/context.rs` `merge_plugin_context`].
- **Verified live on 0.7.5**: a raw socket request `{"method":"plugin.action.invoke","params":{"action_id":"…","context":{"focused_pane_id":"wPROBE:p9","selected_text":"arg-smuggle-test","invocation_source":"trunkr-headless-test"}}}` succeeded; the response context and the action process env showed `HERDR_PANE_ID=wPROBE:p9` (accepted **verbatim, no validation** — `wPROBE:p9` does not exist) and `selected_text` passed through in `HERDR_PLUGIN_CONTEXT_JSON`. Unprovided fields (`workspace_id`, `tab_id`, cwds) were backfilled from the UI-focused workspace.
- There is **no arbitrary-arguments parameter anywhere**: an action's argv is fixed by the manifest; the only variable channel is the context (env / `HERDR_PLUGIN_CONTEXT_JSON`). `selected_text` (or `correlation_id`) is a workable, if unglamorous, data channel for a socket caller [live 0.7.5; source v0.7.5].

## Q2 — What env does a CLI-invoked action process receive?

**Bottom line: the same env as a UI invocation, with IDs derived from the UI-focused workspace — active workspace, its active tab, its focused pane — not from the pane the CLI command ran in. Verified live on 0.7.5 via a throwaway plugin.**

Captured live (probe action ran `env | sort` to a file):

```
HERDR_BIN_PATH=/home/disintegrator/.local/share/mise/installs/github-ogulcancelik-herdr/0.7.5/herdr
HERDR_ENV=1
HERDR_PANE_ID=w22:p1
HERDR_PLUGIN_ACTION_ID=dumpenv
HERDR_PLUGIN_CONFIG_DIR=/home/disintegrator/.config/herdr/plugins/config/<plugin_id>
HERDR_PLUGIN_CONTEXT_JSON={"workspace_id":"w22","workspace_label":"trunkr","workspace_cwd":"…","tab_id":"w22:t1","tab_label":"1","focused_pane_id":"w22:p1","focused_pane_cwd":"…","focused_pane_agent":"claude","focused_pane_status":"working","invocation_source":"cli","correlation_id":"cli:plugin"}
HERDR_PLUGIN_ID=<plugin_id>
HERDR_PLUGIN_ROOT=<plugin dir>
HERDR_PLUGIN_STATE_DIR=/home/disintegrator/.local/state/herdr/plugins/<plugin_id>
HERDR_SOCKET_PATH=/home/disintegrator/.config/herdr/herdr.sock
HERDR_TAB_ID=w22:t1
HERDR_WORKSPACE_ID=w22
```

plus cwd = plugin root (`PWD=<plugin dir>`) [live 0.7.5].

- Which pane the IDs point at: with no context in the request, the server uses `current_plugin_context` → the **active (UI-focused) workspace** (`self.state.active`), that workspace's active tab, and that workspace's `focused_pane_id()` [source v0.7.5 `src/app/api/plugins/context.rs`]. The CLI passes no pane identity of its own (Q1), so a CLI invocation from a background pane still targets whatever pane the UI has focused. (In the live probe the invoking pane happened to be the focused pane, so the values coincided; the derivation-from-UI-state claim is source-verified, and the override probe confirmed env comes from the merged context, not the caller.)
- `HERDR_WORKSPACE_ID`/`HERDR_TAB_ID`/`HERDR_PANE_ID` are set from the merged context's `workspace_id`/`tab_id`/`focused_pane_id`, each omitted when null; `HERDR_BIN_PATH` comes from the server's `current_exe()` [source v0.7.5 `src/app/api/plugins/runtime.rs` lines ~45–75; live-confirmed].
- If no workspace is active, the context is empty (only `invocation_source`/`correlation_id`) and the ID vars are unset [source v0.7.5 `empty_plugin_context`; **not exercised live**].
- The action can detect CLI invocation via `invocation_source: "cli"` in `HERDR_PLUGIN_CONTEXT_JSON` [live 0.7.5].
- Docs corroborate the var list ("Herdr injects `HERDR_SOCKET_PATH`, `HERDR_BIN_PATH`, `HERDR_ENV=1`, `HERDR_PLUGIN_ID`, `HERDR_PLUGIN_ROOT`, `HERDR_PLUGIN_CONFIG_DIR`, `HERDR_PLUGIN_STATE_DIR`, `HERDR_PLUGIN_CONTEXT_JSON`, and available `HERDR_WORKSPACE_ID`, `HERDR_TAB_ID`, and `HERDR_PANE_ID` … Action commands also receive `HERDR_PLUGIN_ACTION_ID`") [docs socket-api.mdx, identical at 0.7.5 and master].

## Q3 — Can a directly-executed binary (not plugin-launched) reach the socket and act?

**Bottom line: yes, fully. Every managed herdr pane's env carries `HERDR_SOCKET_PATH` plus its own workspace/tab/pane IDs; the socket has no authentication or plugin gating, and driving it from ordinary panes/scripts is the documented model. `HERDR_BIN_PATH` is NOT in ordinary pane env — it is plugin-runtime-only.**

- **Verified live**: this research ran inside an ordinary herdr agent pane. Its env: `HERDR_ENV=1`, `HERDR_SOCKET_PATH=/home/disintegrator/.config/herdr/herdr.sock`, `HERDR_WORKSPACE_ID=w22`, `HERDR_TAB_ID=w22:t1`, `HERDR_PANE_ID=w22:p1`. **No `HERDR_BIN_PATH`** (`env | grep -i herdr`). So a directly-executed `bin/trunkr` can self-locate the socket and its own pane, but must find the `herdr` binary via `PATH` (or talk to the socket directly) rather than via `HERDR_BIN_PATH`.
- Source and docs agree: every pane launch gets `HERDR_ENV=1` + `apply_pane_base_env` (which sets only `HERDR_SOCKET_PATH`), and managed panes additionally get the three ID vars [source v0.7.5 `src/pane.rs` lines ~112–131, `src/integration/env.rs` `apply_pane_base_env`]; "Herdr also injects `HERDR_SOCKET_PATH`, `HERDR_ENV=1`, `HERDR_WORKSPACE_ID`, `HERDR_TAB_ID`, and `HERDR_PANE_ID` into managed pane processes" [docs socket-api.mdx]. `HERDR_BIN_PATH` appears only in the plugin runtime env builder [source v0.7.5 `src/app/api/plugins/runtime.rs`]; docs recommend it "for plugins" [docs socket-api.mdx].
- **Acting is unrestricted.** No auth handshake exists in the protocol (requests are bare `{id, method, params}` NDJSON [docs socket-api.mdx]); nothing distinguishes plugin-launched callers. Verified live from this non-plugin pane: read-only calls (`herdr pane current`, `herdr pane list`, `herdr worktree list`, `herdr status --json`) **and mutations** (`herdr plugin link`/`unlink`, `plugin.action.invoke` via both the CLI and a hand-rolled Python Unix-socket client) all succeeded [live 0.7.5]. Pane/workspace open/focus methods (`workspace.create`, `pane.split`, `pane.focus`, `tab.create`, …) sit on the same unauthenticated surface — deliberately not exercised live to avoid disturbing the running session, but the agent-automation docs are explicit that this is supported usage: "Herdr can act as an automation layer … A script can control them, or one agent can create work for other agents", with shell examples running `herdr workspace create` / `herdr pane split` from scripts [docs agent-automation.mdx 0.7.5].
- Processes outside herdr entirely (no env) can still connect: socket resolution is documented as CLI `--session` → `HERDR_SOCKET_PATH` → `HERDR_SESSION` → default `~/.config/herdr/herdr.sock` (named sessions under `~/.config/herdr/sessions/<name>/herdr.sock`) [docs socket-api.mdx; live default confirmed via `herdr status --json`].

## Q4 — `herdr worktree create` flags and `--json` output surface

**Bottom line: `create` takes `[--workspace ID | --cwd PATH] [--branch NAME] [--base REF] [--path PATH] [--label TEXT] [--focus|--no-focus] [--json]` and returns `{type:"worktree_created", workspace, tab, root_pane, worktree}`. Notably, `--json` is accepted but ignored — the CLI always emits the raw JSON response envelope.**

Full help, captured live on 0.7.5 (flags only; help text carries no descriptions):

```
herdr worktree list   [--workspace ID | --cwd PATH] [--json]
herdr worktree create [--workspace ID | --cwd PATH] [--branch NAME] [--base REF]
                      [--path PATH] [--label TEXT] [--focus] [--no-focus] [--json]
herdr worktree open   [--workspace ID | --cwd PATH] (--path PATH | --branch NAME)
                      [--label TEXT] [--focus] [--no-focus] [--json]
herdr worktree remove --workspace ID [--force] [--json]
```

- Semantics [docs cli-reference.mdx, identical at 0.7.5 and master]: "`worktree create` creates a Git worktree checkout, opens it as a workspace, and groups it with the parent repo workspace. If `--branch` names an existing local branch, Herdr checks it out; otherwise it creates the branch from `--base` or `HEAD`. Without `--path`, Herdr creates the checkout under `<worktrees.directory>/<repo>/<branch-slug>`." `worktree remove` "runs `git worktree remove`, never deletes the branch, and requires `--force` when Git refuses a dirty checkout."
- `--workspace` and `--cwd` are mutually exclusive (usage error if both) [source v0.7.5 `src/cli/worktree.rs`]. `focus` defaults to `false` [live schema `WorktreeCreateParams.focus.default`; docs: creation "leave[s] focus unchanged by default"].
- **`--json` is a no-op**: the arg parser swallows it without setting anything (`"--json" => index += 1`) and the command always prints the raw response envelope [source v0.7.5 `src/cli/worktree.rs`; live-confirmed — `herdr worktree list` output is the envelope with or without the flag]. Envelope shape: `{"id":"cli:worktree:list","result":{…}}`.
- `worktree.create` success result (protocol 17 schema, `ResponseResult` variant; all four fields required):

```json
{"type": "worktree_created",
 "workspace": WorkspaceInfo,   // workspace_id, label, number, focused, active_tab_id,
                               // tab_count, pane_count, agent_status, tokens, worktree
 "tab":       TabInfo,
 "root_pane": PaneInfo,        // pane_id, workspace_id, tab_id, cwd, foreground_cwd, agent_status, …
 "worktree":  WorktreeInfo}
```

  (A second, two-field `worktree_created {workspace, worktree}` in the schema is the *event* payload, not the RPC response.) [live 0.7.5 `herdr api schema --json`]
- `WorktreeInfo` fields: `path` (required), `branch?`, `is_bare`, `is_detached`, `is_prunable`, `is_linked_worktree` (required booleans), `label` (required), `open_workspace_id?` [live schema]. Real `worktree list` output verified live, e.g. `{"branch":"main","is_bare":false,"is_detached":false,"is_linked_worktree":false,"is_prunable":false,"label":"trunkr","open_workspace_id":"w22","path":"/home/disintegrator/github.com/disintegrator/trunkr"}`, and the list result also carries a `source` object (`repo_key`, `repo_name`, `repo_root`, `source_checkout_path`, `source_workspace_id`) [live 0.7.5].
- The ergonomic model in herdr's own words: "Creation commands print JSON. Capture IDs from the response instead of predicting them" — e.g. `jq -r '.result.root_pane.pane_id'` [docs agent-automation.mdx 0.7.5]. (Real `worktree create` output was not exercised live — it would create a checkout and workspace in the user's session; shape above is from the bundled schema, which matched live output everywhere it was checked.)

## Implications for trunkr (observations, not decisions)

- The CLI action launcher is a dead end for targeted headless invocation on any released herdr; the raw socket (`plugin.action.invoke` with a context override) or plain CLI wrappers from an ordinary pane are the viable paths today.
- Since every managed pane already has `HERDR_SOCKET_PATH` + its own IDs, `bin/trunkr` run directly from an agent pane has strictly *more* self-knowledge than a CLI-invoked action (which sees the UI-focused pane, not the caller's).
- The `worktree create` ergonomics to mirror: noun-verb CLI over the socket, mutually-exclusive repo selectors (`--workspace|--cwd`), `--focus/--no-focus` defaulting to no-focus, and an always-JSON envelope whose result returns every created resource with its IDs.
