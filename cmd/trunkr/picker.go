package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v3"

	"github.com/disintegrator/trunkr/internal/config"
	"github.com/disintegrator/trunkr/internal/herdr"
	"github.com/disintegrator/trunkr/internal/picker"
	"github.com/disintegrator/trunkr/internal/wt"
)

// pickerRefreshInterval paces the live rolled-up agent status; both backing
// calls (wt list, herdr pane list) are cheap local queries.
const pickerRefreshInterval = 2 * time.Second

// pickerEntrypoint is the [[panes]] id of the picker overlay.
const pickerEntrypoint = "picker"

// pickerPaneCommand runs inside the overlay [[panes]] surface: the
// interactive worktree picker. Actions chosen in it re-invoke this binary's
// runner via tea.ExecProcess, in-overlay, so hook prompts and failure
// hold-open work exactly as they do in the popup runner.
func pickerPaneCommand() *cli.Command {
	return &cli.Command{
		Name:  "picker",
		Usage: "interactive worktree picker (runs inside the picker overlay pane)",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runPicker(ctx)
		},
	}
}

func runPicker(ctx context.Context) error {
	dir := os.Getenv(envDir)
	if dir == "" {
		// Opened directly rather than via the picker action: fall back to
		// the pane's own invocation context.
		ictx, err := herdr.ContextFromEnv()
		if err != nil {
			return err
		}
		dir = ictx.TargetDir()
	}
	if dir == "" {
		return errors.New("no target directory: open the picker from a workspace or pane with a working directory")
	}
	hc, err := herdr.FromEnv()
	if err != nil {
		return err
	}
	// Gated repo config awaiting approval is skipped here; the runner
	// prompts for approval when the first action runs.
	res, err := config.Load(config.SourceFromEnv(dir))
	if err != nil {
		return err
	}
	cfg := res.Config
	wtc, err := wt.New(cfg.WTPath)
	if err != nil {
		return err
	}

	refresh := func() tea.Msg {
		list, err := wtc.List(ctx, dir)
		if err != nil {
			return picker.RefreshedMsg{Err: err}
		}
		// Re-anchor on the trunk worktree: dir may start inside a feature
		// worktree that a merge later deletes, and the trunk path outlives
		// every worktree operation the picker can trigger.
		for _, w := range list.Worktrees {
			if w.IsMain && w.Path != "" {
				dir = w.Path
				break
			}
		}
		panes, err := hc.PaneList(ctx)
		if err != nil {
			return picker.RefreshedMsg{Err: err}
		}
		return picker.RefreshedMsg{Rows: picker.BuildRows(list, panes)}
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving own binary: %w", err)
	}
	execAction := func(a picker.Action) tea.Cmd {
		child := exec.CommandContext(ctx, self, "runner")
		// The picker's inline y/N confirm already ran for merge, so the
		// runner must not re-confirm.
		confirmed := ""
		if a.Op == "merge" {
			confirmed = "1"
		}
		// Later entries win: override the picker pane's own TRUNKR_* vars
		// wholesale so no stale value leaks into the runner.
		child.Env = append(os.Environ(),
			envOp+"="+a.Op,
			envMode+"="+a.Mode,
			envRef+"="+a.Ref,
			envDir+"="+dir,
			envConfirmed+"="+confirmed,
		)
		return tea.ExecProcess(child, func(err error) tea.Msg {
			// Switch-family actions end in a focus change the overlay would
			// sit on top of, so success closes the picker. A merged worktree
			// just leaves the list — the picker stays open and refreshes.
			quit := err == nil && a.Op != "merge"
			return picker.ActionDoneMsg{Err: err, Quit: quit}
		})
	}

	m := picker.New(cfg.PickerKeys, execAction, refresh, pickerRefreshInterval)
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// openPicker is the headless picker action: resolve the target directory and
// open the picker overlay with the same env contract the runner uses.
func openPicker(ctx context.Context) error {
	hc, err := herdr.FromEnv()
	if err != nil {
		return err
	}
	ictx, err := herdr.ContextFromEnv()
	if err != nil {
		return err
	}
	dir := ictx.TargetDir()
	if dir == "" {
		msg := "no target directory: invoke the picker from a workspace or pane with a working directory"
		hc.Notify(ctx, "trunkr: cannot open picker", msg)
		return errors.New(msg)
	}
	env := map[string]string{envDir: dir}
	if ws := os.Getenv("HERDR_WORKSPACE_ID"); ws != "" {
		env[envWorkspaceID] = ws
	}
	// The covered pane, so a split from the picker lands beside it rather
	// than beside the overlay.
	if pane := ictx.FocusedPaneID; pane != "" {
		env[envPaneID] = pane
	}
	if err := hc.PluginPaneOpen(ctx, pickerEntrypoint, env); err != nil {
		hc.Notify(ctx, "trunkr: cannot open picker", err.Error())
		return err
	}
	return nil
}
