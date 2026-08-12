package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"golang.org/x/term"
)

const pluginID = "disintegrator.trunkr"

var errPromptCancelled = errors.New("prompt cancelled")

type invocationContext struct {
	FocusedPaneCWD string `json:"focused_pane_cwd"`
	WorkspaceCWD   string `json:"workspace_cwd"`
	WorkspaceID    string `json:"workspace_id"`
}

type worktree struct {
	Branch  string
	Path    string
	Current bool
}

type wtList struct {
	Items []struct {
		Branch   *string `json:"branch"`
		Worktree *struct {
			Path    string `json:"path"`
			Current bool   `json:"current"`
		} `json:"worktree"`
	} `json:"items"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, errPromptCancelled) {
			return
		}
		fmt.Fprintf(os.Stderr, "trunkr: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 && !(len(args) == 1 && args[0] == "runner") {
		return errors.New("usage: trunkr action <create|open|remove> | runner")
	}

	if args[0] == "action" {
		return openRunner(args[1])
	}

	action := os.Getenv("HERDR_TRUNKR_ACTION")
	cwd := os.Getenv("HERDR_TRUNKR_CWD")
	workspaceID := os.Getenv("HERDR_TRUNKR_WORKSPACE_ID")
	if action == "" || cwd == "" {
		return errors.New("runner context is missing")
	}

	fmt.Printf("Worktree: %s\nProject: %s\n\n", action, cwd)
	switch action {
	case "create":
		return createWorktree(cwd)
	case "open":
		return openWorktree(cwd)
	case "remove":
		return removeWorktree(cwd, workspaceID)
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

func openRunner(action string) error {
	if action != "create" && action != "open" && action != "remove" {
		return fmt.Errorf("unknown action %q", action)
	}

	var context invocationContext
	if err := json.Unmarshal([]byte(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON")), &context); err != nil {
		return fmt.Errorf("decode Herdr context: %w", err)
	}
	cwd := context.FocusedPaneCWD
	if cwd == "" {
		cwd = context.WorkspaceCWD
	}
	if cwd == "" {
		return errors.New("the focused Herdr workspace has no working directory")
	}

	herdr := os.Getenv("HERDR_BIN_PATH")
	if herdr == "" {
		herdr = "herdr"
	}
	args := []string{
		"plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", "runner",
		"--cwd", cwd,
		"--env", "HERDR_TRUNKR_ACTION=" + action,
		"--env", "HERDR_TRUNKR_CWD=" + cwd,
		"--env", "HERDR_TRUNKR_WORKSPACE_ID=" + context.WorkspaceID,
		"--focus",
	}
	cmd := exec.Command(herdr, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createWorktree(cwd string) error {
	reader := bufio.NewReader(os.Stdin)
	branch, err := prompt(reader, "New branch")
	if err != nil {
		return err
	}
	if branch == "" {
		return errors.New("branch is required")
	}
	base, err := prompt(reader, "Base ref (default: repository default)")
	if err != nil {
		return err
	}

	args := []string{"-C", cwd, "switch", "--create", branch, "--no-cd", "--format=json"}
	if base != "" {
		args = append(args, "--base", base)
	}
	var result struct {
		Path string `json:"path"`
	}
	if err := runJSONCommand(&result, "wt", args...); err != nil {
		return err
	}
	if result.Path == "" {
		return errors.New("Worktrunk did not return the new worktree path")
	}
	return runHerdr("worktree", "open", "--cwd", cwd, "--path", result.Path, "--focus")
}

func openWorktree(cwd string) error {
	worktrees, err := listWorktrees(cwd)
	if err != nil {
		return err
	}
	if len(worktrees) == 0 {
		return errors.New("no worktrees found")
	}

	selected, err := selectWorktree(worktrees)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errPromptCancelled
		}
		return err
	}

	return runHerdr("worktree", "open", "--cwd", cwd, "--path", selected, "--focus")
}

func selectWorktree(worktrees []worktree) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		for i, item := range worktrees {
			fmt.Printf("%2d. %s\n", i+1, worktreeLabel(item))
		}
		fmt.Println()
		selection, err := prompt(bufio.NewReader(os.Stdin), "Open number")
		if err != nil {
			return "", err
		}
		for i := range worktrees {
			if selection == fmt.Sprintf("%d", i+1) {
				return worktrees[i].Path, nil
			}
		}
		return "", fmt.Errorf("invalid selection %q", selection)
	}

	options := make([]huh.Option[string], 0, len(worktrees))
	selected := worktrees[0].Path
	for _, item := range worktrees {
		option := huh.NewOption(worktreeLabel(item), item.Path)
		if item.Current {
			option = option.Selected(true)
			selected = item.Path
		}
		options = append(options, option)
	}

	height := min(len(options)+2, 16)
	keymap := huh.NewDefaultKeyMap()
	keymap.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"))
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Switch worktree").
			Description("Choose a branch. Press / to filter by branch and path.").
			Options(options...).
			Value(&selected).
			Height(height),
	)).WithKeyMap(keymap).Run()
	if err != nil {
		return "", fmt.Errorf("select worktree: %w", err)
	}
	return selected, nil
}

func worktreeLabel(item worktree) string {
	marker := "○"
	status := ""
	if item.Current {
		marker = "●"
		status = "  current"
	}
	return fmt.Sprintf("%s %-28s %s%s", marker, item.Branch, displayPath(item.Path), status)
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && path != home && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func removeWorktree(cwd, workspaceID string) error {
	if workspaceID == "" {
		return errors.New("remove requires a Herdr workspace")
	}
	answer, err := prompt(bufio.NewReader(os.Stdin), "Remove this worktree? [y/N]")
	if err != nil {
		return err
	}
	if answer != "y" && answer != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}

	primaryCheckout, err := commandOutput("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	primaryCheckout = filepath.Dir(primaryCheckout)

	cmd := exec.Command("wt", "-C", cwd, "remove", "--foreground", "--format=json", cwd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove with Worktrunk: %w", err)
	}
	if err := os.Chdir(primaryCheckout); err != nil {
		return fmt.Errorf("move to primary checkout after removal: %w", err)
	}
	return runHerdr("workspace", "close", workspaceID)
}

func listWorktrees(cwd string) ([]worktree, error) {
	var list wtList
	if err := runJSONCommand(&list, "wt", "-C", cwd, "list", "--format=json", "--config-set", "list.json-schema=2"); err != nil {
		return nil, err
	}
	items := make([]worktree, 0, len(list.Items))
	for _, item := range list.Items {
		if item.Worktree == nil {
			continue
		}
		branch := "detached"
		if item.Branch != nil {
			branch = *item.Branch
		}
		items = append(items, worktree{Branch: branch, Path: item.Worktree.Path, Current: item.Worktree.Current})
	}
	return items, nil
}

func prompt(reader *bufio.Reader, label string) (value string, err error) {
	fmt.Printf("%s: ", label)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return readLinePrompt(reader)
	}

	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("enable raw input: %w", err)
	}
	defer func() {
		if restoreErr := term.Restore(int(os.Stdin.Fd()), state); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore terminal input: %w", restoreErr))
		}
	}()

	value, err = readRawPrompt(reader, os.Stdout)
	return value, err
}

func readLinePrompt(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read input: %w", err)
	}
	if strings.ContainsRune(value, '\x1b') {
		return "", errPromptCancelled
	}
	return strings.TrimSpace(value), nil
}

func readRawPrompt(reader *bufio.Reader, output io.Writer) (string, error) {
	var value []byte
	for {
		input, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return strings.TrimSpace(string(value)), nil
			}
			return "", fmt.Errorf("read input: %w", err)
		}

		switch input {
		case '\x1b', '\x03':
			return "", errPromptCancelled
		case '\r', '\n':
			if _, err := io.WriteString(output, "\r\n"); err != nil {
				return "", fmt.Errorf("write prompt: %w", err)
			}
			return strings.TrimSpace(string(value)), nil
		case '\b', '\x7f':
			if len(value) == 0 {
				continue
			}
			_, size := utf8.DecodeLastRune(value)
			value = value[:len(value)-size]
			if _, err := io.WriteString(output, "\b \b"); err != nil {
				return "", fmt.Errorf("write prompt: %w", err)
			}
		default:
			if input < 0x20 {
				continue
			}
			value = append(value, input)
			if _, err := output.Write([]byte{input}); err != nil {
				return "", fmt.Errorf("write prompt: %w", err)
			}
		}
	}
}

func runHerdr(args ...string) error {
	cmd := exec.Command(herdrBinary(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run Herdr: %w", err)
	}
	return nil
}

func herdrBinary() string {
	if path := os.Getenv("HERDR_BIN_PATH"); path != "" {
		return path
	}
	return "herdr"
}

func runJSONCommand(result any, name string, args ...string) error {
	var stdout bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	if err := json.Unmarshal(stdout.Bytes(), result); err != nil {
		return fmt.Errorf("decode %s output: %w", name, err)
	}
	return nil
}

func commandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run %s: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}
