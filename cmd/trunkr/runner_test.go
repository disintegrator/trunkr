package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/disintegrator/trunkr/internal/config"
	"github.com/disintegrator/trunkr/internal/herdr"
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
			got, err := findWorktree(list, tt.ref, tt.dir, "merge")
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

func TestPaneShellIdle(t *testing.T) {
	tests := []struct {
		name string
		info herdr.PaneProcessInfo
		want bool
	}{
		{
			name: "shell at prompt",
			info: herdr.PaneProcessInfo{ShellPID: 4200, ForegroundProcessGroupID: 4200},
			want: true,
		},
		{
			name: "no shell yet",
			info: herdr.PaneProcessInfo{},
			want: false,
		},
		{
			name: "startup child holds the foreground",
			info: herdr.PaneProcessInfo{ShellPID: 4200, ForegroundProcessGroupID: 4321},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paneShellIdle(tt.info); got != tt.want {
				t.Errorf("paneShellIdle(%+v) = %v, want %v", tt.info, got, tt.want)
			}
		})
	}
}

func TestWaitPaneReady(t *testing.T) {
	idle := herdr.PaneProcessInfo{ShellPID: 4200, ForegroundProcessGroupID: 4200}
	busy := herdr.PaneProcessInfo{ShellPID: 4200, ForegroundProcessGroupID: 4321}

	t.Run("stops polling once the shell idles", func(t *testing.T) {
		responses := []func() (herdr.PaneProcessInfo, error){
			func() (herdr.PaneProcessInfo, error) { return herdr.PaneProcessInfo{}, errors.New("pane not found") },
			func() (herdr.PaneProcessInfo, error) { return busy, nil },
			func() (herdr.PaneProcessInfo, error) { return idle, nil },
		}
		polls := 0
		waitPaneReady(context.Background(), func(context.Context) (herdr.PaneProcessInfo, error) {
			resp := responses[min(polls, len(responses)-1)]
			polls++
			return resp()
		}, 10, time.Millisecond)
		if polls != 3 {
			t.Errorf("polls = %d, want 3 (errored, busy, idle)", polls)
		}
	})

	t.Run("gives up after the attempt bound", func(t *testing.T) {
		polls := 0
		waitPaneReady(context.Background(), func(context.Context) (herdr.PaneProcessInfo, error) {
			polls++
			return busy, nil
		}, 5, time.Millisecond)
		if polls != 5 {
			t.Errorf("polls = %d, want 5", polls)
		}
	})

	t.Run("returns on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		polls := 0
		waitPaneReady(ctx, func(context.Context) (herdr.PaneProcessInfo, error) {
			polls++
			cancel()
			return busy, nil
		}, 100, time.Hour)
		if polls != 1 {
			t.Errorf("polls = %d, want 1 (cancelled after first poll)", polls)
		}
	})
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
