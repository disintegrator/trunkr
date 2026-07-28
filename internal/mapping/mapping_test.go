package mapping

import (
	"testing"

	"github.com/disintegrator/trunkr/internal/herdr"
)

func pane(id, cwd, fgCwd string) herdr.Pane {
	return herdr.Pane{PaneID: id, Cwd: cwd, ForegroundCwd: fgCwd}
}

func TestPanesIn(t *testing.T) {
	paths := []string{"/home/u/repo", "/home/u/repo.feat-a", "/home/u/repo/.wt/nested"}
	tests := []struct {
		name   string
		panes  []herdr.Pane
		paths  []string
		target string
		want   []string
	}{
		{
			name:   "exact cwd match",
			panes:  []herdr.Pane{pane("p1", "/home/u/repo.feat-a", "")},
			paths:  paths,
			target: "/home/u/repo.feat-a",
			want:   []string{"p1"},
		},
		{
			name:   "subdirectory cwd matches",
			panes:  []herdr.Pane{pane("p1", "/home/u/repo.feat-a/internal/wt", "")},
			paths:  paths,
			target: "/home/u/repo.feat-a",
			want:   []string{"p1"},
		},
		{
			name:   "sibling with shared prefix does not match",
			panes:  []herdr.Pane{pane("p1", "/home/u/repo.feat-a", "")},
			paths:  paths,
			target: "/home/u/repo",
			want:   nil,
		},
		{
			name:   "nested worktree wins longest match",
			panes:  []herdr.Pane{pane("p1", "/home/u/repo/.wt/nested/src", "")},
			paths:  paths,
			target: "/home/u/repo",
			want:   nil,
		},
		{
			name:   "nested worktree claims its own pane",
			panes:  []herdr.Pane{pane("p1", "/home/u/repo/.wt/nested/src", "")},
			paths:  paths,
			target: "/home/u/repo/.wt/nested",
			want:   []string{"p1"},
		},
		{
			name:   "foreground cwd matches when shell cwd is elsewhere",
			panes:  []herdr.Pane{pane("p1", "/home/u", "/home/u/repo.feat-a/src")},
			paths:  paths,
			target: "/home/u/repo.feat-a",
			want:   []string{"p1"},
		},
		{
			name:   "empty cwds never match",
			panes:  []herdr.Pane{pane("p1", "", "")},
			paths:  paths,
			target: "/home/u/repo",
			want:   nil,
		},
		{
			name: "multiple panes filtered to target",
			panes: []herdr.Pane{
				pane("p1", "/home/u/repo", ""),
				pane("p2", "/home/u/repo.feat-a", ""),
				pane("p3", "/home/u/elsewhere", ""),
			},
			paths:  paths,
			target: "/home/u/repo",
			want:   []string{"p1"},
		},
		{
			name:   "trailing slash in worktree path is cleaned",
			panes:  []herdr.Pane{pane("p1", "/home/u/repo.feat-a/src", "")},
			paths:  []string{"/home/u/repo.feat-a/"},
			target: "/home/u/repo.feat-a",
			want:   []string{"p1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PanesIn(tt.panes, tt.paths, tt.target)
			var ids []string
			for _, p := range got {
				ids = append(ids, p.PaneID)
			}
			if !equal(ids, tt.want) {
				t.Errorf("PanesIn = %v, want %v", ids, tt.want)
			}
		})
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
