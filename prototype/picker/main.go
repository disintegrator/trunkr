// Prototype of trunkr's interactive picker pane. All data is fake and every
// action prints the wt/herdr command it *would* run instead of executing it.
//
// Run it in a real terminal:
//
//	go run .
//
// Or print a static snapshot of the list view (no TTY needed):
//
//	go run . -snapshot
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// worktree mirrors the data trunkr derives live: git facts from
// `wt list --format=json`, pane facts from `herdr pane list` matched by cwd.
type worktree struct {
	branch  string
	path    string
	dirty   bool
	ahead   int
	behind  int
	panes   int
	agents  []string // agent_status of each live pane
	isTrunk bool
}

func fakeWorktrees() []worktree {
	return []worktree{
		{branch: "main", path: "~/code/myrepo", isTrunk: true, panes: 1, agents: []string{"idle"}},
		{branch: "feat/auth-retry", path: "~/code/myrepo.feat-auth-retry", dirty: true, ahead: 3, panes: 2, agents: []string{"working", "idle"}},
		{branch: "fix/flaky-tests", path: "~/code/myrepo.fix-flaky-tests", ahead: 1, panes: 1, agents: []string{"blocked"}},
		{branch: "feat/dark-mode", path: "~/code/myrepo.feat-dark-mode", ahead: 7, behind: 2, panes: 1, agents: []string{"done"}},
		{branch: "chore/deps", path: "~/code/myrepo.chore-deps", panes: 0},
	}
}

// rollup picks the most attention-worthy agent status across a worktree's panes.
func rollup(agents []string) string {
	rank := map[string]int{"blocked": 0, "working": 1, "done": 2, "idle": 3, "unknown": 4}
	best := ""
	for _, a := range agents {
		if best == "" || rank[a] < rank[best] {
			best = a
		}
	}
	return best
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	cursorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	trunkStyle   = lipgloss.NewStyle().Faint(true)
	dirtyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	aheadStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	behindStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	statusStyles = map[string]lipgloss.Style{
		"blocked": lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		"working": lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		"done":    lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		"idle":    lipgloss.NewStyle().Faint(true),
		"unknown": lipgloss.NewStyle().Faint(true),
	}
	statusGlyphs = map[string]string{
		"blocked": "■", "working": "●", "done": "✓", "idle": "○", "unknown": "?",
	}
	msgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	helpStyle   = lipgloss.NewStyle().Faint(true)
)

type mode int

const (
	modeList mode = iota
	modeFilter
	modeInput   // create / pr-checkout text entry
	modeConfirm // merge / destroy y-n
)

type model struct {
	trees   []worktree
	cursor  int
	mode    mode
	filter  string
	input   string
	prompt  string        // question shown in input/confirm modes
	pending func() string // action to run on confirm/submit; returns status msg
	status  string        // last "would run: …" message
}

func (m model) visible() []worktree {
	if m.filter == "" {
		return m.trees
	}
	var out []worktree
	for _, t := range m.trees {
		if strings.Contains(t.branch, m.filter) {
			out = append(out, t)
		}
	}
	return out
}

func (m model) current() (worktree, bool) {
	v := m.visible()
	if len(v) == 0 || m.cursor >= len(v) {
		return worktree{}, false
	}
	return v[m.cursor], true
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.mode {
	case modeFilter, modeInput:
		switch key.Type {
		case tea.KeyEsc:
			m.mode, m.input, m.prompt = modeList, "", ""
			if m.mode == modeFilter {
				m.filter = ""
			}
		case tea.KeyEnter:
			if m.mode == modeInput && m.pending != nil && m.input != "" {
				m.status = m.pending()
			}
			m.mode, m.input, m.prompt, m.pending = modeList, "", "", nil
		case tea.KeyBackspace:
			if m.mode == modeFilter && len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
			} else if m.mode == modeInput && len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case tea.KeyRunes, tea.KeySpace:
			if m.mode == modeFilter {
				m.filter += string(key.Runes)
				m.cursor = 0
			} else {
				m.input += string(key.Runes)
			}
		}
		return m, nil

	case modeConfirm:
		switch key.String() {
		case "y", "Y":
			m.status = m.pending()
		}
		m.mode, m.prompt, m.pending = modeList, "", nil
		return m, nil
	}

	// modeList
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible())-1 {
			m.cursor++
		}
	case "/":
		m.mode, m.filter = modeFilter, ""
	case "r":
		m.status = "would refresh: wt list --format=json + herdr pane list"

	case "enter": // generic switch: focus existing, else open in configured container
		if t, ok := m.current(); ok {
			if t.panes > 0 {
				m.status = fmt.Sprintf("would focus existing container of %s (herdr tab focus …)", t.branch)
			} else {
				m.status = fmt.Sprintf("would open %s in default container (tab): wt switch %s → herdr tab create --cwd <path> + agent cmd", t.branch, t.branch)
			}
		}
	case "t", "w", "s": // explicit per-mode opens: always a new pane
		if t, ok := m.current(); ok {
			target := map[string]string{"t": "tab", "w": "workspace", "s": "split"}[key.String()]
			m.status = fmt.Sprintf("would open %s in new %s: wt switch %s → herdr %s create --cwd <path> + agent cmd", t.branch, target, t.branch, target)
		}

	case "c":
		m.mode, m.prompt = modeInput, "new branch name: "
		m.pending = func() string {
			return fmt.Sprintf("would run: wt switch -c %s --format json, then open in default container", m.input)
		}
	case "p":
		m.mode, m.prompt = modeInput, "PR number or URL: "
		m.pending = func() string {
			return fmt.Sprintf("would run: wt switch pr:%s --format json, then open in default container", m.input)
		}

	case "m":
		if t, ok := m.current(); ok && !t.isTrunk {
			m.mode = modeConfirm
			extra := ""
			if t.dirty {
				extra = " (commits dirty tree)"
			}
			m.prompt = fmt.Sprintf("merge %s into main%s and remove its worktree? [y/N] ", t.branch, extra)
			m.pending = func() string {
				return fmt.Sprintf("would run: wt merge %s --format json (squash+rebase, auto-removes worktree)", t.branch)
			}
		}
	case "d":
		if t, ok := m.current(); ok && !t.isTrunk {
			m.mode = modeConfirm
			paneNote := ""
			if t.panes > 0 {
				paneNote = fmt.Sprintf(" — closes %d live pane(s) first", t.panes)
			}
			m.prompt = fmt.Sprintf("destroy %s%s? [y/N] ", t.branch, paneNote)
			m.pending = func() string {
				return fmt.Sprintf("would run: herdr pane close ×%d, then wt remove %s --format json", t.panes, t.branch)
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("trunkr — worktrees") + dimStyle.Render("  (myrepo)") + "\n\n")

	b.WriteString(dimStyle.Render(fmt.Sprintf("  %-22s %-6s %-10s %s", "BRANCH", "PANES", "AGENTS", "GIT")) + "\n")

	for i, t := range m.visible() {
		cursor := "  "
		line := ""
		if i == m.cursor && m.mode == modeList {
			cursor = cursorStyle.Render("▸ ")
		}

		branch := t.branch
		if t.isTrunk {
			branch = trunkStyle.Render(branch + " (trunk)")
			line = fmt.Sprintf("%-22s", branch)
		} else {
			line = fmt.Sprintf("%-22s", branch)
		}

		panes := "–"
		if t.panes > 0 {
			panes = fmt.Sprintf("%d", t.panes)
		}
		line += fmt.Sprintf(" %-6s", panes)

		agents := dimStyle.Render("–") + strings.Repeat(" ", 9)
		if s := rollup(t.agents); s != "" {
			agents = statusStyles[s].Render(fmt.Sprintf("%s %-8s", statusGlyphs[s], s))
		}
		line += " " + agents

		var git []string
		if t.ahead > 0 {
			git = append(git, aheadStyle.Render(fmt.Sprintf("+%d", t.ahead)))
		}
		if t.behind > 0 {
			git = append(git, behindStyle.Render(fmt.Sprintf("−%d", t.behind)))
		}
		if t.dirty {
			git = append(git, dirtyStyle.Render("~dirty"))
		}
		if len(git) == 0 {
			git = append(git, dimStyle.Render("clean"))
		}
		line += " " + strings.Join(git, " ")

		b.WriteString(cursor + line + "\n")
	}

	b.WriteString("\n")
	switch m.mode {
	case modeFilter:
		b.WriteString(promptStyle.Render("filter: ") + m.filter + "█\n")
	case modeInput:
		b.WriteString(promptStyle.Render(m.prompt) + m.input + "█\n")
	case modeConfirm:
		b.WriteString(promptStyle.Render(m.prompt) + "\n")
	default:
		if m.status != "" {
			b.WriteString(msgStyle.Render("→ "+m.status) + "\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("\nenter switch · t tab · w workspace · s split · c create · p pr · m merge · d destroy · / filter · r refresh · q quit\n"))
	return b.String()
}

func main() {
	snapshot := flag.Bool("snapshot", false, "print a static snapshot of the list view and exit")
	flag.Parse()

	m := model{trees: fakeWorktrees(), cursor: 1}
	if *snapshot {
		fmt.Print(m.View())
		return
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
