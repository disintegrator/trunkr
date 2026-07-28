package picker

import (
	"testing"

	"github.com/disintegrator/trunkr/internal/herdr"
	"github.com/disintegrator/trunkr/internal/wt"
)

func TestRollup(t *testing.T) {
	tests := []struct {
		name   string
		agents []string
		want   string
	}{
		{"empty", nil, ""},
		{"single", []string{"idle"}, "idle"},
		{"blocked beats working", []string{"working", "blocked"}, "blocked"},
		{"working beats done", []string{"done", "working"}, "working"},
		{"done beats idle", []string{"idle", "done"}, "done"},
		{"unrecognized ranks as unknown", []string{"weird"}, "unknown"},
		{"idle beats unknown", []string{"weird", "idle"}, "idle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Rollup(tt.agents); got != tt.want {
				t.Errorf("Rollup(%v) = %q, want %q", tt.agents, got, tt.want)
			}
		})
	}
}

func TestBuildRows(t *testing.T) {
	list := wt.ListResult{Worktrees: []wt.Worktree{
		{Branch: "main", Path: "/r", IsMain: true},
		{Branch: "feat/a", Path: "/r.feat-a", Dirty: true, Ahead: 3, Behind: 1},
		{Branch: "", Path: "/r.detached", Detached: true}, // dropped
	}}
	panes := []herdr.Pane{
		{PaneID: "p1", Cwd: "/r", AgentStatus: "idle"},
		{PaneID: "p2", Cwd: "/r.feat-a", AgentStatus: "working"},
		{PaneID: "p3", Cwd: "/r.feat-a/sub", AgentStatus: "blocked"},
		{PaneID: "p4", Cwd: "/elsewhere", AgentStatus: "done"},
	}
	rows := BuildRows(list, panes)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (detached dropped)", len(rows))
	}
	if rows[0].Branch != "main" || rows[0].Panes != 1 || !rows[0].IsTrunk {
		t.Errorf("main row = %+v", rows[0])
	}
	feat := rows[1]
	if feat.Panes != 2 || Rollup(feat.Agents) != "blocked" || !feat.Dirty || feat.Ahead != 3 || feat.Behind != 1 {
		t.Errorf("feat row = %+v (rollup %q)", feat, Rollup(feat.Agents))
	}
	if got := RepoName(rows); got != "r" {
		t.Errorf("RepoName = %q, want r", got)
	}
}
