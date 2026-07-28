// Package herdr calls back into the running herdr server through its CLI —
// the plugin API is the herdr binary itself, reached via HERDR_BIN_PATH.
// Every CLI helper prints a JSON response envelope; this package parses it
// and exposes just the operations trunkr's action slices need.
package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Pane is herdr's view of one live pane, trimmed to what trunkr consumes.
// Cwd is the shell's directory; ForegroundCwd is the foreground process's,
// which may differ when an agent cd's around.
type Pane struct {
	PaneID        string `json:"pane_id"`
	WorkspaceID   string `json:"workspace_id"`
	TabID         string `json:"tab_id"`
	Focused       bool   `json:"focused"`
	AgentStatus   string `json:"agent_status"`
	Cwd           string `json:"cwd"`
	ForegroundCwd string `json:"foreground_cwd"`
	Label         string `json:"label"`
	Agent         string `json:"agent"`
}

// Client invokes a specific herdr binary. The socket routing comes from the
// inherited plugin environment, so no explicit session wiring is needed.
type Client struct {
	Bin      string
	PluginID string
}

// FromEnv builds a Client from the herdr plugin environment.
func FromEnv() (*Client, error) {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		return nil, errors.New("HERDR_BIN_PATH is not set; trunkr must run as a herdr plugin command")
	}
	return &Client{Bin: bin, PluginID: os.Getenv("HERDR_PLUGIN_ID")}, nil
}

// envelope is the JSON response every herdr CLI helper prints.
type envelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// run executes herdr with args and returns the envelope's result. An error
// envelope wins over the exit code; otherwise a failed run carries stderr.
func (c *Client) run(ctx context.Context, args ...string) (json.RawMessage, error) {
	cmd := exec.CommandContext(ctx, c.Bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	out := bytes.TrimSpace(stdout.Bytes())
	if len(out) > 0 && out[0] == '{' {
		var env envelope
		if err := json.Unmarshal(out, &env); err == nil {
			if env.Error != nil {
				return nil, fmt.Errorf("herdr %s: %s (%s)", strings.Join(args, " "), env.Error.Message, env.Error.Code)
			}
			if runErr == nil {
				return env.Result, nil
			}
		}
	}
	if runErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		return nil, fmt.Errorf("herdr %s: %s", strings.Join(args, " "), msg)
	}
	return nil, nil
}

// decode unmarshals a result payload, failing loudly on an empty result so
// shape drift surfaces as an error instead of zero values.
func decode(res json.RawMessage, v any, what string) error {
	if len(res) == 0 {
		return fmt.Errorf("herdr %s: empty result", what)
	}
	if err := json.Unmarshal(res, v); err != nil {
		return fmt.Errorf("herdr %s: %w", what, err)
	}
	return nil
}

// PaneList returns every live pane in the session.
func (c *Client) PaneList(ctx context.Context) ([]Pane, error) {
	res, err := c.run(ctx, "pane", "list")
	if err != nil {
		return nil, err
	}
	var out struct {
		Panes []Pane `json:"panes"`
	}
	if err := decode(res, &out, "pane list"); err != nil {
		return nil, err
	}
	return out.Panes, nil
}

// TabCreate opens and focuses a new tab at cwd and returns its root pane.
// workspaceID may be empty (herdr uses the focused workspace).
func (c *Client) TabCreate(ctx context.Context, workspaceID, cwd, label string) (Pane, error) {
	args := []string{"tab", "create", "--cwd", cwd, "--focus"}
	if workspaceID != "" {
		args = append(args, "--workspace", workspaceID)
	}
	if label != "" {
		args = append(args, "--label", label)
	}
	res, err := c.run(ctx, args...)
	if err != nil {
		return Pane{}, err
	}
	var out struct {
		RootPane Pane `json:"root_pane"`
	}
	if err := decode(res, &out, "tab create"); err != nil {
		return Pane{}, err
	}
	return out.RootPane, nil
}

// WorkspaceCreate opens and focuses a new plain workspace at cwd — never
// herdr's native worktree machinery, which would compete with worktrunk's —
// and returns its root pane.
func (c *Client) WorkspaceCreate(ctx context.Context, cwd, label string) (Pane, error) {
	args := []string{"workspace", "create", "--cwd", cwd, "--focus"}
	if label != "" {
		args = append(args, "--label", label)
	}
	res, err := c.run(ctx, args...)
	if err != nil {
		return Pane{}, err
	}
	var out struct {
		RootPane Pane `json:"root_pane"`
	}
	if err := decode(res, &out, "workspace create"); err != nil {
		return Pane{}, err
	}
	return out.RootPane, nil
}

// PaneSplit splits targetPane (the focused pane when empty) with a new pane
// at cwd and returns it.
func (c *Client) PaneSplit(ctx context.Context, targetPane, cwd string) (Pane, error) {
	args := []string{"pane", "split", "--direction", "right", "--cwd", cwd, "--focus"}
	if targetPane != "" {
		args = append(args, "--pane", targetPane)
	} else {
		args = append(args, "--current")
	}
	res, err := c.run(ctx, args...)
	if err != nil {
		return Pane{}, err
	}
	var out struct {
		Pane Pane `json:"pane"`
	}
	if err := decode(res, &out, "pane split"); err != nil {
		return Pane{}, err
	}
	return out.Pane, nil
}

// PaneProcess is one process in a pane's foreground process group.
type PaneProcess struct {
	PID     int      `json:"pid"`
	Name    string   `json:"name"`
	Cmdline string   `json:"cmdline"`
	Argv    []string `json:"argv"`
	Cwd     string   `json:"cwd"`
}

// PaneProcessInfo is herdr's view of a pane's process state: the shell it
// spawned and whatever currently holds the terminal foreground.
type PaneProcessInfo struct {
	PaneID                   string        `json:"pane_id"`
	ShellPID                 int           `json:"shell_pid"`
	ForegroundProcessGroupID int           `json:"foreground_process_group_id"`
	ForegroundProcesses      []PaneProcess `json:"foreground_processes"`
}

// PaneProcessInfo returns the process state of a pane by id.
func (c *Client) PaneProcessInfo(ctx context.Context, paneID string) (PaneProcessInfo, error) {
	res, err := c.run(ctx, "pane", "process-info", "--pane", paneID)
	if err != nil {
		return PaneProcessInfo{}, err
	}
	var out struct {
		ProcessInfo PaneProcessInfo `json:"process_info"`
	}
	if err := decode(res, &out, "pane process-info"); err != nil {
		return PaneProcessInfo{}, err
	}
	return out.ProcessInfo, nil
}

// shellQuoteSafe are the characters a token may consist of and still be
// passed to the shell unquoted.
const shellQuoteSafe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-./:=@%+,"

// shellQuote returns s as a single POSIX shell word. Plain tokens pass
// through untouched; anything else is single-quoted, with each embedded
// single quote escaped by closing the string, emitting \' and reopening.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsFunc(s, func(r rune) bool {
		return !strings.ContainsRune(shellQuoteSafe, r)
	}) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// PaneRun runs argv in an existing pane's terminal. herdr joins the command
// args into one line typed into the pane's shell, so each argv item is
// shell-quoted to keep its word boundaries (#32).
func (c *Client) PaneRun(ctx context.Context, paneID string, argv []string) error {
	args := []string{"pane", "run", paneID}
	for _, a := range argv {
		args = append(args, shellQuote(a))
	}
	_, err := c.run(ctx, args...)
	return err
}

// PaneClose closes a pane by id.
func (c *Client) PaneClose(ctx context.Context, paneID string) error {
	_, err := c.run(ctx, "pane", "close", paneID)
	return err
}

// TabFocus focuses a tab by id.
func (c *Client) TabFocus(ctx context.Context, tabID string) error {
	_, err := c.run(ctx, "tab", "focus", tabID)
	return err
}

// WorkspaceFocus focuses a workspace by id.
func (c *Client) WorkspaceFocus(ctx context.Context, workspaceID string) error {
	_, err := c.run(ctx, "workspace", "focus", workspaceID)
	return err
}

// Notify shows a herdr notification. Per the action feedback conventions,
// trunkr notifies failures only — success is the visible effect itself.
func (c *Client) Notify(ctx context.Context, title, body string) error {
	args := []string{"notification", "show", title}
	if body != "" {
		args = append(args, "--body", body)
	}
	_, err := c.run(ctx, args...)
	return err
}

// PluginPaneOpen opens one of this plugin's [[panes]] entrypoints with extra
// environment variables. Placement comes from the manifest — the 0.7.5 CLI
// flag cannot request popup, the manifest can.
func (c *Client) PluginPaneOpen(ctx context.Context, entrypoint string, env map[string]string) error {
	if c.PluginID == "" {
		return errors.New("HERDR_PLUGIN_ID is not set; cannot open a plugin pane")
	}
	args := []string{"plugin", "pane", "open", "--plugin", c.PluginID, "--entrypoint", entrypoint}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--env", k+"="+env[k])
	}
	_, err := c.run(ctx, args...)
	return err
}
