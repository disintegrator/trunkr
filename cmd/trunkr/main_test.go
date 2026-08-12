package main

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
)

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
