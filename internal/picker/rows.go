// Package picker is trunkr's interactive worktree picker: a Bubble Tea model
// rendering one row per worktree — branch, live-pane count, worst-first agent
// status, git state — with keys that hand actions back to the caller. The
// model itself performs no I/O; data loading and action execution are
// injected, keeping every Update transition unit-testable.
package picker

import (
	"path/filepath"

	"github.com/disintegrator/trunkr/internal/herdr"
	"github.com/disintegrator/trunkr/internal/mapping"
	"github.com/disintegrator/trunkr/internal/wt"
)

// Row is one worktree line: git facts from wt list joined with live pane
// facts from herdr pane list, matched by cwd. No paths are shown — worktrunk
// paths are template-derived noise; branch is the identity.
type Row struct {
	Branch  string
	Path    string
	IsTrunk bool
	Dirty   bool
	Ahead   int
	Behind  int
	Panes   int
	Agents  []string
}

// BuildRows joins wt's worktree list with herdr's live panes. Detached or
// branchless worktrees are dropped: the picker addresses worktrees by branch.
func BuildRows(list wt.ListResult, panes []herdr.Pane) []Row {
	paths := make([]string, 0, len(list.Worktrees))
	for _, w := range list.Worktrees {
		paths = append(paths, w.Path)
	}
	rows := make([]Row, 0, len(list.Worktrees))
	for _, w := range list.Worktrees {
		if w.Branch == "" || w.Detached {
			continue
		}
		mine := mapping.PanesIn(panes, paths, w.Path)
		var agents []string
		for _, p := range mine {
			agents = append(agents, p.AgentStatus)
		}
		rows = append(rows, Row{
			Branch:  w.Branch,
			Path:    w.Path,
			IsTrunk: w.IsMain,
			Dirty:   w.Dirty,
			Ahead:   w.Ahead,
			Behind:  w.Behind,
			Panes:   len(mine),
			Agents:  agents,
		})
	}
	return rows
}

// statusRank orders agent statuses worst-first: the rolled-up status is the
// one most worth attention.
var statusRank = map[string]int{"blocked": 0, "working": 1, "done": 2, "idle": 3, "unknown": 4}

// Rollup picks the most attention-worthy agent status across a worktree's
// panes; empty when there are none.
func Rollup(agents []string) string {
	best := ""
	for _, a := range agents {
		rank, known := statusRank[a]
		if !known {
			rank = statusRank["unknown"]
		}
		if best == "" || rank < statusRank[best] {
			if !known {
				a = "unknown"
			}
			best = a
		}
	}
	return best
}

// RepoName is the display name for the title bar: the trunk worktree's
// directory name.
func RepoName(rows []Row) string {
	for _, r := range rows {
		if r.IsTrunk {
			return filepath.Base(r.Path)
		}
	}
	return ""
}
