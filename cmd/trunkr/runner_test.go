package main

import (
	"strings"
	"testing"

	"github.com/disintegrator/trunkr/internal/config"
	"github.com/disintegrator/trunkr/internal/wt"
)

func TestDecideOpen(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		fallback      config.Container
		existing      int
		wantFocus     bool
		wantContainer config.Container
	}{
		{
			name: "generic with live panes focuses existing",
			mode: "", fallback: config.ContainerTab, existing: 2,
			wantFocus: true,
		},
		{
			name: "generic without panes opens configured container",
			mode: "", fallback: config.ContainerWorkspace, existing: 0,
			wantFocus: false, wantContainer: config.ContainerWorkspace,
		},
		{
			name: "explicit tab always opens even with live panes",
			mode: "tab", fallback: config.ContainerWorkspace, existing: 3,
			wantFocus: false, wantContainer: config.ContainerTab,
		},
		{
			name: "explicit workspace overrides configured container",
			mode: "workspace", fallback: config.ContainerTab, existing: 0,
			wantFocus: false, wantContainer: config.ContainerWorkspace,
		},
		{
			name: "explicit split",
			mode: "split", fallback: config.ContainerTab, existing: 1,
			wantFocus: false, wantContainer: config.ContainerSplit,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			focus, container := decideOpen(tt.mode, tt.fallback, tt.existing)
			if focus != tt.wantFocus {
				t.Errorf("focus = %v, want %v", focus, tt.wantFocus)
			}
			if !focus && container != tt.wantContainer {
				t.Errorf("container = %q, want %q", container, tt.wantContainer)
			}
		})
	}
}

func TestFindWorktree(t *testing.T) {
	list := wt.ListResult{Worktrees: []wt.Worktree{
		{Branch: "main", Path: "/repo", IsMain: true},
		{Branch: "feat/auth", Path: "/repo.feat-auth"},
		{Path: "/repo.detached", Detached: true},
	}}
	tests := []struct {
		name       string
		ref        string
		dir        string
		wantBranch string
		wantErr    string
	}{
		{
			name: "by ref from the picker",
			ref:  "feat/auth", dir: "/repo",
			wantBranch: "feat/auth",
		},
		{
			name: "unknown ref",
			ref:  "gone", dir: "/repo",
			wantErr: `no worktree for branch "gone"`,
		},
		{
			name:       "by dir containment for the standalone action",
			dir:        "/repo.feat-auth/internal/pkg",
			wantBranch: "feat/auth",
		},
		{
			name:    "dir outside every worktree",
			dir:     "/somewhere/else",
			wantErr: "not inside a worktree",
		},
		{
			name: "trunk refused by ref",
			ref:  "main", dir: "/repo",
			wantErr: "trunk worktree",
		},
		{
			name:    "trunk refused by dir",
			dir:     "/repo/cmd",
			wantErr: "trunk worktree",
		},
		{
			name:    "detached worktree refused",
			dir:     "/repo.detached",
			wantErr: "detached HEAD",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findWorktree(list, tt.ref, tt.dir)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", got.Branch, tt.wantBranch)
			}
		})
	}
}

func TestGist(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"last non-empty line wins", "fetching...\nerror: conflict in main.go\n\n", "error: conflict in main.go"},
		{"single line", "boom", "boom"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gist(tt.input); got != tt.want {
				t.Errorf("gist(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
