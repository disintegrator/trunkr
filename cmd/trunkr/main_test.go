package main

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUsesWorktrunkInCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".config")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "wt.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if !usesWorktrunk(root) {
		t.Fatal("expected Worktrunk to be enabled")
	}
}

func TestUsesWorktrunkWalksParents(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".config")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "wt.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	if !usesWorktrunk(child) {
		t.Fatal("expected parent Worktrunk config to be detected")
	}
}

func TestUsesWorktrunkRequiresFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".config", "wt.toml"), 0o755); err != nil {
		t.Fatal(err)
	}

	if usesWorktrunk(root) {
		t.Fatal("a wt.toml directory must not enable Worktrunk")
	}
}

func TestUsesWorktrunkDisabledWithoutConfig(t *testing.T) {
	if usesWorktrunk(t.TempDir()) {
		t.Fatal("expected Worktrunk to be disabled")
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
		Branch:          "main",
		Path:            "/tmp/project",
		Current:         true,
		OpenWorkspaceID: "",
	})
	if label != "● main                         /tmp/project  current" {
		t.Fatalf("unexpected label %q", label)
	}
}

func TestWorktreeLabelShowsOpenState(t *testing.T) {
	label := worktreeLabel(worktree{
		Branch:          "feature",
		Path:            "/tmp/project.feature",
		Current:         false,
		OpenWorkspaceID: "w1",
	})
	if label != "◆ feature                      /tmp/project.feature  open" {
		t.Fatalf("unexpected label %q", label)
	}
}
