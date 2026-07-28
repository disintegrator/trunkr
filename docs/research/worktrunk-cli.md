# Research: worktrunk CLI surface

- **Source repo:** <https://github.com/max-sixty/worktrunk> (docs site: <https://worktrunk.dev>)
- **Version inspected:** locally installed `wt v0.69.2` (`wt --version`), which matches the latest GitHub release `v0.69.2` (published 2026-07-25). Repo source inspected at commit `21ea6d6f1ae173977b31b35dbe14c4225b687d91` (HEAD of `main`, 2026-07-28).
- **Primary sources used:** `wt <cmd> --help` output from the installed v0.69.2 binary, and repo files (`docs/content/*.md`, `src/**/*.rs`) at the commit above. Cited inline per claim.

Worktrunk is "a CLI for Git worktree management, designed for parallel AI agent workflows" (repo description). Top-level commands: `switch`, `list`, `remove`, `merge`, `step`, `hook`, `config` (`wt --help`, v0.69.2). Global flags on every command: `-C <path>` (working directory), `--config <path>`, `--config-set <toml>` (inline TOML override, repeatable, highest priority), `-v/-vv`, `-y/--yes` (skip approval prompts) (`wt --help`, v0.69.2).

---

## 1. Core commands: semantics, flags, scripted use

### `wt switch [BRANCH] [-- <EXECUTE_ARGS>...]`

"Switch to a worktree; create if needed" (`wt switch --help`, v0.69.2).

- `BRANCH` accepts a branch name, shortcut (`^` default branch, `-` previous, `@` current, `pr:{N}`, `mr:{N}`), or a PR/MR web URL. Omitting it opens an interactive picker (`wt switch --help`).
- Flags: `-c/--create` (create new branch), `-b/--base <BASE>` (base branch, defaults to default branch; accepts the same shortcuts), `-x/--execute <EXECUTE>` (replaces the `wt` process with a command after switching; supports template vars like `{{ worktree_path }}`), `--clobber` (remove stale non-worktree directory at target path), `--no-cd` (skip directory change; "Useful ... for CI/automation"), picker wideners `--branches` / `--remotes` / `--prs`, and automation flags `--no-hooks` and `--format <text|json>` (`wt switch --help`).
- **JSON output:** `--format json` "prints structured result to stdout. Designed for tool integration (e.g., Claude Code WorktreeCreate hooks)" (`wt switch --help`). The exact shape, from `src/commands/worktree/switch.rs` (struct `SwitchJsonOutput`, commit `21ea6d6`):

  ```json
  {
    "action": "created" | "existing" | "already_at",
    "branch": "feature",            // omitted if null (detached)
    "path": "/abs/worktree/path",
    "created_branch": true,          // only on action=created
    "base_branch": "main",           // only on action=created, when known
    "from_remote": "origin/feature"  // only when a tracking branch was auto-created
  }
  ```

### `wt list`

"List worktrees and their status" (`wt list --help`, v0.69.2).

- Flags: `--format <table|json>` (default `table`), `--branches` (include branches without worktrees), `--remotes` (include remote branches), `--full` (adds CI status + LLM summaries — the two columns that go over the network), `--progressive` (auto-enabled on TTY; fast local data first, remote data later). Subcommand: `wt list statusline` (single-line status for current worktree) (`wt list --help`).
- **JSON: yes, `wt list --format=json`.** Two schemas exist while the format migrates, selected by user config `[list] json-schema = 1|2`. Schema 1 is a bare array (the current default when unset, with a warning); schema 2 is an envelope object `{schema: 2, repo: {...}, collected: {...}, items: [...]}`. "A future release flips the default to schema 2 and later removes schema 1" (`wt list --help`, JSON output section). Field vocabularies (both schemas) are exhaustively documented in `wt list --help` — e.g. schema-2 items carry `branch`, `head {sha, short_sha, subject, committed_at}`, `worktree {path, main, current, previous, detached, ..., changes}`, `default_branch {ahead, behind, diff, integration, merge_conflicts}`, `upstream {remote, branch, ahead, behind}`, `pr`, `checks`, `display {state, symbols, statusline}`.
- Semantics of missing data (schema 2): **absent** = nothing to report / not requested; **null** = requested but not determined (timeout, forge fetch failure) — the JSON form of the table's `·` placeholder (`wt list --help`).
- Canonical scripting recipes from the help itself, e.g. current worktree path: `wt list --format=json | jq -r '.items[] | select(.worktree.current) | .worktree.path'` (schema 2) (`wt list --help`).
- Persistent defaults live under `[list]` in user config: `full`, `branches`, `remotes`, `summary`, `json-schema`, `columns`, `task-timeout-ms`, `timeout-ms` (`wt config --help`, User config → List; `docs/content/config.md`).

### `wt merge [TARGET]`

See section 5.

### `wt remove [BRANCHES]...`

"Remove worktree; delete branch if merged. Defaults to the current worktree" (`wt remove --help`, v0.69.2).

- Flags: `--no-delete-branch` (keep branch), `-D/--force-delete` (delete unmerged branch), `-f/--force` (remove dirty worktree), `--foreground` (block until removal completes), `--reap` (experimental; kill processes whose cwd is under the worktree, sparing interactive ones; Unix only), plus automation flags `--no-hooks` and `--format <text|json>` (`wt remove --help`).
- **Background removal by default:** "Removal runs in the background by default — the command returns immediately. The worktree is renamed into `.git/wt/trash/` (instant same-filesystem rename), git metadata is pruned, the branch is deleted, and a detached `rm -rf` finishes cleanup." Logs at `.git/wt/logs/{branch}/internal/remove.log`. Use `--foreground` to block (`wt remove --help`). Trash entries older than 24h are swept after each `wt remove` (`wt remove --help`).
- **Branch-deletion safety:** by default the branch is deleted only when it "would add no changes to the default branch if merged", checked via six ordered conditions (same commit, ancestor, empty three-dot diff, trees match, simulated merge adds nothing, patch-id match) — this covers squash/rebase-merge workflows (`wt remove --help`, "Branch cleanup"). Otherwise `-D` is required.
- **JSON output:** `--format json` prints a structured result after removal completes (`wt remove --help`). Shape from `src/commands/worktree/types.rs` (`RemoveResult::to_json`, commit `21ea6d6`): `{"kind": "worktree", "branch": ..., "path": ..., "branch_deleted": bool}` or `{"kind": "branch_only", "branch": ..., "pruned": bool, "branch_deleted": bool}`.
- Detached-HEAD worktrees have no branch name — pass the worktree path instead (`wt remove --help`).

### Shell integration / how `cd` works

`wt` is a subprocess and cannot change the parent shell's directory, so shell integration wraps it in a shell function. Installed via `wt config shell install`; manual setup is `eval "$(wt config shell init <bash|zsh|fish|nu|powershell>)"` (`wt config shell init --help`, v0.69.2). The mechanism (observed in the installed zsh wrapper function, and documented in `docs/content/config.md` env-var table):

- The wrapper creates two temp files and invokes the real binary with `WORKTRUNK_DIRECTIVE_CD_FILE` and `WORKTRUNK_DIRECTIVE_EXEC_FILE` set. "`wt` writes a raw path; the wrapper `cd`s to it" and "`wt` writes shell commands; the wrapper `source`s the file" (`docs/content/config.md`, Environment variables). The wrapper preserves the binary's exit code, then folds in the `cd`/`source` exit codes.
- "Without shell integration, `wt switch` prints the target directory but cannot `cd` into it" (`docs/content/config.md`; also `wt config --help`). For scripting, prefer `wt switch --format json` and read `.path` — it is explicitly designed for tool integration (`wt switch --help`).
- `WORKTRUNK_BIN` overrides the binary path the wrapper calls; `WORKTRUNK_SHELL` is set internally by wrappers (`docs/content/config.md`).

### Exit codes

Not documented as a stable table; from source at commit `21ea6d6`:

- `0` on success; generic errors exit `1` (`src/main.rs`: `error.exit_code().unwrap_or(1)`).
- Errors that carry a code propagate it: a failed child process (`--execute`, hooks) propagates its exit code unchanged (`src/git/error.rs` `WorktrunkError::ChildProcessExited` / `HookCommandFailed`; also `docs/content/extending.md`: for alias/exec "the child's exit code propagates unchanged").
- Signals follow the shell convention `128 + signal` (130 for SIGINT, 143 for SIGTERM) (`src/git/error.rs`, `WorktrunkError::Interrupted`; comment cites the convention explicitly).
- Practical takeaway: treat non-zero as failure, zero as success; don't assign meaning to specific non-zero values since they may be a hook's or child's code.

---

## 2. `wt switch`: new vs existing branches, PR checkout

- **Existing branch:** `wt switch feature` — if the branch already has a worktree, it changes directory to it; otherwise it *creates a worktree for the existing branch* (pre-switch hooks → create worktree at configured path → switch → pre-start hooks (blocking) → post-start/post-switch hooks in background) (`wt switch --help`, "Creating worktrees").
- **New branch:** requires `-c/--create`; "Without `--create`, the branch must already exist" (`wt switch --help`, "Creating a branch"). `--create` branches from `--base` (default: the default branch). Exception: "Switching to a remote branch (e.g., `wt switch feature` when only `origin/feature` exists) creates a local tracking branch" without `--create` (`wt switch --help`).
- **Failure modes listed in help:** branch doesn't exist (use `--create`), path occupied by another worktree, stale directory at target path (use `--clobber`) (`wt switch --help`, "When wt switch fails").
- **PR/MR checkout:** `wt switch pr:123` (GitHub), `wt switch mr:101` (GitLab), or paste the PR/MR web URL — all resolve to the PR's branch. Same-repo PRs: switches to the branch directly. Fork PRs: fetches `refs/pull/N/head` (or `refs/merge-requests/N/head`), creates a local branch under the PR's own branch name, and configures `pushRemote` to the fork URL so `git push` works normally (`wt switch --help`, "Pull requests and merge requests"). `pr:`/`mr:` also work in `--base`; `--create` cannot be combined with a PR/MR reference (`wt switch --help`). Requires `gh` (GitHub) / `glab` (GitLab) or an equivalent forge CLI, installed and authenticated (`wt switch --help`).
- Shortcuts recap: `^` default branch, `@` current, `-` previous worktree, `pr:{N}`, `mr:{N}` (`wt switch --help`).

---

## 3. Path templates and branch → path resolution

- **Config key:** `worktree-path` in user config (`~/.config/worktrunk/config.toml`), a minijinja template. **Default:** `"{{ repo_path }}/../{{ repo }}.{{ branch | sanitize }}"` — i.e. a *sibling directory* of the repo named `repo.branch-name` (e.g. `~/code/myproject.feature-auth`) (`wt config --help`, "Worktree path template"; `docs/content/config.md`).
- **Template variables** (a smaller set than hooks get): `{{ repo_path }}`, `{{ repo }}`, `{{ owner }}`, `{{ branch }}`, plus filters `sanitize` (`/`,`\` → `-`), `sanitize_db`, `codename(n)`; `~` expands to home; relative paths resolve from `repo_path` (`wt config --help`).
- Per-project override without touching the shared project file: `[projects."github.com/user/repo"] worktree-path = ...` in *user* config, keyed by `<host>/<owner>/<repo>` (`wt config --help`, "User project-specific settings").
- **Resolving branch → path from an external tool** (reliable options):
  1. `wt list --format=json` and select on `.branch` / `.worktree.path` (schema 2) — works for existing worktrees (`wt list --help`).
  2. `wt switch <branch> --format json` prints `{action, path, ...}` — resolves *and* creates if needed; combine with `--no-cd`/`--no-hooks` for side-effect control (`wt switch --help`; `src/commands/worktree/switch.rs`).
  3. `wt step eval` (experimental) evaluates a template expression, and templates support the function `worktree_path_of_branch("main")` → path or empty string (`wt step --help`; `wt hook --help`, "Worktrunk functions"). Marked experimental, so less stable than (1)/(2).
- `wt step relocate` (experimental) moves worktrees to their expected template-derived paths after a template change (`wt step --help`).

---

## 4. Lifecycle hooks

Ten hook types, paired pre/post per event (`wt hook --help`, v0.69.2):

| Event | pre- (blocking) | post- (background) |
|---|---|---|
| switch | `pre-switch` | `post-switch` |
| create | `pre-start` | `post-start` |
| commit | `pre-commit` | `post-commit` |
| merge | `pre-merge` | `post-merge` |
| remove | `pre-remove` | `post-remove` |

- **Config keys:** top-level TOML keys in project config `.config/wt.toml` or user config `~/.config/worktrunk/config.toml`; three forms — string (single command), table (multiple commands run *concurrently*), array-of-tables `[[hook]]` (pipeline of sequential steps; a failing step aborts the rest) (`wt hook --help`, "Hook forms").
- **Execution semantics:** `pre-*` hooks block and *failure aborts the operation*; `post-*` hooks run in the background with output logged (find logs via `wt config state logs`; also `.git/wt/logs/`) (`wt hook --help`). During `wt merge` the order is: pre-commit → post-commit → pre-merge (after rebase, before merge) → pre-remove → post-remove + post-merge (`wt hook --help`; `wt merge --help` "Pipeline").
- **cwd:** the worktree root of the branch being acted on (`{{ cwd }}` var), except: `pre-switch` runs in the *source* worktree; `post-remove` runs in the primary worktree (the removed worktree is gone); `post-merge` after removal runs in the *target* worktree (`wt hook --help`, "Template variables").
- **Context passing:** commands are minijinja templates with variables (`{{ branch }}`, `{{ worktree_path }}`, `{{ commit }}`, `{{ base }}`, `{{ target }}`, `{{ pr_number }}`, `{{ repo_path }}`, `{{ default_branch }}`, `{{ vars.* }}` per-branch state, etc. — full table in `wt hook --help`), with values shell-escaped automatically. Additionally, **hooks receive all template variables as JSON on stdin** (`wt hook --help`, "JSON context"). Note: context arrives via template expansion and stdin JSON, not via exported environment variables.
- **Approval gate:** *project*-defined hook commands require interactive approval on first run (and re-approval when the command text changes); approvals persist in `~/.config/worktrunk/approvals.toml`. "Use `--yes` to bypass prompts — useful for CI and automation. Use `--no-hooks` to skip hooks." Manage with `wt config approvals add|clear` (`wt hook --help`, SECURITY). User-config hooks need no approval.
- **Manual runs:** `wt hook <type>` runs hooks on demand, filterable by `user:`/`project:` and name; `--dry-run` previews; `--branch=X`/`--KEY=VALUE` override template variables; `--` forwards tokens into `{{ args }}` (`wt hook --help`, "Running hooks manually").
- **Orchestrator considerations:** all hook-firing commands accept `--no-hooks` (switch/merge/remove list it under "Automation"), so an external orchestrator can either (a) let wt run the project's hooks — then it must pass `-y/--yes` in non-interactive contexts or approvals will prompt/fail — or (b) pass `--no-hooks` and run its own setup. Post-* hooks run detached in the background, so they may still be executing after the wt command returns; execution history is auditable in `.git/wt/logs/commands.jsonl` (`docs/content/faq.md`).

---

## 5. `wt merge` in detail

"Merge current branch into the target branch. Squash & rebase, fast-forward the target branch, remove the worktree." Direction is inverted vs `git merge`: it merges the *current* branch *into* the target — "Similar to clicking 'Merge pull request' on GitHub, but locally." Target defaults to the default branch (`wt merge --help`, v0.69.2).

**Pipeline** (verbatim structure from `wt merge --help`, "Pipeline"):

1. **Commit** — pre-commit hooks run, then uncommitted changes are committed (post-commit hooks in background). Skipped when squashing (the default) — dirty changes are staged during the squash step instead.
2. **Squash** — combines all commits since target into one (like GitHub "Squash and merge"). `--stage <all|tracked|none>` controls what gets staged (default `all` = untracked + unstaged tracked). Working-tree changes swept into the squash are backed up to `refs/wt-backup/<branch>` first.
3. **Rebase** — onto target; a conflict stops the merge with the rebase left open in the worktree to resolve or abort.
4. **Pre-merge hooks** — run after rebase, before merge; failures abort.
5. **Merge** — fast-forward the target branch (`--no-ff` creates a merge commit instead; non-fast-forward merges are rejected).
6. **Pre-remove hooks** — failures abort.
7. **Cleanup** — removes the worktree and branch (`--no-remove` keeps the worktree; when already on the target branch or in the primary worktree, the worktree is preserved). After cleanup wt switches you to the target's worktree.
8. **Post-remove + post-merge hooks** — run in background.

- **Dirty-tree handling: yes, it commits uncommitted changes by default** (step 1/2 above; `--stage` and `[commit] stage` config control scope; `--no-commit` skips commit+squash and then *requires a clean working tree*) (`wt merge --help`).
- **Flags:** `--no-squash`, `--no-commit`, `--no-rebase`, `--no-ff`, `--no-remove`, `--stage <all|tracked|none>`, `--no-hooks`, `--format <text|json>` (`wt merge --help`). Config defaults under `[merge]`: `squash`, `commit`, `rebase`, `remove`, `verify`, `ff` — all `true` by default (`wt config --help`, Merge).
- **Squash vs merge:** squash is the default; `--no-squash` preserves individual commits; `--no-ff` gives rebased semi-linear history with a merge commit; `--no-commit --no-rebase` preserves the exact commit graph and requires the target to fast-forward (`wt merge --help`).
- **After success: the worktree and branch are auto-removed** (background removal, same machinery as `wt remove`) and the shell is switched to the target's worktree; `--no-remove` opts out (`wt merge --help`).
- **LLM commit messages:** commit/squash messages are generated by an external CLI configured in user config `[commit.generation] command = "..."` — the command receives the rendered prompt on stdin and returns the message on stdout. Documented example configs: Claude Code (`claude -p ...`), Codex (`codex exec ...`), OpenCode, `llm`, `aichat` (`wt config --help`, "LLM commit messages"; `docs/content/config.md`). Prompt templates (`template`, `squash-template`, `template-append`) are customizable minijinja; project config may only contribute `template-append` (approval-gated), never the command itself (`wt config --help`). The command can be overridden per-invocation via env: `WORKTRUNK_COMMIT__GENERATION__COMMAND="echo 'test: automated commit'" wt merge` — the documented CI/mock pattern (`wt config --help`, Environment variables). An editor-based non-LLM setup is documented in `docs/content/tips-patterns.md` ("Manual commit messages").
- **Local-ref semantics:** `wt merge` "targets the *local* default-branch ref and never fetches"; when the local ref lags its upstream, measuring/squash/rebase use the upstream tip; a target that has *diverged* from upstream refuses to merge (`wt merge --help`).
- **JSON output:** `--format json` prints (pretty-printed, stdout, after completion): `{"branch", "target", "committed", "squashed", "rebased", "removed"}` (`src/commands/merge.rs`, commit `21ea6d6`).
- **Building blocks:** each stage is available standalone as `wt step commit|squash|rebase|push` for manual/reviewed workflows (`wt step --help`).

---

## 6. Config files, format, and per-worktree environment

- **Format:** TOML everywhere; user options can also be set via env vars with the `WORKTRUNK_` prefix (kebab-case → SCREAMING_SNAKE_CASE, nesting via `__`, e.g. `commit.generation.command` → `WORKTRUNK_COMMIT__GENERATION__COMMAND`) and per-invocation via repeatable `--config-set <toml fragment>` (highest priority) (`wt config --help`).
- **Locations** (`wt config --help`, "Configuration files"):
  - **User config:** `~/.config/worktrunk/config.toml` (respects `$XDG_CONFIG_HOME`; Windows `%APPDATA%\worktrunk\config.toml`). Holds worktree path template, LLM commit config, `[list]`/`[merge]`/`[remove]`/`[switch]` defaults, user hooks, aliases, `[projects."host/owner/repo"]` per-project overrides. Not committed.
  - **Project config:** `.config/wt.toml` at the repo root — project hooks, dev-server URL template, `[forge]` platform, `[step.copy-ignored] exclude`, shared aliases, `[commit.generation] template-append`. Committed and shared; project commands are approval-gated.
  - **System config:** organizations can deploy a system-wide file; location shown by `wt config show` (`XDG_CONFIG_DIRS`, default `/etc/xdg`) (`wt config --help`; `docs/content/config.md`).
  - Override paths: `--config <path>`, `WORKTRUNK_CONFIG_PATH`, `WORKTRUNK_PROJECT_CONFIG_PATH`, `WORKTRUNK_SYSTEM_CONFIG_PATH` (`docs/content/config.md`).
  - Approvals: `~/.config/worktrunk/approvals.toml`; internal state/cache via `wt config state`; logs under `.git/wt/logs/` (`wt hook --help`; `docs/content/faq.md`).
- **Build caches / per-worktree environments:** worktrunk has **no shared-target-dir or cache-linking feature**. Its mechanism is *copying*: `wt step copy-ignored` copies gitignored files (build caches, `node_modules`, `.env`) from another worktree into the new one, typically wired as a `post-start` hook ("Eliminate cold starts", `docs/content/tips-patterns.md`; `wt hook --help`, "Copying untracked files"). All gitignored files are copied by default; a `.worktreeinclude` file restricts the set (files must be both gitignored and listed); built-in excludes (VCS metadata, tool-state dirs) always apply, extendable via `[step.copy-ignored] exclude` in user or project config (combined) (`docs/content/tips-patterns.md`; `wt config --help`). Per-worktree resources (ports, DB names) are supported via template filters `hash_port`, `sanitize_db`, and per-branch `vars` stored with `wt config state vars set` (`wt hook --help`).

---

## Implications for trunkr

- **Machine-readable outputs exist on every mutating command:** `wt switch --format json`, `wt list --format json`, `wt merge --format json`, `wt remove --format json`. Use these instead of parsing text output; text output is decorated (symbols, ANSI, OSC 8 hyperlinks) and clearly not a stable interface.
- **Pin the list schema.** `wt list` JSON is mid-migration (schema 1 bare array vs schema 2 envelope; default currently 1-with-warning, will flip to 2, then 1 is removed). Trunkr should force schema 2 explicitly per invocation: `wt --config-set 'list.json-schema=2' list --format=json` — this doesn't depend on the user's config. Handle absent-vs-null per the documented semantics.
- **Non-interactive invocation recipe:** pass `-y/--yes` (skip approval prompts) and decide a hook policy: `--no-hooks` to keep wt side-effect-free (trunkr orchestrates setup itself), or let hooks run and accept that `post-*` hooks continue in the background after the command returns and that project hooks may prompt for approval without `-y`. `wt switch --no-cd` skips directory-change directives (irrelevant anyway when invoking the binary directly rather than the shell wrapper).
- **Directory switching is shell-wrapper trickery** (`WORKTRUNK_DIRECTIVE_CD_FILE` / `WORKTRUNK_DIRECTIVE_EXEC_FILE` temp files sourced by a `wt` shell function). An external tool calling the `wt` binary is unaffected — it just won't get `cd` behavior; read the path from `--format json` instead.
- **Branch → path resolution:** prefer `wt list --format=json` (existing worktrees) or `wt switch <branch> --format json` (resolve-and-create). Don't re-implement the `worktree-path` template — it's user-configurable (user config, per-project overrides, env vars), so computed paths must always come from wt itself.
- **Exit codes are boolean in practice:** 0 success, nonzero failure; specific values may be a propagated child/hook exit code or `128+signal`. Don't script against specific nonzero values.
- **`wt merge` commits dirty trees by default and auto-removes the worktree+branch** (in the background). For conservative orchestration use `--no-remove`, `--stage none`/`--no-commit`, or run the `wt step` building blocks individually. LLM commit generation can be stubbed deterministically via `WORKTRUNK_COMMIT__GENERATION__COMMAND`.
- **Removal is asynchronous by default** — `wt remove` returns before the directory is gone (renamed into `.git/wt/trash/` immediately). If trunkr needs the path fully gone (e.g. to reuse it), pass `--foreground`.
- **Watch for interference, not conflict:** worktrunk's hooks are its own TOML config (`.config/wt.toml`), not git hooks; `pre-*` hook failures abort switch/merge/remove with the hook's exit code, which trunkr must surface. Background hook/removal activity is logged under `.git/wt/logs/` (including `commands.jsonl` audit log) for debugging.
