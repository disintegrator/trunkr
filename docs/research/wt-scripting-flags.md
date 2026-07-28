# wt flags for scripted worktree operations — research findings (wt v0.69.2)

Researched 2026-07-28 against primary sources only:

- **S1 — local binary**: worktrunk `wt v0.69.2` (`command wt --version`). `--help` output for `wt`, `wt switch`, `wt merge`, `wt remove`, `wt list`, plus live probes. All mutating probes (`switch --create`, `merge`, `remove`) were run only in a throwaway git repo under the session scratchpad — never against a real repo. Probes ran with stdin redirected from `/dev/null`, so every "verified live" claim about non-TTY behavior is against a genuinely non-interactive stdin.
- **S2 — worktrunk source**: github.com/max-sixty/worktrunk checked out at tag `v0.69.2` (commit `5e03e28`), the exact source of the installed binary. File paths below are relative to that repo. Note: `v0.69.2` is also the **latest release tag** as of 2026-07-28 (`git ls-remote --tags`, version-sorted); `main` (`4c84559`) is 38 unreleased commits ahead.
- **Prior art**: trunkr issue #3 resolution (written against the same wt v0.69.2). Its claims were re-verified here rather than trusted; nothing below contradicts it, and several schema shapes are now captured from live output instead of source-only reading.

Claims are cited as [S1]/[S2] with the command or file. "Verified live on wt v0.69.2" means observed in the throwaway-repo probes; "source-derived" means read from S2 but not executed.

---

## 1. Base ref for `wt switch --create`

**Yes — `-b, --base <BASE>` exists and accepts more than branches.** No workaround needed.

- `wt switch --create <branch> --base <BASE>` creates the new branch from `BASE`; without `--base` it defaults to the repo's default branch (main/master) — **not** current HEAD [S1 `wt switch --help`: "The --create flag creates a new branch from --base — the default branch unless specified"].
- `--base` accepts the same shortcuts as the branch argument: `^` (default branch), `@` (current branch — i.e. `--base=@` gives the "branch from current HEAD" behavior), `-` (previous worktree), `pr:{N}`, `mr:{N}` [S1 help]. For a fork PR/MR base, the head commit is fetched and used as the base SHA without creating a tracking branch [S1 help].
- **Verified live on wt v0.69.2** in the throwaway repo, all exit 0 with `--format json` echoing the base:
  - `--base other` (branch): new branch started at `other`'s tip (confirmed via `git log`); JSON `{"action":"created","branch":"feat1","path":"…/repo.feat1","created_branch":true,"base_branch":"other"}`.
  - `--base <full 40-char SHA>` (raw commit): accepted, `base_branch` echoes the SHA.
  - `--base mytag` (tag): accepted.
- Constraint: `--create` cannot be combined with a `pr:`/`mr:` **branch** argument (the branch already exists) [S1 help]. Using `pr:N` as `--base` is fine.
- Related: switching to a branch that exists only on a remote (`wt switch feature` with only `origin/feature`) auto-creates a local tracking branch — no `--create` needed; the JSON then carries `from_remote` [S1 help; S2 `src/commands/worktree/switch.rs` `SwitchJsonOutput`].

## 2. `--format json` coverage and schemas

There is no separate `create` command — creation is `wt switch --create` — so the JSON surface is **switch, merge, remove, list: all four have `--format <text|json>` / `--format <table|json>`** [S1 per-command `--help`]. In every case the JSON payload is the only thing on stdout; all progress/success/warning text goes to stderr (verified live: `2>/dev/null` leaves pure JSON).

### `wt switch --format json`

Single JSON object, one line, printed to stdout [S2 `src/commands/worktree/switch.rs:1278` `SwitchJsonOutput`; verified live]:

```json
{"action":"created","branch":"feat1","path":"/abs/worktree/path","created_branch":true,"base_branch":"other"}
{"action":"existing","branch":"feat4","path":"/abs/worktree/path"}
{"action":"already_at","branch":"main","path":"/abs/worktree/path"}
```

Schema: `action`: `"created" | "existing" | "already_at"`; `branch`: string|null; `path`: absolute worktree path (always present — this is the canonical way to learn where a worktree lives); optional `created_branch` (bool), `base_branch` (string, echoes what was passed — branch name, SHA, or tag), `from_remote` (string, remote tracking branch if auto-created). Optional fields are omitted, not null [S2 `#[serde(skip_serializing_if)]`].

### `wt merge --format json`

Pretty-printed object to stdout **after** the merge completes [S1 help; S2 `src/commands/merge.rs:458`; verified live]:

```json
{
  "branch": "feat1",
  "committed": false,
  "rebased": false,
  "removed": false,
  "squashed": true,
  "target": "main"
}
```

`branch`/`target` strings; `committed`/`squashed`/`rebased`/`removed` booleans reporting which pipeline steps actually happened. Note key order in serde_json is alphabetical in practice; don't depend on it.

### `wt remove --format json`

Pretty-printed JSON **array** (one element per removed target, also for the single-target case) to stdout [S2 `src/commands/remove.rs:369,448`; verified live]:

```json
[
  {
    "branch": "feat1",
    "branch_deleted": true,
    "kind": "worktree",
    "path": "/abs/worktree/path"
  }
]
```

Two element shapes [S2 `src/commands/worktree/types.rs:227` `RemoveResult::to_json`]:
- worktree removal: `{"kind":"worktree", "branch": string|null, "path": string, "branch_deleted": bool}`
- branch-only deletion (no worktree existed): `{"kind":"branch_only", "branch": string|null, "pruned": bool, "branch_deleted": bool}`

Caution: without `--foreground`, the JSON is printed after the removal is *scheduled* (background rename into `.git/wt/trash/` + detached `rm -rf`), not after the path is gone [S1 `wt remove --help` "Background removal"]. Pass `--foreground` when the path must be free.

### `wt list --format json`

Mid-migration between two schemas, controlled by `[list] json-schema` (config key, so settable per-invocation with `--config-set 'list.json-schema=1|2'`) [S2 `src/commands/list/mod.rs:201-271`; verified live]:

- **Schema 1 (current default)**: bare array of item objects; emitting it prints a deprecation warning on stderr: "JSON output is schema 1; a future release switches the default to schema 2" (verified live). Item shape (`JsonItem`, [S2 `src/commands/list/json_output.rs`]): `branch` (string|null), `path`, `kind` (`"worktree"|"branch"`), `commit` `{sha, short_sha, message, timestamp}`, `working_tree` `{staged, modified, untracked, renamed, deleted, diff:{added,deleted}}`, `main_state` (`is_main|would_conflict|same_commit|integrated|diverged|ahead|behind`), `integration_reason`, `operation_state` (`conflicts|rebase|merge`), `main` `{ahead, behind, diff}`, `remote`, `worktree` `{detached, …}`, `is_main`/`is_current`/`is_previous` (bools), `ci`, `repo_url`, `repo`, `url`, `url_active`, `summary`, `statusline`, `symbols`, `vars`, `columns`. Most fields omitted when absent.
- **Schema 2 (opt-in, the future default)**: envelope object [S2 `src/commands/list/json_v2.rs`; verified live]:

```json
{
  "schema": 2,
  "repo": { "default_branch": "main" },
  "collected": { "ci": false, "summary": false },
  "items": [
    {
      "branch": "main",
      "head": { "sha": "…", "short_sha": "…", "subject": "init", "committed_at": "2026-07-28T19:31:32Z" },
      "worktree": {
        "path": "/abs/path", "main": true, "current": true, "previous": true,
        "detached": false, "branch_mismatch": false,
        "changes": { "staged": false, "modified": false, "untracked": false, "renamed": false, "deleted": false, "conflicted": false, "diff": { "added": 0, "deleted": 0 } }
      },
      "display": { "state": "is_main", "symbols": "^", "statusline": "main …" }
    }
  ]
}
```

  Items also carry (when applicable) `remote` (remote name for remote-only rows), `default_branch` (relation object), `integration`, `upstream`, `pr`, `checks`, `dev_server`, `vars`, `columns` [S2 `JsonItemV2`].
- **Scripting recommendation**: pin the schema explicitly — `wt --config-set 'list.json-schema=2' list --format=json` — so the future default flip is a no-op (and the stderr warning disappears). Verified live.
- Gotcha: the `statusline` string fields embed raw ANSI escapes even when piped (observed live); use `symbols` or the structured booleans instead.

## 3. Force/yes flags, hooks, and headless behavior

### Flag surface [S1 per-command `--help`]

- **Global (every command)**: `-y, --yes` — "Skip approval prompts". `--no-hooks` on switch/merge/remove — skip hooks entirely.
- **`wt remove`**: `-f, --force` (remove a *dirty* worktree — without it, uncommitted changes fail the removal, verified live exit 1); `-D, --force-delete` (delete an *unmerged* branch); `--no-delete-branch` (keep branch); `--foreground` (block until the path is actually gone); `--reap` (kill processes cwd'd under the worktree; experimental, Unix only, spares TTY-holding processes).
- **`wt merge`**: no confirmation prompt of its own — the flags shape the pipeline (`--no-squash`, `--no-commit`, `--no-rebase`, `--no-ff`, `--no-remove`, `--stage all|tracked|none`). A rebase conflict aborts with the rebase left open in the worktree (observed live, exit 1).

### The only interactive prompt in the lifecycle path is hook/command approval

- Project-config commands (hooks in `.config/wt.toml`, aliases) are approval-gated, batch-approved once at the command entry point ("approve at the gate") [S2 `src/commands/command_approval.rs` module docs].
- **Non-TTY stdin fails fast, it does not hang** [S2 `src/commands/command_approval.rs:123-128`]: before prompting, wt checks `stdin.is_terminal()`; if not a terminal it prints the pending command templates to stderr (so they appear in CI logs) and errors with `GitError::NotInteractive`. Verified live on wt v0.69.2: `wt merge --format json </dev/null` with an unapproved `[pre-merge]` hook printed "✗ Cannot prompt for approval in non-interactive environment" + hint "To skip prompts in CI/CD, add --yes; to pre-approve commands, run wt config approvals add", exit 1, hook not run.
- `-y/--yes` approves for this invocation only — approvals are persisted **only** on interactive approval, never with `--yes` [S2 `command_approval.rs:74` comment]. Verified live: same merge with `-y` ran the hook and completed. Alternatives: pre-approve via `wt config approvals add`, or skip hooks with `--no-hooks`.
- Hook types are pre/post × switch, start, commit, merge, remove — 10 total; `pre-create`/`post-create` are accepted aliases for `pre-start`/`post-start` [S2 `src/git/mod.rs:473-494`]. `pre-*` hooks block and fail-fast; `post-*` hooks run detached in the background after success [S2 `src/git/mod.rs` `HookType::is_pre` docs] — so **post-merge/post-remove hooks can never block a headless caller**; only pre-hooks can fail the command, and only the approval step can attempt to prompt.
- Other headless behaviors verified live: `wt switch` with **no branch argument** on non-TTY stdin errors "Interactive picker requires an interactive terminal" (exit 1) — scripts must always pass a branch. `wt merge`'s squash-commit-message generation does **not** prompt when no LLM is configured; it uses a fallback message ("Squash commits from <branch>…") and proceeds.

### Merge defaults a script should know

`wt merge` by default: stages + squashes everything (including untracked files, `--stage all`), rebases onto target, fast-forwards target, then **removes the worktree and branch** (background) and never fetches — it targets the local default-branch ref [S1 `wt merge --help`]. Use `--no-remove` to keep the worktree, `--stage none`/`--no-commit` to control what gets swept in.

## 4. Everything else relevant to scripting

### Exit codes

- `0` success; generic errors exit `1` (verified live: unknown branch, dirty-worktree removal, NotInteractive, rebase conflict).
- **Failing hook/child exit codes propagate**: `WorktrunkError::HookCommandFailed`/`ChildProcessExited` carry the child's code, `Interrupted` maps to `128 + signal`, and `main.rs` exits with `error.exit_code().unwrap_or(1)` [S2 `src/git/error.rs:1525-1533`, `src/main.rs:97,1055`]. Verified live: a `[pre-merge]` hook running `exit 42` made `wt merge -y` exit **42**.
- Practical rule: treat exit codes as boolean success/failure, except that a nonzero code may be a propagated hook/child code rather than wt's own.

### stdout/stderr conventions

- stdout is reserved for machine output: `--format json` payloads (and nothing else in the probes). All progress (`◎`), success (`✓`/`○`), warnings (`▲`), errors (`✗`) and hints (`↳`) go to stderr. Verified live on all four commands.
- Color/styling via `anstream` auto-detection [S2 `src/styling/mod.rs:26-27`, `Cargo.toml`] — ANSI is stripped when the stream is not a terminal (and anstream honors `NO_COLOR`/`CLICOLOR_FORCE`; documented crate behavior, unverified here). Exception noted above: `wt list` JSON `statusline` strings embed ANSI by design.

### `cd` never happens in the binary

The directory change after `wt switch` is implemented by a shell wrapper function: the binary writes directives to temp files named in `WORKTRUNK_DIRECTIVE_CD_FILE` / `WORKTRUNK_DIRECTIVE_EXEC_FILE` and the wrapper `cd`s/`source`s them [S1: the installed `wt` is such a zsh function wrapping `command wt`; S2 env usage]. A scripted caller invoking the binary directly (`command wt` / absolute path) never has its cwd changed — pass `--no-cd` (documented "for CI/automation" [S1 `wt switch --help`]) and take the path from the JSON. `-C <path>` sets the working directory for a single command; `-x/--execute <cmd>` *replaces* the wt process with the command (template vars like `{{ worktree_path }}` supported) [S1 help].

### Env knobs [S2 `src/config/user/mod.rs:113-145` + grep of v0.69.2 source]

- **Any user-config key can be set by env var**: `WORKTRUNK_<KEY>` for top-level (`WORKTRUNK_WORKTREE_PATH=…` → `worktree-path`), double underscore for nesting (`WORKTRUNK__LIST__TIMEOUT_MS=30` → `[list] timeout-ms`, `WORKTRUNK_COMMIT__GENERATION__COMMAND=cmd` → `[commit.generation] command`); segments convert to kebab-case.
- Excluded from that mechanism (infrastructure): `WORKTRUNK_CONFIG_PATH` (user config file), `WORKTRUNK_SYSTEM_CONFIG_PATH`, `WORKTRUNK_APPROVALS_PATH` (approvals store — useful for isolating scripted runs; used for exactly that in these probes, verified live). `WORKTRUNK_TEST_*` are ignored.
- `WORKTRUNK_VERBOSE=0|1|2` — same as `-v`/`-vv` everywhere [S1 `wt --help`]. `-vv` also writes raw subprocess output to `.git/wt/logs/`.
- `WORKTRUNK_BIN` — binary the shell wrapper invokes [S1 wrapper function; S2].
- Config can also be overridden per-invocation with repeatable `--config-set '<toml>'` (e.g. the list schema pin above) and `--config <path>` [S1 `wt --help`].

### Misc

- Project config lives at `<repo>/.config/wt.toml`, checked into git [S2 `src/config/mod.rs:8`, `src/config/project.rs:180`]; hook approval state is per-project-id in the approvals file.
- Removal logs land in `.git/wt/logs/{branch}/internal/remove.log`; trash entries older than 24h are swept by later `wt remove` runs [S1 `wt remove --help`].
- Detached-HEAD worktrees have no branch name — `wt remove` takes the worktree *path* instead [S1 `wt remove --help`].
- `wt list` also has `--branches`, `--remotes`, `--full` (CI + LLM summaries) to widen/enrich the JSON, and `wt list statusline` for single-line status [S1 `wt list --help`].
