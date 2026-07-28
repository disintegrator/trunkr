// Package wt invokes the worktrunk CLI and parses its JSON output.
//
// Every invocation runs with an explicit working directory — the target
// repo/workspace, never the plugin root. Actions run with cwd = plugin root,
// where the shipped mise.toml is untrusted and breaks mise-shimmed wt, so
// the client refuses to run without a directory. Worktree paths are
// user-templated, so they are always resolved from wt's own JSON output,
// never computed.
package wt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// InstallHint is appended to errors when the wt binary cannot be found.
const InstallHint = "worktrunk (wt) is required — install it from https://github.com/max-sixty/worktrunk"

// ErrNotFound reports that the wt binary could not be located.
var ErrNotFound = errors.New(InstallHint)

// Client invokes a specific wt binary.
type Client struct {
	// Path is the wt binary's location.
	Path string
}

// New locates the wt binary. An explicit path (the wt_path config knob) wins;
// empty means PATH lookup. A missing binary returns ErrNotFound with the
// install hint.
func New(explicitPath string) (*Client, error) {
	if explicitPath != "" {
		path, err := exec.LookPath(explicitPath)
		if err != nil {
			return nil, fmt.Errorf("wt_path %q: %w: %s", explicitPath, err, InstallHint)
		}
		return &Client{Path: path}, nil
	}
	path, err := exec.LookPath("wt")
	if err != nil {
		return nil, fmt.Errorf("wt not found on PATH: %w", ErrNotFound)
	}
	return &Client{Path: path}, nil
}

// Command builds an exec.Cmd for a wt invocation with cwd pinned to dir.
// Action slices that stream output to a runner surface (merge) use this
// directly; JSON-parsing helpers go through run.
func (c *Client) Command(ctx context.Context, dir string, args ...string) (*exec.Cmd, error) {
	if dir == "" {
		return nil, errors.New("wt invocation requires an explicit working directory (never the plugin root)")
	}
	cmd := exec.CommandContext(ctx, c.Path, args...)
	cmd.Dir = dir
	return cmd, nil
}

// run executes wt in dir and returns its stdout. On failure the error carries
// wt's stderr; exit codes are effectively boolean, so no code is surfaced.
func (c *Client) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd, err := c.Command(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("wt %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}
