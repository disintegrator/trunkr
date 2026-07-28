package wt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// SwitchAction is what wt switch did to satisfy the request.
type SwitchAction string

const (
	SwitchCreated   SwitchAction = "created"    // worktree (and possibly branch) created
	SwitchExisting  SwitchAction = "existing"   // worktree already existed
	SwitchAlreadyAt SwitchAction = "already_at" // caller is already in it
)

// SwitchResult is wt switch --format json output. Optional fields are
// omitted by wt when not applicable.
type SwitchResult struct {
	Action SwitchAction `json:"action"`
	// Branch is omitted for a detached-HEAD switch.
	Branch string `json:"branch"`
	// Path is the worktree's location — the only trustworthy source for
	// it, since worktree paths are user-templated.
	Path string `json:"path"`
	// CreatedBranch is set only on action=created.
	CreatedBranch bool `json:"created_branch"`
	// BaseBranch is set only on action=created, when known.
	BaseBranch string `json:"base_branch"`
	// FromRemote is set when a tracking branch was auto-created.
	FromRemote string `json:"from_remote"`
}

// SwitchOptions are the wt switch flags trunkr uses.
type SwitchOptions struct {
	// Create passes -c: required when the branch does not exist yet.
	Create bool
	// Base passes -b <base> for created branches.
	Base string
	// Yes passes -y, skipping wt's approval prompts (e.g. project hooks).
	// Leave false when running in a surface where the user can answer.
	Yes bool
	// NoHooks passes --no-hooks, skipping wt's lifecycle hooks entirely.
	NoHooks bool
}

// switchArgs builds the wt switch argv. --no-cd is always passed: trunkr
// opens panes at the returned path instead of relying on wt's shell-wrapper
// directory change.
func switchArgs(ref string, opts SwitchOptions) []string {
	args := []string{"switch", ref, "--no-cd", "--format", "json"}
	if opts.Create {
		args = append(args, "--create")
	}
	if opts.Base != "" {
		args = append(args, "--base", opts.Base)
	}
	if opts.Yes {
		args = append(args, "--yes")
	}
	if opts.NoHooks {
		args = append(args, "--no-hooks")
	}
	return args
}

func parseSwitchResult(out []byte) (SwitchResult, error) {
	var res SwitchResult
	if err := json.Unmarshal(out, &res); err != nil {
		return SwitchResult{}, fmt.Errorf("wt switch: parsing JSON output: %w", err)
	}
	if res.Path == "" {
		return SwitchResult{}, fmt.Errorf("wt switch: JSON output has no path")
	}
	return res, nil
}

// Switch runs wt switch for ref — a branch name, pr:N / mr:N shortcut, or PR
// URL — creating the worktree if needed, and returns where it lives.
func (c *Client) Switch(ctx context.Context, dir, ref string, opts SwitchOptions) (SwitchResult, error) {
	out, err := c.run(ctx, dir, switchArgs(ref, opts)...)
	if err != nil {
		return SwitchResult{}, err
	}
	return parseSwitchResult(out)
}

// SwitchStreaming runs wt switch attached to the caller's terminal: stdin and
// stderr pass through so wt's hook-approval prompts work interactively, while
// stdout (the JSON result) is captured and parsed. This is the runner-surface
// variant of Switch.
func (c *Client) SwitchStreaming(ctx context.Context, dir, ref string, opts SwitchOptions, stdin io.Reader, stderr io.Writer) (SwitchResult, error) {
	cmd, err := c.Command(ctx, dir, switchArgs(ref, opts)...)
	if err != nil {
		return SwitchResult{}, err
	}
	var stdout bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, &stdout, stderr
	if err := cmd.Run(); err != nil {
		return SwitchResult{}, fmt.Errorf("wt switch %s: %w", ref, err)
	}
	return parseSwitchResult(stdout.Bytes())
}

// MergeResult is wt merge --format json output.
type MergeResult struct {
	// Branch is the branch that was merged (the worktree's own branch — wt
	// merge merges the current branch into the target).
	Branch string `json:"branch"`
	// Target is the branch merged into, the default branch unless overridden.
	Target    string `json:"target"`
	Committed bool   `json:"committed"`
	Squashed  bool   `json:"squashed"`
	Rebased   bool   `json:"rebased"`
	Removed   bool   `json:"removed"`
}

// MergeOptions are the wt merge flags trunkr uses.
type MergeOptions struct {
	// NoRemove passes --no-remove, keeping the worktree so trunkr can run its
	// controlled teardown (close panes, then remove) instead of wt's
	// background removal.
	NoRemove bool
	// ExtraArgs are the user's [merge] extra_args, appended to every wt merge
	// invocation.
	ExtraArgs []string
}

// mergeArgs builds the wt merge argv. Extra args come before trunkr's own
// flags so the trunkr contract (--no-remove, --format json) wins if both name
// the same flag.
func mergeArgs(opts MergeOptions) []string {
	args := []string{"merge"}
	args = append(args, opts.ExtraArgs...)
	if opts.NoRemove {
		args = append(args, "--no-remove")
	}
	args = append(args, "--format", "json")
	return args
}

// MergeStreaming runs wt merge in the worktree at dir — wt merge merges the
// *current* branch into the target, so dir selects what gets merged. Stdin
// and stderr pass through so hook-approval prompts work interactively; stdout
// (the JSON result) is captured and parsed.
func (c *Client) MergeStreaming(ctx context.Context, dir string, opts MergeOptions, stdin io.Reader, stderr io.Writer) (MergeResult, error) {
	cmd, err := c.Command(ctx, dir, mergeArgs(opts)...)
	if err != nil {
		return MergeResult{}, err
	}
	var stdout bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, &stdout, stderr
	if err := cmd.Run(); err != nil {
		return MergeResult{}, fmt.Errorf("wt merge: %w", err)
	}
	var res MergeResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return MergeResult{}, fmt.Errorf("wt merge: parsing JSON output: %w", err)
	}
	return res, nil
}

// RemoveResult is one entry of wt remove --format json output.
type RemoveResult struct {
	Branch        string `json:"branch"`
	Path          string `json:"path"`
	BranchDeleted bool   `json:"branch_deleted"`
}

// RemoveOptions are the wt remove flags trunkr uses. There is deliberately
// no force-delete: destroy uses -f (dirty worktree) but never -D (unmerged
// branch), per the action feedback conventions.
type RemoveOptions struct {
	// Force passes -f, removing a dirty worktree.
	Force bool
	// Foreground passes --foreground, blocking until the path is actually
	// gone instead of trashing it in the background.
	Foreground bool
	// Yes passes -y, skipping wt's approval prompts.
	Yes bool
	// NoHooks passes --no-hooks.
	NoHooks bool
}

func removeArgs(branches []string, opts RemoveOptions) []string {
	args := []string{"remove"}
	args = append(args, branches...)
	args = append(args, "--format", "json")
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Foreground {
		args = append(args, "--foreground")
	}
	if opts.Yes {
		args = append(args, "--yes")
	}
	if opts.NoHooks {
		args = append(args, "--no-hooks")
	}
	return args
}

func parseRemoveResults(out []byte) ([]RemoveResult, error) {
	var res []RemoveResult
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("wt remove: parsing JSON output: %w", err)
	}
	return res, nil
}

// Remove runs wt remove for the given branches (empty means the current
// worktree) and returns what was removed.
func (c *Client) Remove(ctx context.Context, dir string, branches []string, opts RemoveOptions) ([]RemoveResult, error) {
	out, err := c.run(ctx, dir, removeArgs(branches, opts)...)
	if err != nil {
		return nil, err
	}
	return parseRemoveResults(out)
}

// RemoveStreaming is the runner-surface variant of Remove: stdin and stderr
// pass through so remove-hook approval prompts work interactively, while
// stdout (the JSON result) is captured and parsed.
func (c *Client) RemoveStreaming(ctx context.Context, dir string, branches []string, opts RemoveOptions, stdin io.Reader, stderr io.Writer) ([]RemoveResult, error) {
	cmd, err := c.Command(ctx, dir, removeArgs(branches, opts)...)
	if err != nil {
		return nil, err
	}
	var stdout bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, &stdout, stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("wt remove: %w", err)
	}
	return parseRemoveResults(stdout.Bytes())
}
