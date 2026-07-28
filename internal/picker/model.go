package picker

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Action is a worktree operation the picker hands back to its caller, which
// executes it (in production: the runner child via tea.ExecProcess).
type Action struct {
	Op   string // switch | create | pr
	Mode string // container override: tab | workspace | split; "" = generic
	Ref  string // branch name or PR input
}

// RefreshedMsg carries a fresh row set (or the load error) into the model.
type RefreshedMsg struct {
	Rows []Row
	Err  error
}

// ActionDoneMsg reports a finished action. Quit closes the picker — set on
// success of switch-family actions, whose outcome is a focus change the
// overlay would otherwise sit on top of.
type ActionDoneMsg struct {
	Err  error
	Quit bool
}

// tickMsg schedules the next automatic refresh.
type tickMsg struct{}

type mode int

const (
	modeList mode = iota
	modeFilter
	modeInput
	modeConfirm
)

// inputKind names what the input prompt collects.
type inputKind int

const (
	inputCreate inputKind = iota
	inputPR
)

// actionOrder fixes the help-line ordering of the [picker.keys] actions.
var actionOrder = []string{"tab", "workspace", "split", "create", "pr", "merge", "destroy"}

// Model is the picker's Bubble Tea model. Exec and Refresh are injected so
// Update stays free of I/O.
type Model struct {
	// Exec turns an Action into the command that runs it. Required.
	Exec func(Action) tea.Cmd
	// Refresh loads rows, returning a RefreshedMsg. Required.
	Refresh func() tea.Msg
	// Interval between automatic refreshes; 0 disables the tick (tests).
	Interval time.Duration

	rows      []Row
	repoName  string
	cursor    int
	mode      mode
	filter    string
	input     string
	prompt    string
	inputKind inputKind
	status    string
	keyAction map[string]string // key → action name, from [picker.keys]

	// confirmAction is the destructive action pending its inline y/N answer.
	confirmAction Action
}

// New builds a picker model. keys is the resolved [picker.keys] map
// (action name → key), defaults already applied by the config layer.
func New(keys map[string]string, exec func(Action) tea.Cmd, refresh func() tea.Msg, interval time.Duration) Model {
	byKey := make(map[string]string, len(keys))
	for action, key := range keys {
		byKey[key] = action
	}
	return Model{
		Exec:      exec,
		Refresh:   refresh,
		Interval:  interval,
		keyAction: byKey,
	}
}

// keyFor returns the key bound to an action name, for the help line.
func (m Model) keyFor(action string) string {
	for key, a := range m.keyAction {
		if a == action {
			return key
		}
	}
	return ""
}

func (m Model) visible() []Row {
	if m.filter == "" {
		return m.rows
	}
	needle := strings.ToLower(m.filter)
	var out []Row
	for _, r := range m.rows {
		if strings.Contains(strings.ToLower(r.Branch), needle) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) current() (Row, bool) {
	v := m.visible()
	if len(v) == 0 || m.cursor >= len(v) {
		return Row{}, false
	}
	return v[m.cursor], true
}

func (m Model) clampCursor() Model {
	if n := len(m.visible()); m.cursor >= n {
		m.cursor = max(0, n-1)
	}
	return m
}

func (m Model) tick() tea.Cmd {
	if m.Interval <= 0 {
		return nil
	}
	return tea.Tick(m.Interval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Refresh, m.tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshedMsg:
		if msg.Err != nil {
			m.status = "refresh failed: " + msg.Err.Error()
			return m, nil
		}
		m.rows = msg.Rows
		if name := RepoName(msg.Rows); name != "" {
			m.repoName = name
		}
		return m.clampCursor(), nil

	case tickMsg:
		return m, tea.Batch(m.Refresh, m.tick())

	case ActionDoneMsg:
		if msg.Err == nil && msg.Quit {
			return m, tea.Quit
		}
		if msg.Err != nil {
			m.status = "failed: " + msg.Err.Error() + " — see notification"
		}
		return m, m.Refresh

	case tea.KeyMsg:
		switch m.mode {
		case modeFilter:
			return m.updateFilter(msg)
		case modeInput:
			return m.updateInput(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m Model) updateFilter(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.mode, m.filter = modeList, ""
		return m.clampCursor(), nil
	case tea.KeyEnter:
		m.mode = modeList
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		m.filter += string(key.Runes)
		m.cursor = 0
	}
	return m.clampCursor(), nil
}

func (m Model) updateInput(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.mode, m.input, m.prompt = modeList, "", ""
	case tea.KeyEnter:
		input := strings.TrimSpace(m.input)
		kind := m.inputKind
		m.mode, m.input, m.prompt = modeList, "", ""
		if input == "" {
			return m, nil
		}
		action := Action{Op: "create", Ref: input}
		if kind == inputPR {
			action = Action{Op: "pr", Ref: input}
		}
		return m, m.Exec(action)
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		m.input += string(key.Runes)
	}
	return m, nil
}

// updateConfirm answers the inline y/N confirm: y executes the pending
// action, anything else cancels.
func (m Model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.confirmAction
	m.mode, m.prompt, m.confirmAction = modeList, "", Action{}
	if key.Type == tea.KeyRunes && strings.EqualFold(string(key.Runes), "y") {
		m.status = ""
		return m, m.Exec(action)
	}
	m.status = "cancelled"
	return m, nil
}

func (m Model) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.visible())-1 {
			m.cursor++
		}
		return m, nil
	case "/":
		m.mode, m.filter, m.cursor = modeFilter, "", 0
		return m, nil
	case "r":
		m.status = "refreshing…"
		return m, m.Refresh
	case "enter":
		if row, ok := m.current(); ok {
			return m, m.Exec(Action{Op: "switch", Ref: row.Branch})
		}
		return m, nil
	}

	switch m.keyAction[key.String()] {
	case "tab", "workspace", "split":
		if row, ok := m.current(); ok {
			return m, m.Exec(Action{Op: "switch", Mode: m.keyAction[key.String()], Ref: row.Branch})
		}
	case "create":
		m.mode, m.inputKind, m.prompt = modeInput, inputCreate, "new branch name: "
	case "pr":
		m.mode, m.inputKind, m.prompt = modeInput, inputPR, "PR number or URL: "
	case "merge":
		if row, ok := m.current(); ok {
			if row.IsTrunk {
				m.status = "cannot merge the trunk worktree"
				return m, nil
			}
			m.mode = modeConfirm
			m.confirmAction = Action{Op: "merge", Ref: row.Branch}
			m.prompt = fmt.Sprintf("merge %s into trunk? commits dirty changes, closes %d pane(s), removes the worktree [y/N] ", row.Branch, row.Panes)
		}
	case "destroy":
		if row, ok := m.current(); ok {
			if row.IsTrunk {
				m.status = "cannot destroy the trunk worktree"
				return m, nil
			}
			m.mode = modeConfirm
			m.confirmAction = Action{Op: "destroy", Ref: row.Branch}
			m.prompt = fmt.Sprintf("destroy %s? DISCARDS uncommitted changes, closes %d pane(s), removes the worktree [y/N] ", row.Branch, row.Panes)
		}
	}
	return m, nil
}

// helpLine lists the active bindings in a stable order.
func (m Model) helpLine() string {
	parts := []string{"enter switch"}
	labels := map[string]string{
		"tab": "tab", "workspace": "workspace", "split": "split",
		"create": "create", "pr": "pr", "merge": "merge", "destroy": "destroy",
	}
	for _, action := range actionOrder {
		label, active := labels[action]
		if !active {
			continue
		}
		if key := m.keyFor(action); key != "" {
			parts = append(parts, fmt.Sprintf("%s %s", key, label))
		}
	}
	parts = append(parts, "/ filter", "r refresh", "q quit")
	return strings.Join(parts, " · ")
}
