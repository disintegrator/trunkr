package main

import (
	"testing"

	"github.com/disintegrator/trunkr/internal/config"
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
