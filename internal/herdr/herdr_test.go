package herdr

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeHerdr installs a fake herdr binary and returns a Client pointed at it.
// The fake records its argv into logDir and behaves per the FAKE_HERDR_*
// environment variables set with t.Setenv:
//
//	FAKE_HERDR_STDOUT — envelope JSON printed to stdout
//	FAKE_HERDR_STDERR — line printed to stderr
//	FAKE_HERDR_EXIT   — exit code (default 0)
func fakeHerdr(t *testing.T, logDir string) *Client {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake herdr harness is a shell script; plugin targets linux/macos")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "herdr")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + logDir + `/args"
[ -n "$FAKE_HERDR_STDERR" ] && echo "$FAKE_HERDR_STDERR" >&2
[ -n "$FAKE_HERDR_STDOUT" ] && printf '%s' "$FAKE_HERDR_STDOUT"
exit "${FAKE_HERDR_EXIT:-0}"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Client{Bin: path, PluginID: "disintegrator.trunkr"}
}

func loggedArgs(t *testing.T, logDir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(logDir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
}

func TestFromEnvRequiresBinPath(t *testing.T) {
	t.Setenv("HERDR_BIN_PATH", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("FromEnv without HERDR_BIN_PATH should fail")
	}
}

func TestPaneList(t *testing.T) {
	logDir := t.TempDir()
	c := fakeHerdr(t, logDir)
	t.Setenv("FAKE_HERDR_STDOUT", `{"id":"cli:pane:list","result":{"type":"pane_list","panes":[
		{"pane_id":"w1:p1","workspace_id":"w1","tab_id":"w1:t1","focused":true,"agent_status":"working",
		 "cwd":"/repo","foreground_cwd":"/repo/sub","agent":"claude","label":null}]}}`)

	panes, err := c.PaneList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Pane{PaneID: "w1:p1", WorkspaceID: "w1", TabID: "w1:t1", Focused: true,
		AgentStatus: "working", Cwd: "/repo", ForegroundCwd: "/repo/sub", Agent: "claude"}
	if len(panes) != 1 || panes[0] != want {
		t.Errorf("panes = %+v, want [%+v]", panes, want)
	}
	if got := loggedArgs(t, logDir); !equal(got, []string{"pane", "list"}) {
		t.Errorf("args = %v", got)
	}
}

func TestTabCreate(t *testing.T) {
	logDir := t.TempDir()
	c := fakeHerdr(t, logDir)
	t.Setenv("FAKE_HERDR_STDOUT", `{"result":{"type":"tab_created","tab":{"tab_id":"w1:t2"},"root_pane":{"pane_id":"w1:p9","workspace_id":"w1","tab_id":"w1:t2"}}}`)

	pane, err := c.TabCreate(context.Background(), "w1", "/repo.feat", "feat")
	if err != nil {
		t.Fatal(err)
	}
	if pane.PaneID != "w1:p9" {
		t.Errorf("pane = %+v, want pane_id w1:p9", pane)
	}
	want := []string{"tab", "create", "--cwd", "/repo.feat", "--focus", "--workspace", "w1", "--label", "feat"}
	if got := loggedArgs(t, logDir); !equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestWorkspaceCreate(t *testing.T) {
	logDir := t.TempDir()
	c := fakeHerdr(t, logDir)
	t.Setenv("FAKE_HERDR_STDOUT", `{"result":{"type":"workspace_created","workspace":{"workspace_id":"w2"},"root_pane":{"pane_id":"w2:p1","workspace_id":"w2"}}}`)

	pane, err := c.WorkspaceCreate(context.Background(), "/repo.feat", "feat")
	if err != nil {
		t.Fatal(err)
	}
	if pane.PaneID != "w2:p1" {
		t.Errorf("pane = %+v, want pane_id w2:p1", pane)
	}
	want := []string{"workspace", "create", "--cwd", "/repo.feat", "--focus", "--label", "feat"}
	if got := loggedArgs(t, logDir); !equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestPaneSplit(t *testing.T) {
	tests := []struct {
		name       string
		targetPane string
		wantTail   []string
	}{
		{"explicit target", "w1:p3", []string{"--pane", "w1:p3"}},
		{"focused pane fallback", "", []string{"--current"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logDir := t.TempDir()
			c := fakeHerdr(t, logDir)
			t.Setenv("FAKE_HERDR_STDOUT", `{"result":{"type":"pane_info","pane":{"pane_id":"w1:p5"}}}`)

			pane, err := c.PaneSplit(context.Background(), tt.targetPane, "/repo.feat")
			if err != nil {
				t.Fatal(err)
			}
			if pane.PaneID != "w1:p5" {
				t.Errorf("pane = %+v, want pane_id w1:p5", pane)
			}
			want := append([]string{"pane", "split", "--direction", "right", "--cwd", "/repo.feat", "--focus"}, tt.wantTail...)
			if got := loggedArgs(t, logDir); !equal(got, want) {
				t.Errorf("args = %v, want %v", got, want)
			}
		})
	}
}

func TestPaneProcessInfo(t *testing.T) {
	logDir := t.TempDir()
	c := fakeHerdr(t, logDir)
	t.Setenv("FAKE_HERDR_STDOUT", `{"id":"cli:pane:process_info","result":{"type":"pane_process_info","process_info":{
		"pane_id":"w1:p5","shell_pid":4200,"foreground_process_group_id":4321,
		"foreground_processes":[{"argv":["claude","--continue"],"cmdline":"claude --continue","cwd":"/repo.feat","name":"claude","pid":4321}]}}}`)

	info, err := c.PaneProcessInfo(context.Background(), "w1:p5")
	if err != nil {
		t.Fatal(err)
	}
	if info.PaneID != "w1:p5" || info.ShellPID != 4200 || info.ForegroundProcessGroupID != 4321 {
		t.Errorf("info = %+v, want pane w1:p5 shell 4200 fg group 4321", info)
	}
	if len(info.ForegroundProcesses) != 1 || info.ForegroundProcesses[0].Name != "claude" || info.ForegroundProcesses[0].PID != 4321 {
		t.Errorf("foreground processes = %+v, want one claude pid 4321", info.ForegroundProcesses)
	}
	if got := loggedArgs(t, logDir); !equal(got, []string{"pane", "process-info", "--pane", "w1:p5"}) {
		t.Errorf("args = %v", got)
	}
}

func TestPaneRun(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want []string
	}{
		{
			"plain tokens pass through",
			[]string{"claude", "--continue"},
			[]string{"claude", "--continue"},
		},
		{
			"multi-word item keeps its word boundaries",
			[]string{"sh", "-c", "echo hi; exec sh"},
			[]string{"sh", "-c", "'echo hi; exec sh'"},
		},
		{
			"single quote in item is escaped",
			[]string{"sh", "-c", "echo 'it works'"},
			[]string{"sh", "-c", `'echo '\''it works'\'''`},
		},
		{
			"empty item stays a word",
			[]string{"env", ""},
			[]string{"env", "''"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logDir := t.TempDir()
			c := fakeHerdr(t, logDir)
			t.Setenv("FAKE_HERDR_STDOUT", `{"result":{"type":"ok"}}`)

			if err := c.PaneRun(context.Background(), "w1:p9", tt.argv); err != nil {
				t.Fatal(err)
			}
			want := append([]string{"pane", "run", "w1:p9"}, tt.want...)
			if got := loggedArgs(t, logDir); !equal(got, want) {
				t.Errorf("args = %v, want %v", got, want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"claude", "claude"},
		{"--continue", "--continue"},
		{"/repo/bin:x=y@%+,", "/repo/bin:x=y@%+,"},
		{"", "''"},
		{"echo hi; exec sh", "'echo hi; exec sh'"},
		{"a b", "'a b'"},
		{"it's", `'it'\''s'`},
		{"'", `''\'''`},
		{"$HOME", "'$HOME'"},
		{"a\"b", `'a"b'`},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestPluginPaneOpenEnvOrder(t *testing.T) {
	logDir := t.TempDir()
	c := fakeHerdr(t, logDir)
	t.Setenv("FAKE_HERDR_STDOUT", `{"result":{"type":"plugin_pane_opened","plugin_pane":{"plugin_id":"disintegrator.trunkr","entrypoint":"runner","pane":{"pane_id":""}}}}`)

	env := map[string]string{"TRUNKR_REF": "feat-a", "TRUNKR_DIR": "/repo", "TRUNKR_OP": "switch"}
	if err := c.PluginPaneOpen(context.Background(), "runner", env); err != nil {
		t.Fatal(err)
	}
	want := []string{"plugin", "pane", "open", "--plugin", "disintegrator.trunkr", "--entrypoint", "runner",
		"--env", "TRUNKR_DIR=/repo", "--env", "TRUNKR_OP=switch", "--env", "TRUNKR_REF=feat-a"}
	if got := loggedArgs(t, logDir); !equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestErrorEnvelope(t *testing.T) {
	logDir := t.TempDir()
	c := fakeHerdr(t, logDir)
	t.Setenv("FAKE_HERDR_STDOUT", `{"error":{"code":"not_found","message":"pane not found"}}`)

	err := c.TabFocus(context.Background(), "w1:t9")
	if err == nil || !strings.Contains(err.Error(), "pane not found") || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("want error carrying envelope message and code, got %v", err)
	}
}

func TestProcessFailureCarriesStderr(t *testing.T) {
	logDir := t.TempDir()
	c := fakeHerdr(t, logDir)
	t.Setenv("FAKE_HERDR_EXIT", "1")
	t.Setenv("FAKE_HERDR_STDERR", "no server running")

	_, err := c.PaneList(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no server running") {
		t.Fatalf("want error carrying stderr, got %v", err)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
