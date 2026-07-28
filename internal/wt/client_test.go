package wt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeWT installs a fake wt binary into a fresh directory and returns its
// path. The fake records its cwd and argv into logDir and behaves per the
// FAKE_WT_* environment variables set with t.Setenv:
//
//	FAKE_WT_STDOUT_FILE — file cat'ed to stdout
//	FAKE_WT_STDERR      — line printed to stderr
//	FAKE_WT_EXIT        — exit code (default 0)
func fakeWT(t *testing.T, logDir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake wt harness is a shell script; plugin targets linux/macos")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "wt")
	script := `#!/bin/sh
pwd > "` + logDir + `/cwd"
printf '%s\n' "$@" > "` + logDir + `/args"
[ -n "$FAKE_WT_STDERR" ] && echo "$FAKE_WT_STDERR" >&2
[ -n "$FAKE_WT_STDOUT_FILE" ] && cat "$FAKE_WT_STDOUT_FILE"
exit "${FAKE_WT_EXIT:-0}"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// loggedCall reads back what the fake wt recorded.
func loggedCall(t *testing.T, logDir string) (cwd string, args []string) {
	t.Helper()
	cwdRaw, err := os.ReadFile(filepath.Join(logDir, "cwd"))
	if err != nil {
		t.Fatal(err)
	}
	argsRaw, err := os.ReadFile(filepath.Join(logDir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(cwdRaw)), strings.Fields(string(argsRaw))
}

// stdoutFixture points the fake wt's stdout at content.
func stdoutFixture(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_WT_STDOUT_FILE", path)
}

func TestNewMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := New("")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "install it from") {
		t.Errorf("error lacks install hint: %v", err)
	}
	if _, err := New(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("explicit wt_path to a missing binary should fail")
	}
}

func TestNewFindsOnPath(t *testing.T) {
	logDir := t.TempDir()
	bin := fakeWT(t, logDir)
	t.Setenv("PATH", filepath.Dir(bin))
	c, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != bin {
		t.Errorf("Path = %q, want %q", c.Path, bin)
	}
}

func TestCommandRequiresDir(t *testing.T) {
	c := &Client{Path: "wt"}
	if _, err := c.Command(context.Background(), "", "list"); err == nil {
		t.Fatal("Command with empty dir should fail: wt must never run in the plugin root")
	}
}

func TestRunSetsCwdToTargetDir(t *testing.T) {
	logDir := t.TempDir()
	c := &Client{Path: fakeWT(t, logDir)}
	stdoutFixture(t, `{"action":"existing","branch":"feat-a","path":"/home/user/repo.feat-a"}`)

	target := t.TempDir()
	if _, err := c.Switch(context.Background(), target, "feat-a", SwitchOptions{}); err != nil {
		t.Fatal(err)
	}
	cwd, _ := loggedCall(t, logDir)
	// Resolve symlinks: pwd may report the resolved form of the temp dir.
	wantDir, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	gotDir, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if gotDir != wantDir {
		t.Errorf("wt ran in %q, want %q", gotDir, wantDir)
	}
}

func TestRunErrorCarriesStderr(t *testing.T) {
	logDir := t.TempDir()
	c := &Client{Path: fakeWT(t, logDir)}
	t.Setenv("FAKE_WT_EXIT", "1")
	t.Setenv("FAKE_WT_STDERR", "pre-switch hook failed: lint")

	_, err := c.Switch(context.Background(), t.TempDir(), "feat-a", SwitchOptions{})
	if err == nil || !strings.Contains(err.Error(), "pre-switch hook failed: lint") {
		t.Fatalf("error should carry wt's stderr, got %v", err)
	}
}

func TestSwitch(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		opts     SwitchOptions
		stdout   string
		wantArgs []string
		want     SwitchResult
		wantErr  string
	}{
		{
			name:     "plain switch",
			ref:      "feat-a",
			stdout:   `{"action":"existing","branch":"feat-a","path":"/home/user/repo.feat-a"}`,
			wantArgs: []string{"switch", "feat-a", "--no-cd", "--format", "json"},
			want:     SwitchResult{Action: SwitchExisting, Branch: "feat-a", Path: "/home/user/repo.feat-a"},
		},
		{
			name:     "create with base",
			ref:      "feat-b",
			opts:     SwitchOptions{Create: true, Base: "main", Yes: true},
			stdout:   `{"action":"created","branch":"feat-b","path":"/home/user/repo.feat-b","created_branch":true,"base_branch":"main"}`,
			wantArgs: []string{"switch", "feat-b", "--no-cd", "--format", "json", "--create", "--base", "main", "--yes"},
			want:     SwitchResult{Action: SwitchCreated, Branch: "feat-b", Path: "/home/user/repo.feat-b", CreatedBranch: true, BaseBranch: "main"},
		},
		{
			name:     "pr checkout with hooks off",
			ref:      "pr:123",
			opts:     SwitchOptions{NoHooks: true},
			stdout:   `{"action":"created","branch":"pr-123","path":"/home/user/repo.pr-123","created_branch":true,"from_remote":"origin/pr-123"}`,
			wantArgs: []string{"switch", "pr:123", "--no-cd", "--format", "json", "--no-hooks"},
			want:     SwitchResult{Action: SwitchCreated, Branch: "pr-123", Path: "/home/user/repo.pr-123", CreatedBranch: true, FromRemote: "origin/pr-123"},
		},
		{
			name:    "missing path rejected",
			ref:     "feat-a",
			stdout:  `{"action":"existing","branch":"feat-a"}`,
			wantErr: "no path",
		},
		{
			name:    "garbage output rejected",
			ref:     "feat-a",
			stdout:  "Switched to feat-a",
			wantErr: "parsing JSON",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logDir := t.TempDir()
			c := &Client{Path: fakeWT(t, logDir)}
			stdoutFixture(t, tt.stdout)

			got, err := c.Switch(context.Background(), t.TempDir(), tt.ref, tt.opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("result = %+v, want %+v", got, tt.want)
			}
			if _, args := loggedCall(t, logDir); !equalStrings(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	logDir := t.TempDir()
	c := &Client{Path: fakeWT(t, logDir)}
	stdoutFixture(t, `[{"branch":"feat-a","branch_deleted":true,"kind":"worktree","path":"/home/user/repo.feat-a"}]`)

	got, err := c.Remove(context.Background(), t.TempDir(), []string{"feat-a"}, RemoveOptions{Force: true, Foreground: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []RemoveResult{{Branch: "feat-a", Path: "/home/user/repo.feat-a", BranchDeleted: true}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("result = %+v, want %+v", got, want)
	}
	wantArgs := []string{"remove", "feat-a", "--format", "json", "--force", "--foreground"}
	if _, args := loggedCall(t, logDir); !equalStrings(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}
}

func TestListPinsSchema2(t *testing.T) {
	logDir := t.TempDir()
	c := &Client{Path: fakeWT(t, logDir)}
	stdoutFixture(t, `{"schema":2,"repo":{"default_branch":"main"},"collected":{},"items":[]}`)

	if _, err := c.List(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--config-set", "list.json-schema=2", "list", "--format=json"}
	if _, args := loggedCall(t, logDir); !equalStrings(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}
}

func TestSwitchStreaming(t *testing.T) {
	logDir := t.TempDir()
	c := &Client{Path: fakeWT(t, logDir)}
	stdoutFixture(t, `{"action":"created","branch":"feat-a","path":"/home/user/repo.feat-a","created_branch":true}`)
	t.Setenv("FAKE_WT_STDERR", "creating worktree...")

	var stderr strings.Builder
	got, err := c.SwitchStreaming(context.Background(), t.TempDir(), "feat-a", SwitchOptions{Create: true}, strings.NewReader(""), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := SwitchResult{Action: SwitchCreated, Branch: "feat-a", Path: "/home/user/repo.feat-a", CreatedBranch: true}
	if got != want {
		t.Errorf("result = %+v, want %+v", got, want)
	}
	// The hook/progress stream must reach the caller's writer, not the JSON.
	if !strings.Contains(stderr.String(), "creating worktree...") {
		t.Errorf("stderr writer did not receive wt's stream: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), `"path"`) {
		t.Errorf("JSON result leaked into the streamed output: %q", stderr.String())
	}
	wantArgs := []string{"switch", "feat-a", "--no-cd", "--format", "json", "--create"}
	if _, args := loggedCall(t, logDir); !equalStrings(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}
}

func TestSwitchStreamingFailure(t *testing.T) {
	logDir := t.TempDir()
	c := &Client{Path: fakeWT(t, logDir)}
	t.Setenv("FAKE_WT_EXIT", "1")
	t.Setenv("FAKE_WT_STDERR", "pre-switch hook failed")

	var stderr strings.Builder
	_, err := c.SwitchStreaming(context.Background(), t.TempDir(), "feat-a", SwitchOptions{}, strings.NewReader(""), &stderr)
	if err == nil {
		t.Fatal("want error on nonzero exit")
	}
	if !strings.Contains(stderr.String(), "pre-switch hook failed") {
		t.Errorf("failure output should have streamed to the writer: %q", stderr.String())
	}
}

func equalStrings(a, b []string) bool {
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
