package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"github.com/disintegrator/trunkr/internal/config"
	"github.com/disintegrator/trunkr/internal/herdr"
	"github.com/disintegrator/trunkr/internal/mapping"
	"github.com/disintegrator/trunkr/internal/wt"
)

// runnerCommand runs inside the popup [[panes]] surface. It is the visible
// runner from the action feedback conventions: wt runs attached to this
// terminal so hook-approval prompts work, the popup auto-closes on success
// (the process exits), and on failure it holds open and fires a notification.
func runnerCommand() *cli.Command {
	return &cli.Command{
		Name:  "runner",
		Usage: "run a worktree operation inside the popup surface (opened by trunkr actions)",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runRunner(ctx)
		},
	}
}

// runnerRequest is the TRUNKR_* contract set by the action that opened the
// popup (see action.go).
type runnerRequest struct {
	Op          string
	Mode        string
	Ref         string
	Dir         string
	WorkspaceID string
	PaneID      string
}

func requestFromEnv() runnerRequest {
	return runnerRequest{
		Op:          os.Getenv(envOp),
		Mode:        os.Getenv(envMode),
		Ref:         os.Getenv(envRef),
		Dir:         os.Getenv(envDir),
		WorkspaceID: os.Getenv(envWorkspaceID),
		PaneID:      os.Getenv(envPaneID),
	}
}

// opTitle names an op in notifications and prompts.
func opTitle(op string) string {
	switch op {
	case "create":
		return "create"
	case "pr":
		return "PR checkout"
	default:
		return "switch"
	}
}

func runRunner(ctx context.Context) error {
	req := requestFromEnv()
	title := opTitle(req.Op)

	hc, err := herdr.FromEnv()
	if err != nil {
		return failHold(err)
	}
	fail := func(err error) error {
		hc.Notify(ctx, "trunkr: "+title+" failed", gist(err.Error()))
		return failHold(err)
	}

	if req.Dir == "" {
		return fail(errors.New(envDir + " is not set — the runner popup is opened by trunkr actions"))
	}
	cfg, err := loadConfig(req.Dir)
	if err != nil {
		return fail(err)
	}
	wtc, err := wt.New(cfg.WTPath)
	if err != nil {
		return fail(err)
	}

	ref := req.Ref
	if ref == "" {
		label := "branch"
		switch req.Op {
		case "create":
			label = "new branch name"
		case "pr":
			label = "PR number or URL"
		}
		ref, err = promptLine(label)
		if err != nil {
			return fail(err)
		}
		if ref == "" {
			fmt.Fprintln(os.Stderr, "cancelled")
			return nil
		}
	}

	opts := wt.SwitchOptions{}
	switch req.Op {
	case "create":
		opts.Create = true
	case "pr":
		ref, err = wt.PRRef(ref)
		if err != nil {
			return fail(err)
		}
	}

	if opts.Create {
		fmt.Fprintf(os.Stderr, "→ wt switch --create %s\n", ref)
	} else {
		fmt.Fprintf(os.Stderr, "→ wt switch %s\n", ref)
	}
	// Tee wt's streamed output so the failure notification can carry its
	// last line as the gist.
	var wtOut bytes.Buffer
	res, err := wtc.SwitchStreaming(ctx, req.Dir, ref, opts, os.Stdin, io.MultiWriter(os.Stderr, &wtOut))
	if err != nil {
		if tail := gist(wtOut.String()); tail != "" {
			hc.Notify(ctx, "trunkr: "+title+" failed", tail)
			return failHold(err)
		}
		return fail(err)
	}

	if err := openOrFocus(ctx, hc, wtc, cfg, req, res); err != nil {
		return fail(err)
	}
	return nil
}

// openOrFocus lands the user in the switched worktree per the pane-mapping
// design: the generic switch focuses the worktree's existing container when
// it has live panes; otherwise (and always for explicit per-mode requests) a
// new pane opens in the chosen container, running the configured agent
// command.
func openOrFocus(ctx context.Context, hc *herdr.Client, wtc *wt.Client, cfg config.Config, req runnerRequest, res wt.SwitchResult) error {
	panes, err := hc.PaneList(ctx)
	if err != nil {
		return err
	}
	paths := []string{res.Path}
	// All worktree paths make the cwd match boundary-aware; on error the
	// target path alone still gives a correct (if coarser) answer.
	if list, err := wtc.List(ctx, req.Dir); err == nil {
		for _, w := range list.Worktrees {
			paths = append(paths, w.Path)
		}
	}
	existing := mapping.PanesIn(panes, paths, res.Path)

	focusExisting, container := decideOpen(req.Mode, cfg.Container, len(existing))
	if focusExisting {
		target := existing[0]
		if target.WorkspaceID != "" {
			if err := hc.WorkspaceFocus(ctx, target.WorkspaceID); err != nil {
				return err
			}
		}
		if target.TabID != "" {
			return hc.TabFocus(ctx, target.TabID)
		}
		return nil
	}

	// Containers are labeled with the bare branch name.
	label := res.Branch
	if label == "" {
		label = filepath.Base(res.Path)
	}
	var pane herdr.Pane
	switch container {
	case config.ContainerWorkspace:
		pane, err = hc.WorkspaceCreate(ctx, res.Path, label)
	case config.ContainerSplit:
		pane, err = hc.PaneSplit(ctx, req.PaneID, res.Path)
	default:
		pane, err = hc.TabCreate(ctx, req.WorkspaceID, res.Path, label)
	}
	if err != nil {
		return err
	}
	if len(cfg.AgentCommand) > 0 && pane.PaneID != "" {
		return hc.PaneRun(ctx, pane.PaneID, cfg.AgentCommand)
	}
	return nil
}

// decideOpen resolves the post-switch plan. An explicit mode always opens a
// new pane in that container; the generic action focuses the worktree's
// existing panes when there are any, else opens in the configured container.
func decideOpen(mode string, fallback config.Container, existingPanes int) (focusExisting bool, container config.Container) {
	switch mode {
	case "tab":
		return false, config.ContainerTab
	case "workspace":
		return false, config.ContainerWorkspace
	case "split":
		return false, config.ContainerSplit
	}
	if existingPanes > 0 {
		return true, ""
	}
	return false, fallback
}

// loadConfig resolves trunkr config for dir, prompting in the popup terminal
// to approve gated repo config files — the once-per-repo approval from the
// configuration design.
func loadConfig(dir string) (config.Config, error) {
	res, err := config.Load(config.SourceFromEnv(dir))
	if err != nil {
		return config.Config{}, err
	}
	if len(res.Pending) == 0 {
		return res.Config, nil
	}
	fmt.Fprintln(os.Stderr, "this repository has trunkr config awaiting approval:")
	for _, p := range res.Pending {
		fmt.Fprintf(os.Stderr, "\n--- %s ---\n%s\n\n", p.Path, strings.TrimRight(p.Content, "\n"))
	}
	answer, err := promptLine("apply this repo's trunkr config? [y/N]")
	if err != nil {
		return config.Config{}, err
	}
	if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		fmt.Fprintln(os.Stderr, "continuing without repo config (you will be asked again next time)")
		return res.Config, nil
	}
	if err := config.Approve(os.Getenv("HERDR_PLUGIN_STATE_DIR"), res.RepoID); err != nil {
		return config.Config{}, err
	}
	res, err = config.Load(config.SourceFromEnv(dir))
	if err != nil {
		return config.Config{}, err
	}
	return res.Config, nil
}

// stdinReader is shared across prompts so buffered type-ahead isn't lost
// between consecutive reads.
var stdinReader = bufio.NewReader(os.Stdin)

// promptLine asks for one line of input in the popup terminal. An empty
// answer means the user cancelled.
func promptLine(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	line, err := stdinReader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// failHold prints the error and keeps the popup open until a keypress so the
// output stays readable; the popup closes when the process exits.
func failHold(err error) error {
	fmt.Fprintln(os.Stderr, "\ntrunkr: "+err.Error())
	fmt.Fprint(os.Stderr, "\npress any key to close ")
	fd := int(os.Stdin.Fd())
	if oldState, rawErr := term.MakeRaw(fd); rawErr == nil {
		defer term.Restore(fd, oldState)
	}
	buf := make([]byte, 1)
	os.Stdin.Read(buf)
	return err
}

// gist condenses (possibly multi-line) output to a one-line notification
// body: the last non-empty line, truncated.
func gist(s string) string {
	last := ""
	for line := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			last = t
		}
	}
	if len(last) > 120 {
		last = last[:117] + "..."
	}
	return last
}
