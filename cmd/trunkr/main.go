package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"golang.org/x/term"
)

const pluginID = "disintegrator.trunkr"

var errPromptCancelled = errors.New("prompt cancelled")

// Keep handoff barriers open until the runner process exits. The detached
// helper receives EOF only after the popup command has completed.
var handoffBarriers []*os.File

type invocationContext struct {
	FocusedPaneCWD string `json:"focused_pane_cwd"`
	WorkspaceCWD   string `json:"workspace_cwd"`
	WorkspaceID    string `json:"workspace_id"`
	Worktree       *struct {
		RepoRoot string `json:"repo_root"`
	} `json:"worktree"`
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
	if len(args) == 3 && args[0] == "open-after-popup" {
		return openAfterPopup(args[1], args[2])
	}
	if len(args) == 4 && args[0] == "return-after-popup" {
		return returnAfterPopup(args[1], args[2], args[3])
	}
	if len(args) != 2 && !(len(args) == 1 && args[0] == "runner") {
		return errors.New("usage: trunkr action <create|open|merge|remove> | runner")
	}

	if args[0] == "action" {
		return openRunner(args[1])
	}

	action := os.Getenv("HERDR_TRUNKR_ACTION")
	cwd := os.Getenv("HERDR_TRUNKR_CWD")
	sourceCWD := os.Getenv("HERDR_TRUNKR_SOURCE_CWD")
	workspaceID := os.Getenv("HERDR_TRUNKR_WORKSPACE_ID")
	if action == "" || cwd == "" {
		return errors.New("runner context is missing")
	}
	if sourceCWD == "" {
		sourceCWD = cwd
	}

	fmt.Printf("Worktree: %s\nProject: %s\n\n", action, cwd)
	switch action {
	case "create":
		return createWorktree(cwd, sourceCWD)
	case "open":
		return openWorktree(cwd, sourceCWD)
	case "merge":
		return mergeWorktree(cwd, workspaceID)
	case "remove":
		return removeWorktree(cwd, workspaceID)
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

func openRunner(action string) error {
	if action != "create" && action != "open" && action != "merge" && action != "remove" {
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
	sourceCWD := herdrSourceCWD(context, cwd)

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
		"--env", "HERDR_TRUNKR_SOURCE_CWD=" + sourceCWD,
		"--env", "HERDR_TRUNKR_WORKSPACE_ID=" + context.WorkspaceID,
		"--focus",
	}
	cmd := exec.Command(herdr, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func herdrSourceCWD(context invocationContext, fallback string) string {
	if context.Worktree != nil && context.Worktree.RepoRoot != "" {
		return context.Worktree.RepoRoot
	}
	return fallback
}

func createWorktree(cwd, sourceCWD string) error {
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
	return handoffOpen(sourceCWD, result.Path)
}

func openWorktree(cwd, sourceCWD string) error {
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

	return handoffOpen(sourceCWD, selected)
}

func handoffOpen(cwd, path string) error {
	return handoffAfterPopup(cwd, "open.log", "open-after-popup", cwd, path)
}

func handoffReturn(repoRoot, checkout, workspaceID string) error {
	return handoffAfterPopup(repoRoot, "return.log", "return-after-popup", repoRoot, checkout, workspaceID)
}

func handoffAfterPopup(cwd, logName string, args ...string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate trunkr executable: %w", err)
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open null device: %w", err)
	}
	defer devNull.Close()
	output := io.Writer(devNull)
	if stateDir := os.Getenv("HERDR_PLUGIN_STATE_DIR"); stateDir != "" {
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return fmt.Errorf("create plugin state directory: %w", err)
		}
		logFile, err := os.OpenFile(filepath.Join(stateDir, logName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open worktree handoff log: %w", err)
		}
		defer logFile.Close()
		output = logFile
	}
	barrierReader, barrierWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create worktree handoff barrier: %w", err)
	}

	cmd := exec.Command(executable, args...)
	cmd.Dir = cwd
	cmd.Stdin = devNull
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.ExtraFiles = []*os.File{barrierReader}
	cmd.Env = append(os.Environ(), "HERDR_TRUNKR_BARRIER_FD=3")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = barrierReader.Close()
		_ = barrierWriter.Close()
		return fmt.Errorf("start worktree opener: %w", err)
	}
	if err := barrierReader.Close(); err != nil {
		_ = barrierWriter.Close()
		return fmt.Errorf("close worktree handoff reader: %w", err)
	}
	handoffBarriers = append(handoffBarriers, barrierWriter)
	return nil
}

func openAfterPopup(cwd, path string) error {
	if err := waitForRunner(); err != nil {
		return err
	}
	if err := closePopup(); err != nil {
		return err
	}
	return runHerdr("worktree", "open", "--cwd", cwd, "--path", path, "--focus")
}

func returnAfterPopup(repoRoot, checkout, workspaceID string) error {
	if err := waitForRunner(); err != nil {
		return err
	}
	if err := closePopup(); err != nil {
		return err
	}

	openErr := runHerdr("worktree", "open", "--cwd", repoRoot, "--path", repoRoot, "--focus")
	closeErr := runHerdr("workspace", "close", workspaceID)
	if closeErr != nil || openErr != nil {
		return fmt.Errorf("return from removed worktree %s: %w", checkout, errors.Join(openErr, closeErr))
	}
	return nil
}

func waitForRunner() error {
	if barrierFD := os.Getenv("HERDR_TRUNKR_BARRIER_FD"); barrierFD != "" {
		fd, err := strconv.Atoi(barrierFD)
		if err != nil {
			return fmt.Errorf("parse worktree handoff barrier: %w", err)
		}
		barrier := os.NewFile(uintptr(fd), "trunkr-runner-barrier")
		if barrier == nil {
			return errors.New("open worktree handoff barrier")
		}
		if _, err := io.Copy(io.Discard, barrier); err != nil {
			return fmt.Errorf("wait for trunkr runner: %w", err)
		}
		if err := barrier.Close(); err != nil {
			return fmt.Errorf("close worktree handoff barrier: %w", err)
		}
	}
	return nil
}

func closePopup() error {
	socketPath := os.Getenv("HERDR_SOCKET_PATH")
	if socketPath == "" {
		return nil
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to Herdr: %w", err)
	}
	defer conn.Close()

	request := map[string]any{
		"id":     "trunkr:popup-close",
		"method": "popup.close",
		"params": map[string]any{},
	}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("close trunkr popup: %w", err)
	}
	var response struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("read popup close response: %w", err)
	}
	if response.Error != nil && response.Error.Code != "popup_not_open" {
		return fmt.Errorf("close trunkr popup: %s", response.Error.Message)
	}
	return nil
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
	checkout, repoRoot, err := destructiveWorktreeRoots(cwd)
	if err != nil {
		return err
	}
	answer, err := prompt(bufio.NewReader(os.Stdin), "Remove this worktree? [y/N]")
	if err != nil {
		return err
	}
	if answer != "y" && answer != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}

	cmd := exec.Command("wt", "-C", checkout, "remove", "--foreground", "--format=json", checkout)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove with Worktrunk: %w", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		return fmt.Errorf("move to primary checkout after removal: %w", err)
	}
	return finishRemovedWorktree(repoRoot, checkout, workspaceID)
}

func mergeWorktree(cwd, workspaceID string) error {
	if workspaceID == "" {
		return errors.New("merge requires a Herdr workspace")
	}
	checkout, repoRoot, err := destructiveWorktreeRoots(cwd)
	if err != nil {
		return err
	}
	answer, err := prompt(bufio.NewReader(os.Stdin), "Merge this worktree into the default branch? [y/N]")
	if err != nil {
		return err
	}
	if answer != "y" && answer != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}

	cmd := exec.Command("wt", "-C", checkout, "merge", "--format=json")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merge with Worktrunk: %w", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		return fmt.Errorf("move to primary checkout after merge: %w", err)
	}
	return finishRemovedWorktree(repoRoot, checkout, workspaceID)
}

func destructiveWorktreeRoots(cwd string) (checkout, repoRoot string, err error) {
	checkout, err = checkoutRoot(cwd)
	if err != nil {
		return "", "", err
	}
	commonDir, err := commandOutput("git", "-C", checkout, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	repoRoot, err = checkoutRoot(filepath.Dir(commonDir))
	if err != nil {
		return "", "", err
	}
	if samePath(checkout, repoRoot) {
		return "", "", errors.New("the primary checkout cannot be removed or merged")
	}
	return checkout, repoRoot, nil
}

func checkoutRoot(cwd string) (string, error) {
	root, err := commandOutput("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--show-toplevel")
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve checkout root: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func finishRemovedWorktree(repoRoot, checkout, workspaceID string) error {
	registered, err := isRegisteredWorktree(repoRoot, checkout)
	if err != nil {
		return err
	}
	if registered {
		return errors.New("Worktrunk kept the checkout; leaving its Herdr workspace open")
	}
	return handoffReturn(repoRoot, checkout, workspaceID)
}

func isRegisteredWorktree(repoRoot, checkout string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain", "-z")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("list Git worktrees: %w", err)
	}
	for _, path := range worktreePaths(output) {
		if samePath(path, checkout) {
			return true, nil
		}
	}
	return false, nil
}

func worktreePaths(output []byte) []string {
	var paths []string
	for field := range bytes.SplitSeq(output, []byte{0}) {
		if path, ok := bytes.CutPrefix(field, []byte("worktree ")); ok {
			paths = append(paths, string(path))
		}
	}
	return paths
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
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
