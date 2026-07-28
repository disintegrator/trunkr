package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/disintegrator/trunkr/internal/herdr"
)

// Environment contract between the headless action commands and the runner
// popup they open. Actions are the only writer; the runner is the only
// reader. The picker pane will reuse the same contract.
const (
	envOp          = "TRUNKR_OP"           // switch | create | pr | merge | destroy
	envMode        = "TRUNKR_MODE"         // container override: tab | workspace | split (generic when unset)
	envRef         = "TRUNKR_REF"          // prefilled ref; the runner prompts when unset
	envDir         = "TRUNKR_DIR"          // target repo/workspace directory
	envWorkspaceID = "TRUNKR_WORKSPACE_ID" // workspace the action was invoked from
	envPaneID      = "TRUNKR_PANE_ID"      // pane the action was invoked from (split target)
	envConfirmed   = "TRUNKR_CONFIRMED"    // "1" when the picker's inline confirm already ran
)

// runnerEntrypoint is the [[panes]] id of the popup runner surface.
const runnerEntrypoint = "runner"

// actionCommand is the [[actions]] entrypoint family. Action commands are
// headless — their output lands in the plugin command log — so each one just
// resolves the target directory and opens the interactive runner popup,
// which does the real work where hooks can prompt.
func actionCommand() *cli.Command {
	modeFlag := &cli.StringFlag{
		Name:  "mode",
		Usage: "container for the new pane: tab, workspace, or split (always opens a new pane)",
	}
	return &cli.Command{
		Name:  "action",
		Usage: "open the runner popup for a worktree operation",
		Commands: []*cli.Command{
			{
				Name:      "switch",
				Usage:     "switch to a worktree, opening or focusing its panes",
				Flags:     []cli.Flag{modeFlag},
				ArgsUsage: "[ref]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return openRunner(ctx, "switch", cmd.String("mode"), cmd.Args().First())
				},
			},
			{
				Name:      "create",
				Usage:     "create a worktree for a new branch",
				ArgsUsage: "[branch]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return openRunner(ctx, "create", "", cmd.Args().First())
				},
			},
			{
				Name:      "pr",
				Usage:     "check out a pull request into a worktree",
				ArgsUsage: "[number|pr:N|url]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return openRunner(ctx, "pr", "", cmd.Args().First())
				},
			},
			{
				Name:      "merge",
				Usage:     "merge a worktree's branch into trunk, close its panes, and remove it",
				ArgsUsage: "[branch]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return openRunner(ctx, "merge", "", cmd.Args().First())
				},
			},
			{
				Name:      "destroy",
				Usage:     "destroy a worktree: discard changes, close its panes, remove it",
				ArgsUsage: "[branch]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return openRunner(ctx, "destroy", "", cmd.Args().First())
				},
			},
			{
				Name:  "picker",
				Usage: "open the interactive worktree picker overlay",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return openPicker(ctx)
				},
			},
		},
	}
}

func openRunner(ctx context.Context, op, mode, ref string) error {
	switch mode {
	case "", "tab", "workspace", "split":
	default:
		return fmt.Errorf("invalid --mode %q: must be tab, workspace, or split", mode)
	}
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
		msg := "no target directory: invoke the action from a workspace or pane with a working directory"
		hc.Notify(ctx, "trunkr: cannot run "+op, msg)
		return errors.New(msg)
	}

	env := map[string]string{envOp: op, envDir: dir}
	if mode != "" {
		env[envMode] = mode
	}
	if ref != "" {
		env[envRef] = ref
	}
	if ws := os.Getenv("HERDR_WORKSPACE_ID"); ws != "" {
		env[envWorkspaceID] = ws
	}
	if pane := os.Getenv("HERDR_PANE_ID"); pane != "" {
		env[envPaneID] = pane
	}
	if err := hc.PluginPaneOpen(ctx, runnerEntrypoint, env); err != nil {
		hc.Notify(ctx, "trunkr: cannot run "+op, err.Error())
		return err
	}
	return nil
}
