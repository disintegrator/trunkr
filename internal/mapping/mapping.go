// Package mapping derives the worktree→pane association live, by matching
// pane working directories against worktree paths from wt's own output.
// Nothing is persisted, so the mapping survives herdr restarts, hand-opened
// panes, and worktrees touched outside trunkr.
package mapping

import (
	"path/filepath"
	"strings"

	"github.com/disintegrator/trunkr/internal/herdr"
)

// PanesIn returns the panes living inside the worktree at target. Both of a
// pane's directories (shell cwd and foreground process cwd) are matched
// longest-wins against all known worktree paths, so a pane inside a nested
// worktree never counts toward an enclosing one.
func PanesIn(panes []herdr.Pane, worktreePaths []string, target string) []herdr.Pane {
	target = filepath.Clean(target)
	var out []herdr.Pane
	for _, p := range panes {
		if BestMatch(p.Cwd, worktreePaths) == target || BestMatch(p.ForegroundCwd, worktreePaths) == target {
			out = append(out, p)
		}
	}
	return out
}

// BestMatch returns the longest worktree path containing dir, or "". The
// longest-wins rule keeps a directory inside a nested worktree from matching
// an enclosing one.
func BestMatch(dir string, worktreePaths []string) string {
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	best := ""
	for _, wt := range worktreePaths {
		if wt == "" {
			continue
		}
		wt = filepath.Clean(wt)
		if dir != wt && !strings.HasPrefix(dir, wt+string(filepath.Separator)) {
			continue
		}
		if len(wt) > len(best) {
			best = wt
		}
	}
	return best
}
