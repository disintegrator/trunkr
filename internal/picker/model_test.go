package picker

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/disintegrator/trunkr/internal/config"
)

// harness builds a model with fake exec/refresh and applies key strokes.
type harness struct {
	model    Model
	actions  []Action
	refreshs int
}

func newHarness(rows []Row) *harness {
	h := &harness{}
	h.model = New(config.Default().PickerKeys,
		func(a Action) tea.Cmd {
			h.actions = append(h.actions, a)
			return nil
		},
		func() tea.Msg {
			h.refreshs++
			return RefreshedMsg{Rows: rows}
		},
		0, // no auto tick in tests
	)
	h.model.rows = rows
	return h
}

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press applies a message and keeps the returned model (running any returned
// non-exec cmd is the caller's business).
func (h *harness) press(msg tea.Msg) tea.Cmd {
	m, cmd := h.model.Update(msg)
	h.model = m.(Model)
	return cmd
}

func rows3() []Row {
	return []Row{
		{Branch: "main", Path: "/r", IsTrunk: true, Panes: 1, Agents: []string{"idle"}},
		{Branch: "feat/auth", Path: "/r.feat-auth", Panes: 2, Agents: []string{"working", "blocked"}},
		{Branch: "fix/tests", Path: "/r.fix-tests"},
	}
}

func TestNavigationBounds(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("k")) // already at top
	if h.model.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", h.model.cursor)
	}
	h.press(keyRunes("j"))
	h.press(keyRunes("j"))
	h.press(keyRunes("j")) // clamped at bottom
	if h.model.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", h.model.cursor)
	}
}

func TestEnterEmitsGenericSwitch(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("j"))
	h.press(tea.KeyMsg{Type: tea.KeyEnter})
	want := Action{Op: "switch", Ref: "feat/auth"}
	if len(h.actions) != 1 || h.actions[0] != want {
		t.Fatalf("actions = %+v, want [%+v]", h.actions, want)
	}
}

func TestPerModeKeysEmitModeActions(t *testing.T) {
	tests := []struct {
		key  string
		mode string
	}{
		{"t", "tab"}, {"w", "workspace"}, {"s", "split"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			h := newHarness(rows3())
			h.press(keyRunes(tt.key))
			want := Action{Op: "switch", Mode: tt.mode, Ref: "main"}
			if len(h.actions) != 1 || h.actions[0] != want {
				t.Fatalf("actions = %+v, want [%+v]", h.actions, want)
			}
		})
	}
}

func TestRemappedKeys(t *testing.T) {
	keys := config.Default().PickerKeys
	keys["tab"] = "T" // user remap via [picker.keys]
	h := newHarness(rows3())
	h.model = New(keys,
		func(a Action) tea.Cmd { h.actions = append(h.actions, a); return nil },
		func() tea.Msg { return RefreshedMsg{} }, 0)
	h.model.rows = rows3()

	h.press(keyRunes("t")) // old key: no longer bound
	if len(h.actions) != 0 {
		t.Fatalf("unbound key emitted %+v", h.actions)
	}
	h.press(keyRunes("T"))
	if len(h.actions) != 1 || h.actions[0].Mode != "tab" {
		t.Fatalf("actions = %+v, want one tab action", h.actions)
	}
}

func TestCreatePromptFlow(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("c"))
	if h.model.mode != modeInput {
		t.Fatal("c should enter input mode")
	}
	for _, r := range "feat/new" {
		h.press(keyRunes(string(r)))
	}
	h.press(tea.KeyMsg{Type: tea.KeyEnter})
	want := Action{Op: "create", Ref: "feat/new"}
	if len(h.actions) != 1 || h.actions[0] != want {
		t.Fatalf("actions = %+v, want [%+v]", h.actions, want)
	}
	if h.model.mode != modeList {
		t.Fatal("enter should return to list mode")
	}
}

func TestPRPromptFlow(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("p"))
	for _, r := range "123" {
		h.press(keyRunes(string(r)))
	}
	h.press(tea.KeyMsg{Type: tea.KeyEnter})
	want := Action{Op: "pr", Ref: "123"}
	if len(h.actions) != 1 || h.actions[0] != want {
		t.Fatalf("actions = %+v, want [%+v]", h.actions, want)
	}
}

func TestInputEscapeCancels(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("c"))
	h.press(keyRunes("x"))
	h.press(tea.KeyMsg{Type: tea.KeyEsc})
	if len(h.actions) != 0 {
		t.Fatalf("escape should not emit an action, got %+v", h.actions)
	}
	if h.model.mode != modeList || h.model.input != "" {
		t.Fatalf("escape should reset input mode: %+v", h.model)
	}
}

func TestEmptyInputEmitsNothing(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("c"))
	h.press(tea.KeyMsg{Type: tea.KeyEnter})
	if len(h.actions) != 0 {
		t.Fatalf("empty input should not emit an action, got %+v", h.actions)
	}
}

func TestFilterFlow(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("/"))
	for _, r := range "FEAT" { // case-insensitive
		h.press(keyRunes(string(r)))
	}
	if v := h.model.visible(); len(v) != 1 || v[0].Branch != "feat/auth" {
		t.Fatalf("visible = %+v, want just feat/auth", v)
	}
	// Enter keeps the filter; the cursor row is the filtered one.
	h.press(tea.KeyMsg{Type: tea.KeyEnter})
	h.press(tea.KeyMsg{Type: tea.KeyEnter})
	if len(h.actions) != 1 || h.actions[0].Ref != "feat/auth" {
		t.Fatalf("actions = %+v, want switch feat/auth", h.actions)
	}
	// Escape from list mode quits; escape from filter mode clears.
	h.press(keyRunes("/"))
	h.press(tea.KeyMsg{Type: tea.KeyEsc})
	if h.model.filter != "" || len(h.model.visible()) != 3 {
		t.Fatalf("escape should clear the filter: %+v", h.model)
	}
}

func TestRefreshedReplacesRowsAndClampsCursor(t *testing.T) {
	h := newHarness(rows3())
	h.model.cursor = 2
	h.press(RefreshedMsg{Rows: rows3()[:1]})
	if len(h.model.rows) != 1 || h.model.cursor != 0 {
		t.Fatalf("rows = %d cursor = %d, want 1 row cursor 0", len(h.model.rows), h.model.cursor)
	}
	if h.model.repoName != "r" {
		t.Fatalf("repoName = %q, want r (from trunk path)", h.model.repoName)
	}
}

func TestRefreshErrorKeepsRows(t *testing.T) {
	h := newHarness(rows3())
	h.press(RefreshedMsg{Err: errors.New("wt exploded")})
	if len(h.model.rows) != 3 {
		t.Fatal("rows should survive a failed refresh")
	}
	if !strings.Contains(h.model.status, "wt exploded") {
		t.Fatalf("status = %q, want the error surfaced", h.model.status)
	}
}

func TestActionDone(t *testing.T) {
	h := newHarness(rows3())
	cmd := h.press(ActionDoneMsg{Quit: true})
	if cmd == nil {
		t.Fatal("successful quit-after action should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("want tea.Quit, got %T", cmd())
	}

	h2 := newHarness(rows3())
	cmd = h2.press(ActionDoneMsg{Err: errors.New("switch failed"), Quit: true})
	if !strings.Contains(h2.model.status, "switch failed") {
		t.Fatalf("status = %q, want failure surfaced", h2.model.status)
	}
	if cmd == nil {
		t.Fatal("failed action should return the refresh command")
	}
	cmd()
	if h2.refreshs != 1 {
		t.Fatalf("failed action should trigger a refresh, got %d", h2.refreshs)
	}
}

func TestMergeDestroyStubbed(t *testing.T) {
	h := newHarness(rows3())
	h.press(keyRunes("m"))
	if len(h.actions) != 0 || !strings.Contains(h.model.status, "merge") {
		t.Fatalf("m should only set a status: actions=%+v status=%q", h.actions, h.model.status)
	}
	h.press(keyRunes("d"))
	if len(h.actions) != 0 || !strings.Contains(h.model.status, "destroy") {
		t.Fatalf("d should only set a status: actions=%+v status=%q", h.actions, h.model.status)
	}
}

func TestManualRefreshKey(t *testing.T) {
	h := newHarness(rows3())
	cmd := h.press(keyRunes("r"))
	if cmd == nil {
		t.Fatal("r should return the refresh command")
	}
	cmd()
	if h.refreshs != 1 {
		t.Fatalf("refreshs = %d, want 1", h.refreshs)
	}
}

func TestViewRendersEveryMode(t *testing.T) {
	h := newHarness(rows3())
	h.press(RefreshedMsg{Rows: rows3()})
	if v := h.model.View(); !strings.Contains(v, "feat/auth") || !strings.Contains(v, "(trunk)") {
		t.Fatalf("list view missing rows:\n%s", v)
	}
	h.press(keyRunes("/"))
	if v := h.model.View(); !strings.Contains(v, "filter:") {
		t.Fatalf("filter view missing prompt:\n%s", v)
	}
	h.press(tea.KeyMsg{Type: tea.KeyEsc})
	h.press(keyRunes("c"))
	if v := h.model.View(); !strings.Contains(v, "new branch name") {
		t.Fatalf("input view missing prompt:\n%s", v)
	}
	if v := newHarness(nil).model.View(); !strings.Contains(v, "no worktrees") {
		t.Fatalf("empty view missing hint:\n%s", v)
	}
}

func TestActionKeysNeedARow(t *testing.T) {
	h := newHarness(nil)
	h.press(tea.KeyMsg{Type: tea.KeyEnter})
	h.press(keyRunes("t"))
	if len(h.actions) != 0 {
		t.Fatalf("no rows: no actions, got %+v", h.actions)
	}
}
