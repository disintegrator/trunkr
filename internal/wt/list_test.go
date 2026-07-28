package wt

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseList(t *testing.T) {
	// Both fixtures drop the branch-only row (no worktree) and keep the
	// detached-HEAD worktree with an empty Branch. Schema 1 has no
	// conflicted change flag — its conflicts signal is operation_state —
	// and no repo envelope, so no default branch.
	tests := []struct {
		name              string
		fixture           string
		wantDefaultBranch string
		want              []Worktree
	}{
		{
			name:              "schema 2 envelope",
			fixture:           "list_schema2.json",
			wantDefaultBranch: "main",
			want: []Worktree{
				{Branch: "main", Path: "/home/user/repo", IsMain: true, IsCurrent: true, State: "is_main"},
				{Branch: "feat-a", Path: "/home/user/repo.feat-a", Dirty: true, Ahead: 3, Behind: 1, State: "diverged"},
				{Branch: "", Path: "/home/user/repo.detached", Detached: true, Conflicted: true, State: "conflicts"},
			},
		},
		{
			name:    "schema 1 bare array",
			fixture: "list_schema1.json",
			want: []Worktree{
				{Branch: "main", Path: "/home/user/repo", IsMain: true, IsCurrent: true, State: "is_main"},
				{Branch: "feat-a", Path: "/home/user/repo.feat-a", Dirty: true, Conflicted: true, Ahead: 3, Behind: 1, State: "diverged"},
				{Branch: "", Path: "/home/user/repo.detached", Detached: true, State: "detached"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseList(data)
			if err != nil {
				t.Fatal(err)
			}
			if got.DefaultBranch != tt.wantDefaultBranch {
				t.Errorf("DefaultBranch = %q, want %q", got.DefaultBranch, tt.wantDefaultBranch)
			}
			if !reflect.DeepEqual(got.Worktrees, tt.want) {
				t.Errorf("worktrees mismatch\n got: %+v\nwant: %+v", got.Worktrees, tt.want)
			}
		})
	}
}

func TestParseListRejectsGarbage(t *testing.T) {
	for _, data := range []string{"", "Switched to feat-a", "42"} {
		if _, err := ParseList([]byte(data)); err == nil {
			t.Errorf("ParseList(%q) should fail", data)
		}
	}
}
