package wt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// Worktree is trunkr's view of one wt list row that has an actual worktree.
// Branch-only and remote-only rows are dropped.
type Worktree struct {
	// Branch is empty for a detached-HEAD worktree.
	Branch    string
	Path      string
	IsMain    bool
	IsCurrent bool
	Detached  bool
	// Dirty is true when any working-tree change flag is set (staged,
	// modified, untracked, renamed, deleted).
	Dirty bool
	// Conflicted is true when the working tree has merge conflicts.
	Conflicted bool
	// Ahead/Behind are commits relative to the default branch; zero for
	// the default branch itself.
	Ahead  int
	Behind int
	// State is wt's collapsed status vocabulary (display.state in schema
	// 2, main_state in schema 1), e.g. "integrated", "diverged", "empty".
	State string
}

// ListResult is the parsed output of wt list.
type ListResult struct {
	// DefaultBranch is known only under schema 2; empty otherwise.
	DefaultBranch string
	Worktrees     []Worktree
}

// List runs wt list in dir. Schema 2 is pinned per invocation so the result
// doesn't depend on the user's [list] json-schema setting, but the parser
// accepts both schemas anyway.
func (c *Client) List(ctx context.Context, dir string) (ListResult, error) {
	out, err := c.run(ctx, dir, "--config-set", "list.json-schema=2", "list", "--format=json")
	if err != nil {
		return ListResult{}, err
	}
	return ParseList(out)
}

// ParseList decodes wt list JSON in either schema: the schema 2 envelope
// object or the schema 1 bare array.
func ParseList(data []byte) (ListResult, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ListResult{}, fmt.Errorf("wt list: empty output")
	}
	if trimmed[0] == '[' {
		return parseListSchema1(trimmed)
	}
	return parseListSchema2(trimmed)
}

// schema 2: {schema: 2, repo: {...}, collected: {...}, items: [...]}

type schema2Envelope struct {
	Schema int `json:"schema"`
	Repo   struct {
		DefaultBranch string `json:"default_branch"`
	} `json:"repo"`
	Items []schema2Item `json:"items"`
}

type schema2Item struct {
	Branch   Opt[string] `json:"branch"`
	Worktree Opt[struct {
		Path     string `json:"path"`
		Main     bool   `json:"main"`
		Current  bool   `json:"current"`
		Detached bool   `json:"detached"`
		Changes  Opt[struct {
			Staged     bool `json:"staged"`
			Modified   bool `json:"modified"`
			Untracked  bool `json:"untracked"`
			Renamed    bool `json:"renamed"`
			Deleted    bool `json:"deleted"`
			Conflicted bool `json:"conflicted"`
		}] `json:"changes"`
	}] `json:"worktree"`
	// Absent on the default branch itself.
	DefaultBranch Opt[struct {
		Ahead  int `json:"ahead"`
		Behind int `json:"behind"`
	}] `json:"default_branch"`
	Display Opt[struct {
		State string `json:"state"`
	}] `json:"display"`
}

func parseListSchema2(data []byte) (ListResult, error) {
	var env schema2Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return ListResult{}, fmt.Errorf("wt list (schema 2): %w", err)
	}
	res := ListResult{DefaultBranch: env.Repo.DefaultBranch}
	for _, item := range env.Items {
		tree, ok := item.Worktree.Get()
		if !ok {
			continue
		}
		w := Worktree{
			Path:      tree.Path,
			IsMain:    tree.Main,
			IsCurrent: tree.Current,
			Detached:  tree.Detached,
		}
		if branch, ok := item.Branch.Get(); ok {
			w.Branch = branch
		}
		if ch, ok := tree.Changes.Get(); ok {
			w.Dirty = ch.Staged || ch.Modified || ch.Untracked || ch.Renamed || ch.Deleted
			w.Conflicted = ch.Conflicted
		}
		if rel, ok := item.DefaultBranch.Get(); ok {
			w.Ahead, w.Behind = rel.Ahead, rel.Behind
		}
		if d, ok := item.Display.Get(); ok {
			w.State = d.State
		}
		res.Worktrees = append(res.Worktrees, w)
	}
	return res, nil
}

// schema 1: bare array; commit → head, working_tree → worktree.changes,
// main + main_state → default_branch + display.state.

type schema1Item struct {
	Branch      Opt[string] `json:"branch"`
	Path        string      `json:"path"`
	Kind        string      `json:"kind"`
	WorkingTree Opt[struct {
		Staged    bool `json:"staged"`
		Modified  bool `json:"modified"`
		Untracked bool `json:"untracked"`
		Renamed   bool `json:"renamed"`
		Deleted   bool `json:"deleted"`
	}] `json:"working_tree"`
	MainState      string `json:"main_state"`
	OperationState string `json:"operation_state"`
	Main           Opt[struct {
		Ahead  int `json:"ahead"`
		Behind int `json:"behind"`
	}] `json:"main"`
	Worktree Opt[struct {
		Detached bool `json:"detached"`
	}] `json:"worktree"`
	IsMain    bool `json:"is_main"`
	IsCurrent bool `json:"is_current"`
}

func parseListSchema1(data []byte) (ListResult, error) {
	var items []schema1Item
	if err := json.Unmarshal(data, &items); err != nil {
		return ListResult{}, fmt.Errorf("wt list (schema 1): %w", err)
	}
	var res ListResult
	for _, item := range items {
		if item.Kind != "worktree" {
			continue
		}
		w := Worktree{
			Path:       item.Path,
			IsMain:     item.IsMain,
			IsCurrent:  item.IsCurrent,
			Conflicted: item.OperationState == "conflicts",
			State:      item.MainState,
		}
		if branch, ok := item.Branch.Get(); ok {
			w.Branch = branch
		}
		if tree, ok := item.Worktree.Get(); ok {
			w.Detached = tree.Detached
		}
		if ch, ok := item.WorkingTree.Get(); ok {
			w.Dirty = ch.Staged || ch.Modified || ch.Untracked || ch.Renamed || ch.Deleted
		}
		if rel, ok := item.Main.Get(); ok {
			w.Ahead, w.Behind = rel.Ahead, rel.Behind
		}
		res.Worktrees = append(res.Worktrees, w)
	}
	return res, nil
}
