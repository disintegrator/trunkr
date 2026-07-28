---
name: pull-request
description: >
  Guide the full workflow of creating a pull request for this repo: surveying
  the changes, recording a bumper version bump, writing conventional commits,
  pushing, and opening the PR with gh. Use this skill whenever the user asks to
  create, open, or prepare a pull request, or says things like "ship this",
  "push this up", "PR this", "get this merged", or otherwise wants finished
  work turned into a PR — even if they don't say "pull request" explicitly.
---

# Creating a pull request

Every PR in this repo carries two artifacts besides the code: a **bumper bump
file** (which drives versioning and release notes) and a **conventional-commit
history**. Preparing a PR means getting all three right, then pushing and
opening the PR with `gh`.

## 1. Survey the change

Build a picture of what's actually shipping before writing anything:

- `git status` — uncommitted work that belongs in this PR.
- `git log --oneline main..HEAD` and `git diff main...HEAD --stat` — what the
  branch already contains. (Adjust if the default branch differs.)
- If you're on `main`, create a branch first: `<type>/<short-slug>`, e.g.
  `feat/worktree-picker` or `fix/pane-cleanup`, using the same type you'll use
  for the commit.

Read the diff well enough to describe the change from a *user's* perspective —
that framing drives the bump level, the changelog entry, and the PR
description.

## 2. Record the version bump

This project uses [bumper](https://bumper.disintegrator.dev/) to manage
versions and changelogs. Each PR includes a bump file recording how it should
move the version; the release process (`bumper commit`) consumes these files
later — never run `bumper commit` as part of a PR.

First check whether the branch already has a bump file
(`git diff main...HEAD --name-only -- .bumper/`). If one exists, update it
rather than adding a second — one bump file per PR keeps the changelog one
entry per change.

Read `.bumper/config.toml` for the release groups (currently just `trunkr`).
Decide the level from the change's impact on users of the plugin:

| The change... | Level |
| --- | --- |
| breaks existing behavior, config, or CLI usage | `--major` |
| adds a capability users can see or use | `--minor` |
| fixes or tweaks behavior without adding anything | `--patch` |
| touches only docs, CI, tests, or internals users never see | `--empty` |

Then record it, without asking for confirmation — just state the choice and
reasoning in your summary:

```sh
bumper bump --group trunkr --minor -m "Add a worktree picker pane"
# or, for changes with no user-facing impact:
bumper bump --empty
```

The `-m` message becomes a **changelog entry read by users in release notes**,
not a commit message. Describe the outcome from their point of view ("Add
...", "Fix ...", "Rename ... to ..."), and skip internal details like file
names or refactoring steps. Markdown is fine for longer entries.

This creates `.bumper/bump-<slug>.md` — commit it with the PR.

## 3. Commit

Use conventional commits: `type(scope): summary` with types `feat`, `fix`,
`docs`, `refactor`, `test`, `chore`, `ci`, `perf`; add `!` for breaking
changes (which should match a `--major` bump). Scope is optional — use it when
the change is clearly localized.

Keep the subject imperative and under ~72 characters. If the work is still
uncommitted, stage and commit it together with the bump file; if the branch
already has good commits, just add the bump file (`chore: add version bump` or
amend if the last commit is yours and unpushed).

Run the project's checks before pushing if any exist (see `mise tasks`); a PR
that fails CI immediately wastes a round-trip.

## 4. Push and open the PR

```sh
git push -u origin <branch>
gh pr create --title "<conventional title>" --body "<body>"
```

The PR title follows the same conventional format as the commit (for a
single-commit PR, reuse the commit subject). Structure the body as:

```markdown
## Summary
One or two sentences: what changed and why.

## Changes
- Bullet per meaningful change (not per file).

## Version
`trunkr` <level> bump — or "no version impact" for empty bumps.

## Testing
How the change was verified (tests run, manual checks), or "not tested" honestly.
```

Finish by reporting the PR URL, the bump level you chose and why, and anything
you noticed that the user should double-check.
