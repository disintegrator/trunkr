package wt

import (
	"context"
	"encoding/json"
	"fmt"
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

// Switch runs wt switch for ref — a branch name, pr:N / mr:N shortcut, or PR
// URL — creating the worktree if needed, and returns where it lives. --no-cd
// is always passed: trunkr opens panes at the returned path instead of
// relying on wt's shell-wrapper directory change.
func (c *Client) Switch(ctx context.Context, dir, ref string, opts SwitchOptions) (SwitchResult, error) {
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
	out, err := c.run(ctx, dir, args...)
	if err != nil {
		return SwitchResult{}, err
	}
	var res SwitchResult
	if err := json.Unmarshal(out, &res); err != nil {
		return SwitchResult{}, fmt.Errorf("wt switch: parsing JSON output: %w", err)
	}
	if res.Path == "" {
		return SwitchResult{}, fmt.Errorf("wt switch: JSON output has no path")
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

// Remove runs wt remove for the given branches (empty means the current
// worktree) and returns what was removed.
func (c *Client) Remove(ctx context.Context, dir string, branches []string, opts RemoveOptions) ([]RemoveResult, error) {
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
	out, err := c.run(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	var res []RemoveResult
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("wt remove: parsing JSON output: %w", err)
	}
	return res, nil
}
