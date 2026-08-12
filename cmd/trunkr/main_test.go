package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHerdrSourceCWDUsesRepositoryRoot(t *testing.T) {
	var context invocationContext
	err := json.Unmarshal([]byte(`{
		"focused_pane_cwd":"/tmp/project.feature",
		"worktree":{"repo_root":"/tmp/project"}
	}`), &context)
	if err != nil {
		t.Fatal(err)
	}
	if got := herdrSourceCWD(context, context.FocusedPaneCWD); got != "/tmp/project" {
		t.Fatalf("expected repository root, got %q", got)
	}
}

func TestHerdrSourceCWDFallsBackToFocusedCheckout(t *testing.T) {
	if got := herdrSourceCWD(invocationContext{}, "/tmp/project"); got != "/tmp/project" {
		t.Fatalf("expected focused checkout, got %q", got)
	}
}

func TestReadRawPromptAcceptsInput(t *testing.T) {
	var output bytes.Buffer
	value, err := readRawPrompt(bufio.NewReader(bytes.NewBufferString("feature/test\r")), &output)
	if err != nil {
		t.Fatal(err)
	}
	if value != "feature/test" {
		t.Fatalf("expected feature/test, got %q", value)
	}
	if output.String() != "feature/test\r\n" {
		t.Fatalf("unexpected terminal output %q", output.String())
	}
}

func TestReadRawPromptHandlesBackspace(t *testing.T) {
	var output bytes.Buffer
	value, err := readRawPrompt(bufio.NewReader(bytes.NewBufferString("fixx\x7f\r")), &output)
	if err != nil {
		t.Fatal(err)
	}
	if value != "fix" {
		t.Fatalf("expected fix, got %q", value)
	}
}

func TestReadRawPromptCancelsOnEscape(t *testing.T) {
	var output bytes.Buffer
	_, err := readRawPrompt(bufio.NewReader(bytes.NewBufferString("partial\x1b")), &output)
	if !errors.Is(err, errPromptCancelled) {
		t.Fatalf("expected prompt cancellation, got %v", err)
	}
}

func TestReadLinePromptCancelsOnEscape(t *testing.T) {
	_, err := readLinePrompt(bufio.NewReader(bytes.NewBufferString("\x1b\n")))
	if !errors.Is(err, errPromptCancelled) {
		t.Fatalf("expected prompt cancellation, got %v", err)
	}
}

func TestWorktreeLabelShowsCurrentState(t *testing.T) {
	label := worktreeLabel(worktree{
		Branch:  "main",
		Path:    "/tmp/project",
		Current: true,
	})
	if label != "● main                         /tmp/project  current" {
		t.Fatalf("unexpected label %q", label)
	}
}

func TestWorktreeLabelShowsDefaultState(t *testing.T) {
	label := worktreeLabel(worktree{
		Branch:  "feature",
		Path:    "/tmp/project.feature",
		Current: false,
	})
	if label != "○ feature                      /tmp/project.feature" {
		t.Fatalf("unexpected label %q", label)
	}
}

func TestWorktreePathsParsesPorcelainOutput(t *testing.T) {
	output := []byte("worktree /tmp/project\x00HEAD abc\x00branch refs/heads/main\x00\x00worktree /tmp/project.feature\x00HEAD def\x00branch refs/heads/feature\x00\x00")
	paths := worktreePaths(output)
	if len(paths) != 2 {
		t.Fatalf("expected two worktree paths, got %d", len(paths))
	}
	if paths[0] != "/tmp/project" || paths[1] != "/tmp/project.feature" {
		t.Fatalf("unexpected worktree paths %#v", paths)
	}
}

func TestSamePathCleansPaths(t *testing.T) {
	if !samePath("/tmp/project/feature/..", "/tmp/project") {
		t.Fatal("expected cleaned paths to match")
	}
}

func TestDestructiveWorktreeRootsFindsPrimaryCheckout(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "project")
	runGit(t, "init", primary)
	runGit(t, "-C", primary, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	linked := filepath.Join(t.TempDir(), "project.feature")
	runGit(t, "-C", primary, "worktree", "add", "-b", "feature", linked)
	subdirectory := filepath.Join(linked, "nested")
	if err := os.Mkdir(subdirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	checkout, repoRoot, err := destructiveWorktreeRoots(subdirectory)
	if err != nil {
		t.Fatal(err)
	}
	if checkout != linked {
		t.Fatalf("expected linked checkout %q, got %q", linked, checkout)
	}
	if repoRoot != primary {
		t.Fatalf("expected primary checkout %q, got %q", primary, repoRoot)
	}
}

func TestDestructiveWorktreeRootsRejectsPrimaryCheckout(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "project")
	runGit(t, "init", primary)

	_, _, err := destructiveWorktreeRoots(primary)
	if err == nil || err.Error() != "the primary checkout cannot be removed or merged" {
		t.Fatalf("expected primary checkout error, got %v", err)
	}
}

func TestIsRegisteredWorktreeTracksRemoval(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "project")
	runGit(t, "init", primary)
	runGit(t, "-C", primary, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	linked := filepath.Join(t.TempDir(), "project.feature")
	runGit(t, "-C", primary, "worktree", "add", "-b", "feature", linked)

	registered, err := isRegisteredWorktree(primary, linked)
	if err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("expected linked checkout to be registered")
	}

	runGit(t, "-C", primary, "worktree", "remove", linked)
	registered, err = isRegisteredWorktree(primary, linked)
	if err != nil {
		t.Fatal(err)
	}
	if registered {
		t.Fatal("expected removed checkout not to be registered")
	}
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
