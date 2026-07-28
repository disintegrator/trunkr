// trunkr is a herdr plugin binary. Every manifest entrypoint (actions, panes)
// invokes this binary with a subcommand; herdr sets the plugin root as cwd and
// passes all context via HERDR_* environment variables.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: trunkr <hello|hello-pane>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "hello":
		runHello()
	case "hello-pane":
		runHelloPane()
	default:
		fmt.Fprintf(os.Stderr, "trunkr: unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
}

// herdr invokes the running herdr binary (via HERDR_BIN_PATH) with the given
// args, inheriting the plugin environment so socket routing just works.
func herdr(args ...string) (string, error) {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		return "", fmt.Errorf("HERDR_BIN_PATH is not set; trunkr must run as a herdr plugin command")
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// runHello is the walking-skeleton action: prove we can find wt on PATH and
// call back into herdr. Output lands in the plugin command log.
func runHello() {
	wtPath, err := exec.LookPath("wt")
	if err != nil {
		msg := "worktrunk (wt) not found on PATH — install it from https://github.com/max-sixty/worktrunk"
		fmt.Fprintln(os.Stderr, "trunkr: "+msg)
		herdr("notification", "show", "trunkr: wt missing", "--body", msg, "--sound", "request")
		os.Exit(1)
	}

	verOut, err := exec.Command(wtPath, "--version").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "trunkr: found %s but `wt --version` failed: %v\n", wtPath, err)
		os.Exit(1)
	}
	wtVersion := strings.TrimSpace(string(verOut))

	body := fmt.Sprintf("%s at %s · plugin %s", wtVersion, wtPath, os.Getenv("HERDR_PLUGIN_ID"))
	if out, err := herdr("notification", "show", "trunkr says hello", "--body", body); err != nil {
		fmt.Fprintf(os.Stderr, "trunkr: herdr callback failed: %v\n%s\n", err, out)
		os.Exit(1)
	}

	fmt.Printf("hello ok: %s; herdr callback ok (HERDR_BIN_PATH=%s)\n", body, os.Getenv("HERDR_BIN_PATH"))
}

type helloModel struct{}

func (helloModel) Init() tea.Cmd { return nil }

func (m helloModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (helloModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("trunkr skeleton")
	dim := lipgloss.NewStyle().Faint(true)
	return fmt.Sprintf("%s\n\nBubble Tea is running inside a herdr plugin pane.\n%s\n%s\n\n%s\n",
		title,
		dim.Render("pane:      "+os.Getenv("HERDR_PANE_ID")),
		dim.Render("workspace: "+os.Getenv("HERDR_WORKSPACE_ID")),
		dim.Render("press q to close"),
	)
}

// runHelloPane is the [[panes]] smoke test: a minimal Bubble Tea program, so
// the picker's TUI stack is proven inside a real herdr overlay pane.
func runHelloPane() {
	if _, err := tea.NewProgram(helloModel{}, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
